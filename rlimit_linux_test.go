//go:build linux

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

// TestOsRunRlimitCPUKillsBusyLoop exercises the real Linux prlimit(2) implementation
// (not a fake): a CPU-bound busy loop under a 1-second RLIMIT_CPU should be killed by
// the kernel (SIGXCPU) well before conf.Timeout, proving the limit is actually enforced.
func TestOsRunRlimitCPUKillsBusyLoop(t *testing.T) {
	runner := OsRunner{}
	conf := SrvConfig{}
	conf.Timeout = 10 * time.Second
	conf.RlimitCPU = 1
	tmpd, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatal("tmpdir", err)
	}
	defer os.RemoveAll(tmpd)
	conf.BaseDir = tmpd
	script := "#!/bin/sh\nwhile true; do :; done\n"
	if err := os.WriteFile(filepath.Join(tmpd, "busy"), []byte(script), 0755); err != nil {
		t.Fatal("writefile", err)
	}

	env := map[string]string{}
	stdin := io.NopCloser(&bytes.Buffer{})
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	start := time.Now()
	_ = runner.Run(conf, "busy", env, stdin, stdout, stderr, context.Background())
	elapsed := time.Since(start)

	if elapsed >= conf.Timeout {
		t.Errorf("process ran for %s (>= timeout %s): RLIMIT_CPU=1s does not appear to have been enforced", elapsed, conf.Timeout)
	}
}
