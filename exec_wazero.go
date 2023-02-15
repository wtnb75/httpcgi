//go:build wazero
// +build wazero

package main

import (
	"context"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"golang.org/x/exp/slog"
	"io"
	"os"
	"path/filepath"
)

// WazeroRunner implements CGI Runner execute by wasmer
type WazeroRunner struct {
}

func (runner WazeroRunner) Run(conf SrvConfig, cmdname string, envvar map[string]string,
	stdin io.ReadCloser, stdout io.Writer, stderr io.Writer,
	ctx context.Context) error {
	fn := filepath.Join(conf.BaseDir, cmdname)
	bytecode, err := os.ReadFile(fn)
	if err != nil {
		slog.Error("read bytecode", err, "filename", fn)
		return err
	}
	slog.Debug("bytecode read", "length", len(bytecode), "filename", fn)
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)
	wconf := wazero.NewModuleConfig().
		WithStdout(stdout).
		WithStderr(stderr).
		WithStdin(stdin).
		WithStartFunctions("_start")
	for k, v := range envvar {
		wconf = wconf.WithEnv(k, v)
	}
	code, err := rt.CompileModule(ctx, bytecode)
	if err != nil {
		slog.Error("compile", err)
		return err
	}
	wasi_snapshot_preview1.MustInstantiate(ctx, rt)
	_, err = rt.InstantiateModule(ctx, code, wconf)
	if err != nil {
		slog.Error("instantiate", err)
	}
	return err
}

func (runner WazeroRunner) Exists(conf SrvConfig, path string, ctx context.Context) (string, string, error) {
	return splitPathInfo(conf.BaseDir, path, conf.Suffix)
}

func init() {
	runnerMap["wazero"] = func(SrvConfig) Runner {
		return WazeroRunner{}
	}
}
