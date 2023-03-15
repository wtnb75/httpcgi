package main

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sync"

	"golang.org/x/exp/slog"
)

// OsRunner is normal CGI executor
type OsRunner struct {
}

func (runner *OsRunner) getPipe(cmd *exec.Cmd) (
	stdin io.WriteCloser, stdout io.ReadCloser, stderr io.ReadCloser, err error) {
	if stdin, err = cmd.StdinPipe(); err != nil {
		slog.Error("stdin", err)
		return
	}
	if stdout, err = cmd.StdoutPipe(); err != nil {
		slog.Error("stdout", err)
		return
	}
	if stderr, err = cmd.StderrPipe(); err != nil {
		slog.Error("stderr", err)
		return
	}
	return
}

// Run implements Runner.Run
func (runner *OsRunner) Run(conf SrvConfig, cmdname string, envvar map[string]string,
	stdin io.ReadCloser, stdout io.Writer, stderr io.Writer, ctx context.Context) error {
	fn := filepath.Join(conf.BaseDir, cmdname)
	slog.Debug("path", "full-path", fn)
	cmd := exec.Command(fn)
	slog.Debug("pid", "process", cmd.Process)
	cmdStdin, cmdStdout, cmdStderr, err := runner.getPipe(cmd)
	if err != nil {
		slog.Error("pipe error", err)
		return err
	}
	defer cmdStdin.Close()
	defer cmdStdout.Close()
	defer cmdStderr.Close()
	for k, v := range envvar {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	slog.Debug("starting command", "cmd", cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	slog.Debug("pid", "process", cmd.Process)
	defer func() {
		if err := cmd.Wait(); err != nil {
			slog.Error("wait", err)
		}
	}()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		if err := DoPipe(stdin, cmdStdin, &wg); err != nil {
			slog.Error("stdin", err)
		}
	}()
	wg.Add(1)
	go func() {
		if err := DoPipe(cmdStderr, stderr, &wg); err != nil {
			slog.Error("stderr", err)
		}
	}()
	wg.Add(1)
	go func() {
		if err := DoPipe(cmdStdout, stdout, &wg); err != nil {
			slog.Error("stdout", err)
		}
	}()
	wg.Wait()
	return nil
}

func (runner OsRunner) Exists(conf SrvConfig, path string, ctx context.Context) (string, string, error) {
	return splitPathInfo(conf.BaseDir, path, conf.Suffix)
}

func init() {
	runnerMap["os"] = func(SrvConfig) Runner {
		return &OsRunner{}
	}
}
