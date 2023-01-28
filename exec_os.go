package main

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sync"

	"golang.org/x/exp/slog"
)

type OsRunner struct {
}

func (runner *OsRunner) getPipe(cmd *exec.Cmd) (stdin io.WriteCloser, stdout io.ReadCloser, stderr io.ReadCloser, err error) {
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

func (runner *OsRunner) Run(conf SrvConfig, cmdname string, envvar map[string]string, stdin io.ReadCloser, stdout io.Writer, stderr io.Writer) error {
	fn := filepath.Join(conf.BaseDir, cmdname)
	slog.Debug("path", "full-path", fn)
	cmd := exec.Command(fn)
	slog.Debug("pid", "process", cmd.Process)
	cmd_stdin, cmd_stdout, cmd_stderr, err := runner.getPipe(cmd)
	if err != nil {
		slog.Error("pipe error", err)
		return err
	}
	for k, v := range envvar {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	slog.Debug("starting command", "cmd", cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	slog.Debug("pid", "process", cmd.Process)
	var wg sync.WaitGroup
	wg.Add(1)
	go DoPipe(stdin, cmd_stdin, &wg)
	wg.Add(1)
	go DoPipe(cmd_stderr, stderr, &wg)
	wg.Add(1)
	go DoPipe(cmd_stdout, stdout, &wg)
	wg.Wait()
	cmd_stdin.Close()
	cmd_stderr.Close()
	return nil
}
