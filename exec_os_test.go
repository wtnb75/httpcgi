package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestOsExists(t *testing.T) {
	t.Parallel()
	runner := OsRunner{}
	conf := SrvConfig{}
	conf.Timeout = time.Duration(1000_000_000)
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
	conf := SrvConfig{}
	conf.Timeout = time.Duration(1000_000_000)
	tmpd, err := os.MkdirTemp("/var/tmp", "")
	if err != nil {
		t.Error("tmpdir", err)
	}
	conf.BaseDir = tmpd
	ctx := context.Background()
	if err = os.WriteFile(filepath.Join(tmpd, "cmd1"), []byte("#! /bin/sh\n"), 0755); err != nil {
		t.Error("writefile", err)
	}
	env := map[string]string{}
	stdin := io.NopCloser(&bytes.Buffer{})
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	if err = runner.Run(conf, "cmd1", env, stdin, stdout, stderr, ctx); err != nil {
		t.Error("error", err)
	}
	if stdout.Len() != 0 {
		t.Error("out", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Error("out", stderr.String())
	}
}

func TestOsRunStartErrorLogged(t *testing.T) {
	runner := OsRunner{}
	conf := SrvConfig{}
	conf.Timeout = time.Duration(1000_000_000)
	tmpd, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatal("tmpdir", err)
	}
	defer os.RemoveAll(tmpd)
	conf.BaseDir = tmpd
	ctx := context.Background()
	env := map[string]string{}
	stdin := io.NopCloser(&bytes.Buffer{})
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	logs := captureLog(func() {
		err = runner.Run(conf, "does-not-exist", env, stdin, stdout, stderr, ctx)
	})
	if err == nil {
		t.Fatal("expected error for a command that cannot start")
	}
	if !strings.Contains(logs, "cmd=does-not-exist") {
		t.Errorf("log does not identify which command failed to start: %s", logs)
	}
}

// These rlimit tests must not call t.Parallel(): they override the package-level
// setProcessRlimits var, and every other test in this package calls t.Parallel() as its
// first statement, so this test's body (including the restore) runs to completion before
// any of them execute (see captureLog in exec_if_test.go for the same reasoning).

func TestOsRunAppliesRlimitsWhenConfigured(t *testing.T) {
	runner := OsRunner{}
	conf := SrvConfig{}
	conf.Timeout = time.Duration(1000_000_000)
	conf.RlimitCPU = 5
	conf.RlimitMem = 1000
	tmpd, err := os.MkdirTemp("/var/tmp", "")
	if err != nil {
		t.Fatal("tmpdir", err)
	}
	defer os.RemoveAll(tmpd)
	conf.BaseDir = tmpd
	if err := os.WriteFile(filepath.Join(tmpd, "cmd1"), []byte("#! /bin/sh\n"), 0755); err != nil {
		t.Fatal("writefile", err)
	}

	var gotPid int
	var gotCPU, gotMem int64
	orig := setProcessRlimits
	setProcessRlimits = func(pid int, cpuSeconds, memBytes int64) error {
		gotPid = pid
		gotCPU = cpuSeconds
		gotMem = memBytes
		return nil
	}
	defer func() { setProcessRlimits = orig }()

	env := map[string]string{}
	stdin := io.NopCloser(&bytes.Buffer{})
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	if err := runner.Run(conf, "cmd1", env, stdin, stdout, stderr, context.Background()); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if gotPid <= 0 {
		t.Errorf("setProcessRlimits was not called with a valid pid: %d", gotPid)
	}
	if gotCPU != 5 || gotMem != 1000 {
		t.Errorf("setProcessRlimits called with cpu=%d mem=%d, want 5/1000", gotCPU, gotMem)
	}
}

func TestOsRunSkipsRlimitsWhenNotConfigured(t *testing.T) {
	runner := OsRunner{}
	conf := SrvConfig{}
	conf.Timeout = time.Duration(1000_000_000)
	tmpd, err := os.MkdirTemp("/var/tmp", "")
	if err != nil {
		t.Fatal("tmpdir", err)
	}
	defer os.RemoveAll(tmpd)
	conf.BaseDir = tmpd
	if err := os.WriteFile(filepath.Join(tmpd, "cmd1"), []byte("#! /bin/sh\n"), 0755); err != nil {
		t.Fatal("writefile", err)
	}

	called := false
	orig := setProcessRlimits
	setProcessRlimits = func(pid int, cpuSeconds, memBytes int64) error {
		called = true
		return nil
	}
	defer func() { setProcessRlimits = orig }()

	env := map[string]string{}
	stdin := io.NopCloser(&bytes.Buffer{})
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	if err := runner.Run(conf, "cmd1", env, stdin, stdout, stderr, context.Background()); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if called {
		t.Error("setProcessRlimits should not be called when RlimitCPU/RlimitMem are unset")
	}
}

func TestOsRunKillsProcessOnRlimitFailure(t *testing.T) {
	runner := OsRunner{}
	conf := SrvConfig{}
	conf.Timeout = time.Duration(1000_000_000)
	conf.RlimitCPU = 5
	tmpd, err := os.MkdirTemp("/var/tmp", "")
	if err != nil {
		t.Fatal("tmpdir", err)
	}
	defer os.RemoveAll(tmpd)
	conf.BaseDir = tmpd
	if err := os.WriteFile(filepath.Join(tmpd, "cmd1"), []byte("#! /bin/sh\nsleep 10\n"), 0755); err != nil {
		t.Fatal("writefile", err)
	}

	var gotPid int
	orig := setProcessRlimits
	setProcessRlimits = func(pid int, cpuSeconds, memBytes int64) error {
		gotPid = pid
		return fmt.Errorf("rlimit boom")
	}
	defer func() { setProcessRlimits = orig }()

	env := map[string]string{}
	stdin := io.NopCloser(&bytes.Buffer{})
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	err = runner.Run(conf, "cmd1", env, stdin, stdout, stderr, context.Background())
	if err == nil {
		t.Fatal("expected error when setProcessRlimits fails")
	}
	if gotPid <= 0 {
		t.Fatal("setProcessRlimits was not called")
	}
	if procErr := syscall.Kill(gotPid, 0); procErr == nil {
		t.Errorf("process %d is still alive after rlimit failure", gotPid)
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
