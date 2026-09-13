package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestSplit(t *testing.T) {
	t.Parallel()
	name, pathinfo, err := splitPathInfo(".", "exec_if_test.go/hello/world", ".go")
	if err != nil {
		t.Errorf("error: %s", err)
	}
	if name != "exec_if_test.go" {
		t.Errorf("name %s != exec_if_test.go", name)
	}
	if pathinfo != "/hello/world" {
		t.Errorf("pathinfo %s != /hello/world", pathinfo)
	}
}

func TestSplitNotFound(t *testing.T) {
	t.Parallel()
	name, pathinfo, err := splitPathInfo(".", "xyz/hello/world", "")
	if err == nil {
		t.Errorf("found: name=%s, pathinfo=%s", name, pathinfo)
	}
}

func TestSplitSuffix(t *testing.T) {
	t.Parallel()
	name, pathinfo, err := splitPathInfo(".", "exec_if_test.go/hello/world", ".ext")
	if err == nil {
		t.Errorf("found: name=%s, pathinfo=%s", name, pathinfo)
	}
}

func TestDoPipeWriteClose(t *testing.T) {
	t.Parallel()
	rd, wr := io.Pipe()
	var wg sync.WaitGroup
	wr.Close()
	wg.Go(func() {
		err := DoPipe(rd, wr)
		if err != nil {
			t.Errorf("pipe error: %s", err)
		}
	})
	wg.Wait()
}

func TestDoPipeReadClose(t *testing.T) {
	t.Parallel()
	rd, wr := io.Pipe()
	var wg sync.WaitGroup
	rd.Close()
	wg.Go(func() {
		err := DoPipe(rd, wr)
		if err == nil {
			t.Error("pipe no-error")
		}
	})
	wg.Wait()
}

type runner1 struct{}
type runner2 struct{}
type writer struct {
	out *bytes.Buffer
}

func (runner runner1) Run(conf SrvConfig, cmdname string, envvar map[string]string,
	stdin io.ReadCloser, stdout io.Writer, stderr io.Writer, ctx context.Context) error {
	fmt.Fprintln(stdout, "Status: 200")
	fmt.Fprintln(stdout, "Content-Type: application/json")
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "{\"hello\": true}")
	return nil
}

func (runner runner2) Run(conf SrvConfig, cmdname string, envvar map[string]string,
	stdin io.ReadCloser, stdout io.Writer, stderr io.Writer, ctx context.Context) error {
	fmt.Fprintln(stdout, "Status: 500")
	fmt.Fprintln(stdout, "Content-Type: application/json")
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "{\"hello\": true}")
	return nil
}

func (runner runner1) Exists(conf SrvConfig, path string, ctx context.Context) (string, string, error) {
	return splitPathInfo(conf.BaseDir, path, conf.Suffix)
}

func (runner runner2) Exists(conf SrvConfig, path string, ctx context.Context) (string, string, error) {
	return splitPathInfo(conf.BaseDir, path, conf.Suffix)
}

func (w writer) Header() http.Header {
	return http.Header{}
}

func (w writer) Write(data []byte) (int, error) {
	return w.out.Write(data)
}

func (w writer) WriteHeader(statusCode int) {
	fmt.Fprintf(w, "status code = %d\n", statusCode)
}

type runnerErr struct{}

func (runner runnerErr) Run(conf SrvConfig, cmdname string, envvar map[string]string,
	stdin io.ReadCloser, stdout io.Writer, stderr io.Writer, ctx context.Context) error {
	return fmt.Errorf("boom: script failed")
}

func (runner runnerErr) Exists(conf SrvConfig, path string, ctx context.Context) (string, string, error) {
	return splitPathInfo(conf.BaseDir, path, conf.Suffix)
}

type runnerCaptureEnv struct {
	env *map[string]string
}

func (runner runnerCaptureEnv) Run(conf SrvConfig, cmdname string, envvar map[string]string,
	stdin io.ReadCloser, stdout io.Writer, stderr io.Writer, ctx context.Context) error {
	*runner.env = envvar
	fmt.Fprintln(stdout, "Status: 200")
	fmt.Fprintln(stdout, "Content-Type: application/json")
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "{}")
	return nil
}

func (runner runnerCaptureEnv) Exists(conf SrvConfig, path string, ctx context.Context) (string, string, error) {
	return splitPathInfo(conf.BaseDir, path, conf.Suffix)
}

type runnerBadHeader struct{}

func (runner runnerBadHeader) Run(conf SrvConfig, cmdname string, envvar map[string]string,
	stdin io.ReadCloser, stdout io.Writer, stderr io.Writer, ctx context.Context) error {
	fmt.Fprintln(stdout, "no-colon-header-line")
	return nil
}

func (runner runnerBadHeader) Exists(conf SrvConfig, path string, ctx context.Context) (string, string, error) {
	return splitPathInfo(conf.BaseDir, path, conf.Suffix)
}

// captureLog swaps the default slog logger for the duration of fn and returns what was logged.
// Must run without t.Parallel(): other tests in this package call t.Parallel() as their first
// statement, which pauses them before they log anything, so a non-parallel test runs to
// completion (including this global swap) before any of them execute their bodies.
func captureLog(fn func()) string {
	var buf bytes.Buffer
	orig := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(orig)
	fn()
	return buf.String()
}

type spanRecorder struct {
	mu    sync.Mutex
	spans []sdktrace.ReadOnlySpan
}

func (s *spanRecorder) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spans = append(s.spans, spans...)
	return nil
}

func (s *spanRecorder) Shutdown(ctx context.Context) error {
	return nil
}

// TestRunBySetsRunSpanErrorOnRunnerFailure must not call t.Parallel(): it swaps the global
// TracerProvider, and every other test in this package calls t.Parallel() as its first
// statement, so they pause before emitting spans and this test's body runs to completion,
// restore included, before any of them execute (see captureLog for the same reasoning).
func TestRunBySetsRunSpanErrorOnRunnerFailure(t *testing.T) {
	rec := &spanRecorder{}
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(rec))
	origTP := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(origTP)
	defer tp.Shutdown(context.Background())

	opts := SrvConfig{}
	opts.Timeout = time.Duration(1000_000_000)
	opts.Addr = ":9999"
	opts.BaseDir = "."
	runner := runnerErr{}
	bio := bytes.NewBufferString("")
	w := writer{out: bio}
	u, _ := url.Parse("http://hello.world.example.com/exec_if_test.go/hello/world?a=b&c=123")
	r := http.Request{
		Method:     http.MethodGet,
		RemoteAddr: "127.0.0.1:9999",
		URL:        u,
		Proto:      "tcp",
		RequestURI: "/exec_if_test.go",
	}
	_ = RunBy(opts, runner, &w, &r)

	var found bool
	for _, sp := range rec.spans {
		if sp.Name() != "run" {
			continue
		}
		for _, kv := range sp.Attributes() {
			if string(kv.Key) == "script" {
				found = true
				if sp.Status().Code != codes.Error {
					t.Errorf("CGI execution span status = %v, want Error", sp.Status().Code)
				}
			}
		}
	}
	if !found {
		t.Fatal("did not find the CGI-execution 'run' span (with a script attribute)")
	}
}

func TestRunByLogsRunnerError(t *testing.T) {
	opts := SrvConfig{}
	opts.Timeout = time.Duration(1000_000_000)
	opts.Addr = ":9999"
	opts.BaseDir = "."
	runner := runnerErr{}
	bio := bytes.NewBufferString("")
	w := writer{out: bio}
	u, _ := url.Parse("http://hello.world.example.com/exec_if_test.go/hello/world?a=b&c=123")
	r := http.Request{
		Method:     http.MethodGet,
		RemoteAddr: "127.0.0.1:9999",
		URL:        u,
		Proto:      "tcp",
		RequestURI: "/exec_if_test.go",
	}
	logs := captureLog(func() {
		_ = RunBy(opts, runner, &w, &r)
	})
	if !strings.Contains(logs, "boom: script failed") {
		t.Errorf("log does not contain the runner error: %s", logs)
	}
	if !strings.Contains(logs, "script=") {
		t.Errorf("log does not identify which script failed: %s", logs)
	}
	if !strings.Contains(logs, "remote-addr=127.0.0.1:9999") {
		t.Errorf("log does not identify the requesting client: %s", logs)
	}
}

func TestRunByLogsOutputFilterError(t *testing.T) {
	opts := SrvConfig{}
	opts.Timeout = time.Duration(1000_000_000)
	opts.Addr = ":9999"
	opts.BaseDir = "."
	runner := runnerBadHeader{}
	bio := bytes.NewBufferString("")
	w := writer{out: bio}
	u, _ := url.Parse("http://hello.world.example.com/exec_if_test.go/hello/world?a=b&c=123")
	r := http.Request{
		Method:     http.MethodGet,
		RemoteAddr: "127.0.0.1:9999",
		URL:        u,
		Proto:      "tcp",
		RequestURI: "/exec_if_test.go",
	}
	logs := captureLog(func() {
		_ = RunBy(opts, runner, &w, &r)
	})
	if !strings.Contains(logs, "script=") {
		t.Errorf("output filter error log does not identify which script failed: %s", logs)
	}
	if !strings.Contains(logs, "remote-addr=127.0.0.1:9999") {
		t.Errorf("output filter error log does not identify the requesting client: %s", logs)
	}
}

func TestOutputFilterVerboseLogsAreDebugLevel(t *testing.T) {
	stdout := bytes.NewBufferString("Status: 200\nContent-Type: text/plain\n\nhello\n")
	bio := bytes.NewBufferString("")
	w := writer{out: bio}
	logs := captureLog(func() {
		if _, err := OutputFilter(stdout, w); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	})
	if strings.Contains(logs, "header finished") {
		t.Errorf(`"header finished" is logged at Info level, want Debug: %s`, logs)
	}
	if strings.Contains(logs, "status code update") {
		t.Errorf(`"status code update" is logged at Info level, want Debug: %s`, logs)
	}
}

func TestOutputFilterHeaderFormatError(t *testing.T) {
	t.Parallel()
	bio := bytes.NewBufferString("")
	w := writer{out: bio}
	stdout := bytes.NewBufferString("invalid-header-no-colon\nbody\n")
	status, err := OutputFilter(stdout, w)
	if err == nil {
		t.Error("expected error for malformed header line")
	}
	if status != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", status, http.StatusInternalServerError)
	}
	if !strings.Contains(bio.String(), "status code = 500") {
		t.Errorf("client did not receive a response: %q", bio.String())
	}
}

func TestRunBy(t *testing.T) {
	t.Parallel()
	opts := SrvConfig{}
	opts.Timeout = time.Duration(1000_000_000)
	opts.Addr = ":9999"
	opts.BaseDir = "."
	runner := runner1{}
	bio := bytes.NewBufferString("")
	w := writer{
		out: bio,
	}
	u, _ := url.Parse("http://hello.world.example.com/exec_if_test.go/hello/world?a=b&c=123")
	r := http.Request{
		Method:     http.MethodGet,
		RemoteAddr: "127.0.0.1:9999",
		URL:        u,
		Proto:      "tcp",
		RequestURI: "/exec_if_test.go",
	}
	err := RunBy(opts, runner, &w, &r)
	if err != nil {
		t.Errorf("error: %s", err)
	}
	res := w.out.String()
	expected := "status code = 200\n{\"hello\": true}\n"
	if res != expected {
		t.Errorf("status code %s != %s", res, expected)
	}
}

func TestMergeExtraEnvAddsVariable(t *testing.T) {
	t.Parallel()
	env := map[string]string{"REQUEST_METHOD": "GET"}
	if err := mergeExtraEnv(env, []string{"FOO=bar"}); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if env["FOO"] != "bar" {
		t.Errorf("env[FOO] = %q, want %q", env["FOO"], "bar")
	}
}

func TestMergeExtraEnvAddsMultipleVariables(t *testing.T) {
	t.Parallel()
	env := map[string]string{}
	if err := mergeExtraEnv(env, []string{"FOO=1", "BAR=2"}); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if env["FOO"] != "1" || env["BAR"] != "2" {
		t.Errorf("env = %v, want FOO=1 BAR=2", env)
	}
}

func TestMergeExtraEnvRejectsMalformedEntry(t *testing.T) {
	t.Parallel()
	env := map[string]string{}
	if err := mergeExtraEnv(env, []string{"NOEQUALSIGN"}); err == nil {
		t.Error("expected error for entry without '='")
	}
}

func TestMergeExtraEnvRejectsStandardVariableOverride(t *testing.T) {
	t.Parallel()
	env := map[string]string{"REQUEST_METHOD": "GET"}
	err := mergeExtraEnv(env, []string{"REQUEST_METHOD=POST"})
	if err == nil {
		t.Fatal("expected error overriding REQUEST_METHOD")
	}
	if env["REQUEST_METHOD"] != "GET" {
		t.Errorf("REQUEST_METHOD was overwritten to %q", env["REQUEST_METHOD"])
	}
}

func TestLoadEnvFilesReadsKeyValuePairs(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("FOO=bar\n# comment\n\nBAZ=qux\n"), 0o600); err != nil {
		t.Fatalf("write temp env file: %s", err)
	}
	extra, err := loadEnvFiles([]string{path})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !slices.Contains(extra, "FOO=bar") || !slices.Contains(extra, "BAZ=qux") {
		t.Errorf("extra = %v, want to contain FOO=bar and BAZ=qux", extra)
	}
}

func TestLoadEnvFilesMissingFile(t *testing.T) {
	t.Parallel()
	if _, err := loadEnvFiles([]string{filepath.Join(t.TempDir(), "does-not-exist.env")}); err == nil {
		t.Error("expected error for missing env file")
	}
}

func TestRunByLoadsEnvFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("FOO=bar\n"), 0o600); err != nil {
		t.Fatalf("write temp env file: %s", err)
	}
	opts := SrvConfig{}
	opts.Timeout = time.Duration(1000_000_000)
	opts.Addr = ":9999"
	opts.BaseDir = "."
	opts.EnvFile = []string{path}
	var captured map[string]string
	runner := runnerCaptureEnv{env: &captured}
	bio := bytes.NewBufferString("")
	w := writer{out: bio}
	u, _ := url.Parse("http://hello.world.example.com/exec_if_test.go/hello/world?a=b&c=123")
	r := http.Request{
		Method:     http.MethodGet,
		RemoteAddr: "127.0.0.1:9999",
		URL:        u,
		Proto:      "tcp",
		RequestURI: "/exec_if_test.go",
	}
	if err := RunBy(opts, runner, &w, &r); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if captured["FOO"] != "bar" {
		t.Errorf("envvar[FOO] = %q, want %q", captured["FOO"], "bar")
	}
}

func TestRunByErrorsOnMissingEnvFile(t *testing.T) {
	t.Parallel()
	opts := SrvConfig{}
	opts.Timeout = time.Duration(1000_000_000)
	opts.Addr = ":9999"
	opts.BaseDir = "."
	opts.EnvFile = []string{filepath.Join(t.TempDir(), "does-not-exist.env")}
	runner := runner1{}
	bio := bytes.NewBufferString("")
	w := writer{out: bio}
	u, _ := url.Parse("http://hello.world.example.com/exec_if_test.go/hello/world?a=b&c=123")
	r := http.Request{
		Method:     http.MethodGet,
		RemoteAddr: "127.0.0.1:9999",
		URL:        u,
		Proto:      "tcp",
		RequestURI: "/exec_if_test.go",
	}
	err := RunBy(opts, runner, &w, &r)
	if err == nil {
		t.Fatal("expected error for missing --env-file")
	}
	if !strings.Contains(w.out.String(), "status code = 500") {
		t.Errorf("client did not receive a 500 response: %q", w.out.String())
	}
}

func TestRunByErrorsOnEnvFileOverridingStandardVar(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("REQUEST_METHOD=POST\n"), 0o600); err != nil {
		t.Fatalf("write temp env file: %s", err)
	}
	opts := SrvConfig{}
	opts.Timeout = time.Duration(1000_000_000)
	opts.Addr = ":9999"
	opts.BaseDir = "."
	opts.EnvFile = []string{path}
	runner := runner1{}
	bio := bytes.NewBufferString("")
	w := writer{out: bio}
	u, _ := url.Parse("http://hello.world.example.com/exec_if_test.go/hello/world?a=b&c=123")
	r := http.Request{
		Method:     http.MethodGet,
		RemoteAddr: "127.0.0.1:9999",
		URL:        u,
		Proto:      "tcp",
		RequestURI: "/exec_if_test.go",
	}
	err := RunBy(opts, runner, &w, &r)
	if err == nil {
		t.Fatal("expected error when --env-file overrides a standard CGI variable")
	}
	if !strings.Contains(w.out.String(), "status code = 500") {
		t.Errorf("client did not receive a 500 response: %q", w.out.String())
	}
}

func TestRunByPassesExtraEnvToRunner(t *testing.T) {
	t.Parallel()
	opts := SrvConfig{}
	opts.Timeout = time.Duration(1000_000_000)
	opts.Addr = ":9999"
	opts.BaseDir = "."
	opts.Env = []string{"FOO=bar"}
	var captured map[string]string
	runner := runnerCaptureEnv{env: &captured}
	bio := bytes.NewBufferString("")
	w := writer{out: bio}
	u, _ := url.Parse("http://hello.world.example.com/exec_if_test.go/hello/world?a=b&c=123")
	r := http.Request{
		Method:     http.MethodGet,
		RemoteAddr: "127.0.0.1:9999",
		URL:        u,
		Proto:      "tcp",
		RequestURI: "/exec_if_test.go",
	}
	if err := RunBy(opts, runner, &w, &r); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if captured["FOO"] != "bar" {
		t.Errorf("envvar[FOO] = %q, want %q", captured["FOO"], "bar")
	}
}

func TestRunByErrorsOnStandardEnvOverride(t *testing.T) {
	t.Parallel()
	opts := SrvConfig{}
	opts.Timeout = time.Duration(1000_000_000)
	opts.Addr = ":9999"
	opts.BaseDir = "."
	opts.Env = []string{"REQUEST_METHOD=POST"}
	runner := runner1{}
	bio := bytes.NewBufferString("")
	w := writer{out: bio}
	u, _ := url.Parse("http://hello.world.example.com/exec_if_test.go/hello/world?a=b&c=123")
	r := http.Request{
		Method:     http.MethodGet,
		RemoteAddr: "127.0.0.1:9999",
		URL:        u,
		Proto:      "tcp",
		RequestURI: "/exec_if_test.go",
	}
	err := RunBy(opts, runner, &w, &r)
	if err == nil {
		t.Fatal("expected error when --env overrides a standard CGI variable")
	}
	if !strings.Contains(w.out.String(), "status code = 500") {
		t.Errorf("client did not receive a 500 response: %q", w.out.String())
	}
}

func TestRunByStatusCode(t *testing.T) {
	t.Parallel()
	opts := SrvConfig{}
	opts.Timeout = time.Duration(1000_000_000)
	opts.Addr = ":9999"
	opts.BaseDir = "."
	runner := runner2{}
	bio := bytes.NewBufferString("")
	w := writer{
		out: bio,
	}
	u, _ := url.Parse("http://hello.world.example.com/exec_if_test.go/hello/world?a=b&c=123")
	r := http.Request{
		Method:     http.MethodGet,
		RemoteAddr: "127.0.0.1:9999",
		URL:        u,
		Proto:      "tcp",
		RequestURI: "/exec_if_test.go",
	}
	err := RunBy(opts, runner, w, &r)
	if err != nil {
		t.Errorf("error: %s", err)
	}
	res := w.out.String()
	expected := "status code = 500\n{\"hello\": true}\n"
	if res != expected {
		t.Errorf("status code %s != %s", res, expected)
	}
}
