package logger

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The whole point of this package: one line, valid JSON, enough fields to
// debug without a debugger.
func TestNewEmitsStructuredJSON(t *testing.T) {
	var buf bytes.Buffer

	l, _, err := New(Config{
		Env:     "development",
		Level:   "debug",
		Service: "test",
		Version: "test",
		Out:     &buf,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Simulate the two-step error flow: wrap where it is born...
	cause := WithStack(errors.New("boom"))
	// ...and log once at the boundary.
	l.Error().Stack().Err(cause).Str("dependency", "mysql").Msg("health check failed")

	var got map[string]any
	if uerr := json.Unmarshal(buf.Bytes(), &got); uerr != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", uerr, buf.String())
	}

	for _, key := range []string{
		"level", "message", "error", "stack", "caller",
		"service", "env", "version", "time", "dependency",
	} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing field %q in %s", key, buf.String())
		}
	}

	if got["error"] != "boom" {
		t.Errorf("error = %v, want boom", got["error"])
	}
	if stack, _ := got["stack"].(string); !strings.Contains(stack, "TestNewEmitsStructuredJSON") {
		t.Errorf("stack does not point at the test:\n%v", got["stack"])
	}
}

func TestLevelFiltersNoise(t *testing.T) {
	var buf bytes.Buffer

	l, _, err := New(Config{Level: "info", Out: &buf})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	l.Debug().Msg("noise")
	l.Info().Msg("signal")

	if strings.Contains(buf.String(), "noise") {
		t.Errorf("debug line leaked at info level: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "signal") {
		t.Errorf("info line missing: %s", buf.String())
	}
}

// errors.Is must keep working through WithStack.
func TestWithStackKeepsWrapping(t *testing.T) {
	sentinel := errors.New("sentinel")
	err := WithStack(sentinel)

	if !errors.Is(err, sentinel) {
		t.Fatal("errors.Is broken through WithStack")
	}
	if StackTrace(err) == "" {
		t.Fatal("WithStack did not capture a stack")
	}
	if StackTrace(nil) != "" {
		t.Fatal("nil must have no stack")
	}
}

// Logs must land in a file too, not only on stdout, and the file must be JSON.
func TestFileSinkWritesJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "app.log")

	var console bytes.Buffer
	l, closer, err := New(Config{Level: "info", Out: &console, File: path})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer closer.Close()

	l.Info().Str("component", "test").Msg("hello file")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}

	var got map[string]any
	if uerr := json.Unmarshal(data, &got); uerr != nil {
		t.Fatalf("file is not valid JSON: %v\n%s", uerr, data)
	}
	if got["message"] != "hello file" {
		t.Errorf("file message = %v", got["message"])
	}
	if !strings.Contains(console.String(), "hello file") {
		t.Errorf("stdout should keep working too: %s", console.String())
	}
}
