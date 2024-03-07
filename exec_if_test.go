package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"
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
	wg.Add(1)
	err := DoPipe(rd, wr, &wg)
	wg.Wait()
	if err != nil {
		t.Errorf("pipe error: %s", err)
	}
}

func TestDoPipeReadClose(t *testing.T) {
	t.Parallel()
	rd, wr := io.Pipe()
	var wg sync.WaitGroup
	rd.Close()
	wg.Add(1)
	err := DoPipe(rd, wr, &wg)
	wg.Wait()
	if err == nil {
		t.Error("pipe no-error")
	}
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
