//go:build wasmtime

package main

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"
)

func TestWasmtime(t *testing.T) {
	testWasmAll(t, &WasmtimeRunner{})
}

type errStdin struct{}

func (errStdin) Read(p []byte) (int, error) {
	return 0, fmt.Errorf("read boom")
}

func (errStdin) Close() error {
	return nil
}

func TestWasmtimeStdinCopyError(t *testing.T) {
	t.Parallel()
	conf := SrvConfig{}
	conf.Timeout = time.Duration(1000_000_000)
	conf.BaseDir = "examples"
	runner := WasmtimeRunner{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	err := runner.Run(conf, "hello.wasm", map[string]string{}, errStdin{}, stdout, stderr, context.Background())
	if err == nil {
		t.Error("expected error from stdin copy failure, got nil")
	}
}
