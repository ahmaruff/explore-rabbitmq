// This file gives errors a stack trace.
//
// In Go an error is just a message: once you `return err` the call path is gone,
// and a few layers up you are left with text like "connection refused" with no
// clue where it came from. So we attach a stack ONCE, the moment the error is
// born (WithStack), carry it up untouched through fmt.Errorf("%w"), and read it
// back when we log (StackTrace).

package logger

import (
	"errors"
	"runtime"
	"strconv"
	"strings"
)

// stackDepth caps how many frames a captured stack keeps. Deep enough to show
// the path from the entry point down to where the error was born, shallow
// enough to stay readable in a log line.
const stackDepth = 16

// StackTrace digs through err (and everything it wraps) for one that carries a
// stack, and returns it, or "" when none is found.
//
// "Carries a stack" means the error has a Stack() string method - which is what
// *stackedError provides. errors.As walks the Unwrap chain for us, so it finds
// the stack even when the error got wrapped a few times by fmt.Errorf("%w").
//
// The configured zerolog.ErrorStackMarshaler calls this to fill the JSON "stack"
// field; returning "" tells zerolog to skip the field, so plain errors stay
// clean.
func StackTrace(err error) string {
	var s interface{ Stack() string }
	if errors.As(err, &s) {
		return s.Stack()
	}
	return ""
}

// WithStack wraps err so the place it was born is remembered.
//
// Call it ONCE, the moment the error is created - right after the db query, the
// HTTP call or the parse that failed - because that is the only moment the call
// path is still on the stack. Higher layers only add text with
// fmt.Errorf("...: %w", err) and must NOT call WithStack again. Then log the
// error once, at the boundary that decides the outcome (handler, worker, or
// startup in main).
//
// It is a no-op when err is nil or already carries a stack, so re-wrapping
// never doubles the frames. Capturing a stack allocates, so keep it off hot
// paths whose errors are never logged.
func WithStack(err error) error {
	if err == nil {
		return nil
	}
	if StackTrace(err) != "" {
		return err
	}
	return &stackedError{err: err, stack: captureStack()}
}

// stackedError is what WithStack returns: the original error plus the stack
// captured at wrap time.
type stackedError struct {
	err   error
	stack string
}

// Error returns the original message, so wrapping is invisible to the caller.
func (e *stackedError) Error() string { return e.err.Error() }

// Unwrap exposes the wrapped error so errors.Is / errors.As keep working
// through this wrapper.
func (e *stackedError) Unwrap() error { return e.err }

// Stack satisfies the interface StackTrace looks for.
func (e *stackedError) Stack() string { return e.stack }

// captureStack records the current call path and formats it for a log line.
//
// runtime.Callers fills pcs with program counters, one per frame. skip=3 drops
// the three frames we do not care about - runtime.Callers, captureStack itself
// and WithStack - so the first recorded frame is WithStack's caller, i.e. the
// place the error was born.
//
// CallersFrames turns those addresses into readable info (function, file,
// line). One PC can expand into several frames because of inlining, which is
// why we walk it with Next() instead of reading pcs directly.
//
// The result looks like debug.Stack() but is trimmed: no "goroutine N
// [running]" header, no runtime/WithStack frames, no hex offsets, and both the
// function name and the file path are shortened.
func captureStack() string {
	pcs := make([]uintptr, stackDepth)

	// skip=3 drops runtime.Callers, captureStack and WithStack.
	n := runtime.Callers(3, pcs)
	if n == 0 {
		return ""
	}

	frames := runtime.CallersFrames(pcs[:n])

	// Each frame is written as:
	//     package.Func
	//         dir/file.go:123
	var b strings.Builder
	for {
		frame, more := frames.Next() // Next advances; more reports if a frame remains

		b.WriteString(shortFunc(frame.Function))
		b.WriteString("\n\t")
		b.WriteString(shortenPath(frame.File))
		b.WriteString(":")
		b.WriteString(strconv.Itoa(frame.Line))
		b.WriteString("\n")

		if !more {
			break
		}
	}

	return b.String()
}

// shortFunc trims a fully qualified function name to its last path element,
// e.g. "github.com/x/internal/platform/db.New" -> "db.New".
func shortFunc(name string) string {
	if i := strings.LastIndexByte(name, '/'); i != -1 {
		return name[i+1:]
	}
	return name
}
