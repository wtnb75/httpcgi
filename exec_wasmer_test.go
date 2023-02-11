//go:build wasmer
// +build wasmer

package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestWasmerOpenError(t *testing.T) {
	t.Parallel()
	runner := &WasmerRunner{}
	conf := SrvConfig{}
	fname := "test.wasm"
	stdin := io.NopCloser(bytes.NewBufferString(""))
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	envvar := map[string]string{}
	err := runner.Run(conf, fname, envvar, stdin, stdout, stderr)
	if err == nil {
		t.Error("no-error")
	}
	t.Log("err", err)
}

func TestWasmerHello(t *testing.T) {
	t.Parallel()
	runner := &WasmerRunner{}
	conf := SrvConfig{SrvConfigBase{BaseDir: "examples"}}
	fname := "hello.wasm"
	stdin := io.NopCloser(bytes.NewBufferString(""))
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	envvar := map[string]string{}
	err := runner.Run(conf, fname, envvar, stdin, stdout, stderr)
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
