package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOsExists(t *testing.T) {
	t.Parallel()
	runner := OsRunner{}
	conf := SrvConfig{SrvConfigBase{Timeout: time.Duration(1000_000_000)}}
	tmpd, err := os.MkdirTemp("", "")
	if err != nil {
		t.Error("tmpdir", err)
	}
	defer os.RemoveAll(tmpd)
	conf.BaseDir = tmpd
	ctx := context.Background()
	_, _, err = runner.Exists(conf, "notexists", ctx)
	if err == nil {
		t.Error("not error", err)
	}
	if err = os.WriteFile(filepath.Join(tmpd, "exists"), []byte(""), 0755); err != nil {
		t.Error("writefile", err)
	}
	res1, res2, err := runner.Exists(conf, "exists", ctx)
	if err != nil {
		t.Error("not error", err)
	}
	if res1 != "exists" {
		t.Error("mismatch(script)", res1)
	}
	if res2 != "" {
		t.Error("mismatch(pathinfo)", res2)
	}
	res1_2, res2_2, err := runner.Exists(conf, "exists/hello/world", ctx)
	if err != nil {
		t.Error("not error", err)
	}
	if res1_2 != "exists" {
		t.Error("mismatch(script)", res1)
	}
	if res2_2 != "/hello/world" {
		t.Error("mismatch(pathinfo)", res2)
	}
}

func TestOsRun(t *testing.T) {
	t.Parallel()
	runner := OsRunner{}
	conf := SrvConfig{SrvConfigBase{Timeout: time.Duration(1000_000_000)}}
	tmpd, err := os.MkdirTemp("", "")
	if err != nil {
		t.Error("tmpdir", err)
	}
	defer os.RemoveAll(tmpd)
	conf.BaseDir = tmpd
	ctx := context.Background()
	if err = os.WriteFile(filepath.Join(tmpd, "cmd1"), []byte("#! /bin/sh\n"), 0755); err != nil {
		t.Error("writefile", err)
	}
	env := map[string]string{}
	stdin := io.NopCloser(&bytes.Buffer{})
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	err = runner.Run(conf, "cmd1", env, stdin, stdout, stderr, ctx)
	if err != nil {
		t.Error("error", err)
	}
	if stdout.Len() != 0 {
		t.Error("out", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Error("out", stderr.String())
	}
}

func TestOsRunTimeout(t *testing.T) {
	t.Parallel()
	runner := OsRunner{}
	conf := SrvConfig{}
	tmpd, err := os.MkdirTemp("", "")
	if err != nil {
		t.Error("tmpdir", err)
	}
	defer os.RemoveAll(tmpd)
	conf.BaseDir = tmpd
	ctx := context.Background()
	if err = os.WriteFile(filepath.Join(tmpd, "cmd1"), []byte("#! /bin/sh\nsleep 10"), 0755); err != nil {
		t.Error("writefile", err)
	}
	env := map[string]string{}
	stdin := io.NopCloser(&bytes.Buffer{})
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	err = runner.Run(conf, "cmd1", env, stdin, stdout, stderr, ctx)
	if err == nil {
		t.Error("no timeout ?")
	}
}
