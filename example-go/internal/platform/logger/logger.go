package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/DeRuina/timberjack"
	"github.com/rs/zerolog"
)

const timeFormat = "2006-01-02T15:04:05.000Z07:00"

type Config struct {
	Env string

	// Level = trace|debug|info|warn|error|fatal|panic|disabled.
	Level string

	Service string
	Version string

	// Out is where JSON lines are written. Defaults to os.Stdout.
	Out io.Writer

	File string

	// MaxSizeMB rotates the file early when it grows past this size in MB. Default 100.
	MaxSizeMB int

	// MaxBackups is how many rotated files to keep. Default 5.
	MaxBackups int

	// MaxAgeDays deletes rotated files older than this. Default 28.
	MaxAgeDays int

	// Compression is "none", "gzip" or "zstd". Empty means "none".
	Compression string

	// SampleBurst, when > 0, keeps at most SampleBurst low-severity events per
	// SamplePeriod and drops the rest. Errors are never sampled.
	// Leave 0 while debugging.
	SampleBurst  uint32
	SamplePeriod time.Duration
}

var (
	base        zerolog.Logger
	baseCfg     Config
	globalsOnce sync.Once

	fileSinkMu sync.Mutex
	fileSink   io.Closer
)

func init() {
	// Usable before Init so nothing nil-panics during early startup.
	base = zerolog.New(os.Stdout).
		With().Timestamp().
		Str("service", "unknown").
		Logger()
}

func Init(cfg Config) error {
	l, sink, err := New(cfg)
	base = l

	fileSinkMu.Lock()
	baseCfg = cfg
	fileSink = sink
	fileSinkMu.Unlock()

	return err
}

// Close releases the log file handle opened by Init and drops the file sink, so
// logging after Close stays valid (stdout/Out only). Safe to call more than once.
func Close() error {
	fileSinkMu.Lock()
	defer fileSinkMu.Unlock()

	if fileSink == nil {
		return nil
	}

	err := fileSink.Close()
	fileSink = nil

	// Never leave the logger pointing at a closed file: the next log line would
	// fail. Rebuild it without the file sink.
	stdoutOnly := baseCfg
	stdoutOnly.File = ""
	base = build(stdoutOnly, output(stdoutOnly))

	return err
}

// L returns the process-wide logger.
// Prefer Ctx inside request handlers so the request fields come along.
func L() *zerolog.Logger { return &base }

// convert slog.logger into this zerolog
func Slog() *slog.Logger {
	return slog.New(zerolog.NewSlogHandler(base))
}

// New builds a configured logger without touching the process-wide one.
func New(cfg Config) (zerolog.Logger, io.Closer, error) {
	globalsOnce.Do(configureGlobals)

	out := output(cfg)

	if cfg.File != "" {
		fw, err := openFile(cfg)
		if err != nil {
			// Still hand back a stdout logger so the caller is never blind.
			return build(cfg, out), nil, err
		}
		return build(cfg, io.MultiWriter(out, fw)), fw, nil
	}

	return build(cfg, out), nil, nil
}

// output resolves the base writer: cfg.Out, or stdout.
func output(cfg Config) io.Writer {
	if cfg.Out != nil {
		return cfg.Out
	}
	return os.Stdout
}

// build assembles the logger from an already-resolved writer.
func build(cfg Config, out io.Writer) zerolog.Logger {
	service := cfg.Service
	if service == "" {
		service = "unknown"
	}

	l := zerolog.New(out).
		Level(parseLevel(cfg.Level, cfg.Env)).
		With().
		Timestamp().
		Str("service", service).
		Str("env", cfg.Env).
		Str("version", cfg.Version).
		Logger().
		With().
		Caller().
		Logger()

	if cfg.SampleBurst > 0 {
		period := cfg.SamplePeriod
		if period <= 0 {
			period = time.Second
		}
		l = l.Sample(keepErrors{next: &zerolog.BurstSampler{
			Burst:  cfg.SampleBurst,
			Period: period,
		}})
	}

	return l
}

// dailyRotationAt is the wall-clock time that starts a new log file each day.
var dailyRotationAt = []string{"00:00"}

// openFile returns a daily-rotating writer for cfg.File, creating the directory
func openFile(cfg Config) (io.WriteCloser, error) {
	if dir := filepath.Dir(cfg.File); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create log dir %q: %w", dir, err)
		}
	}

	return &timberjack.Logger{
		Filename:    cfg.File,
		MaxSize:     fallback(cfg.MaxSizeMB, 100),
		MaxBackups:  fallback(cfg.MaxBackups, 5),
		MaxAge:      fallback(cfg.MaxAgeDays, 28),
		Compression: cfg.Compression,
		LocalTime:   true,

		RotateAt:         dailyRotationAt,
		BackupTimeFormat: "2006-01-02-15-04-05.000",
	}, nil
}

func fallback(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

type keepErrors struct {
	next zerolog.Sampler
}

func (s keepErrors) Sample(lvl zerolog.Level) bool {
	if lvl >= zerolog.WarnLevel {
		return true
	}
	return s.next.Sample(lvl)
}

func configureGlobals() {
	zerolog.TimeFieldFormat = timeFormat

	// Keep caller short: "http/handler.go:42", not the whole module path.
	zerolog.CallerMarshalFunc = func(_ uintptr, file string, line int) string {
		return fmt.Sprintf("%s:%d", shortenPath(file), line)
	}

	// Attach a stack trace when the error carries one (see WithStack). Returning
	// nil tells zerolog to skip the field, so plain errors stay clean.
	zerolog.ErrorStackMarshaler = func(err error) any {
		if st := StackTrace(err); st != "" {
			return st
		}
		return nil
	}

	// Never lose logs silently.
	zerolog.ErrorHandler = func(err error) {
		fmt.Fprintf(os.Stderr, "logger: failed to write log: %v\n", err)
	}
}

func shortenPath(file string) string {
	parts := strings.Split(filepath.ToSlash(file), "/")
	if len(parts) > 2 {
		parts = parts[len(parts)-2:]
	}
	return strings.Join(parts, "/")
}

func parseLevel(level, env string) zerolog.Level {
	if level != "" {
		if l, err := zerolog.ParseLevel(strings.ToLower(strings.TrimSpace(level))); err == nil {
			return l
		}
	}

	switch strings.ToLower(env) {
	case "development", "dev", "local":
		return zerolog.DebugLevel
	default:
		return zerolog.InfoLevel
	}
}
