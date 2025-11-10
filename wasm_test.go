//go:build wazero || wasmtime || wasmer

package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func testWasmOpenError(t *testing.T, runner Runner) {
	conf := SrvConfig{SrvConfigBase{Timeout: time.Duration(1000_000_000)}}
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

func testWasmHello(t *testing.T, runner Runner) {
	conf := SrvConfig{}
	conf.Timeout = time.Duration(1000_000_000)
	conf.BaseDir = "examples"
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

func testWasmAll(t *testing.T, runner Runner) {
	t.Parallel()
	t.Run("Hello", func(t *testing.T) {
		t.Parallel()
		testWasmHello(t, runner)
	})
	t.Run("OpenError", func(t *testing.T) {
		t.Parallel()
		testWasmOpenError(t, runner)
	})
}
