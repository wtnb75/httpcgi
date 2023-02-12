//go:build wasmtime
// +build wasmtime

package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

func TestWasmtimeOpenError(t *testing.T) {
	t.Parallel()
	runner := &WasmtimeRunner{}
	conf := SrvConfig{}
	fname := "test.wasm"
	stdin := io.NopCloser(bytes.NewBufferString(""))
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	envvar := map[string]string{}
	err := runner.Run(conf, fname, envvar, stdin, stdout, stderr, context.Background())
	if err == nil {
		t.Error("no-error")
	}
	t.Log("err", err)
}

func TestWasmtimeHello(t *testing.T) {
	t.Parallel()
	runner := &WasmtimeRunner{}
	conf := SrvConfig{SrvConfigBase{BaseDir: "examples"}}
	fname := "hello.wasm"
	stdin := io.NopCloser(bytes.NewBufferString(""))
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	envvar := map[string]string{}
	err := runner.Run(conf, fname, envvar, stdin, stdout, stderr, context.Background())
	if err != nil {
		t.Errorf("error %s", err)
	}
	if stderr.String() != "finished(stderr).\n" {
		t.Errorf("stderr %s", stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "Content-Type:") {
		t.Errorf("stdout %s", stdout.String())
	}
}
