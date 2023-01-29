package main

import (
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
	stdin io.ReadCloser, stdout io.Writer, stderr io.Writer) error {
	fn := filepath.Join(conf.BaseDir, cmdname)
	slog.Debug("path", "full-path", fn)
	cmd := exec.Command(fn)
	slog.Debug("pid", "process", cmd.Process)
	cmdStdin, cmdStdout, cmdStderr, err := runner.getPipe(cmd)
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
	go DoPipe(stdin, cmdStdin, &wg)
	wg.Add(1)
	go DoPipe(cmdStderr, stderr, &wg)
	wg.Add(1)
	go DoPipe(cmdStdout, stdout, &wg)
	wg.Wait()
	cmdStdin.Close()
	cmdStderr.Close()
	return nil
}
