//go:build wasmtime
// +build wasmtime

package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/bytecodealliance/wasmtime-go"
)

// WasmtimeRunner implements CGI Runner execute by wasmtime
type WasmtimeRunner struct {
}

// Run implements Runner.Run
func (runner WasmtimeRunner) Run(conf SrvConfig, cmdname string, envvar map[string]string,
	stdin io.ReadCloser, stdout io.Writer, stderr io.Writer, ctx context.Context) error {
	fn := filepath.Join(conf.BaseDir, cmdname)
	bytecode, err := os.ReadFile(fn)
	if err != nil {
		slog.Error("read bytecode", "error", err, "filename", fn)
		return err
	}
	slog.Debug("bytecode read", "length", len(bytecode), "filename", fn)
	engine := wasmtime.NewEngine()
	module, err := wasmtime.NewModule(engine, bytecode)
	if err != nil {
		slog.Error("wasmtime module", "error", err)
		return err
	}
	linker := wasmtime.NewLinker(engine)
	err = linker.DefineWasi()
	if err != nil {
		slog.Error("define wasi", "error", err)
		return err
	}
	wasiConfig := wasmtime.NewWasiConfig()
	keys := []string{}
	vals := []string{}
	for k, v := range envvar {
		keys = append(keys, k)
		vals = append(vals, v)
	}
	wasiConfig.SetEnv(keys, vals)
	dir, err := os.MkdirTemp("", "out")
	if err != nil {
		slog.Error("mkdtemp", "error", err)
		return err
	}
	defer os.RemoveAll(dir)
	slog.Debug("tmp dir", "dir", dir)
	stdoutPath := filepath.Join(dir, "stdout")
	stderrPath := filepath.Join(dir, "stderr")
	stdinPath := filepath.Join(dir, "stdin")
	stdinFp, err := os.OpenFile(stdinPath, os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		slog.Error("open stdin", "error", err)
		return err
	}
	io.Copy(stdinFp, stdin)
	err = stdinFp.Close()
	if err != nil {
		slog.Error("stdin close", "error", err)
		return err
	}
	wasiConfig.SetStdinFile(stdinPath)
	wasiConfig.SetStdoutFile(stdoutPath)
	wasiConfig.SetStderrFile(stderrPath)
	store := wasmtime.NewStore(engine)
	store.SetWasi(wasiConfig)
	instance, err := linker.Instantiate(store, module)
	if err != nil {
		slog.Error("wasmtime instantiate", "error", err)
		return err
	}
	cgi := instance.GetFunc(store, "_start")
	res, err := cgi.Call(store)
	if err != nil {
		slog.Error("wasmtime run", "error", err)
	}
	if res != nil {
		slog.Debug("result", "res", res)
	} else {
		slog.Debug("result(nil)")
	}
	stdoutContent, err := os.ReadFile(stdoutPath)
	if err != nil {
		slog.Error("reading stdout", "error", err)
		return err
	}
	cnt, err := stdout.Write(stdoutContent)
	if err != nil {
		slog.Error("writing stdout", "error", err)
	} else {
		slog.Debug("stdout", "cnt", cnt)
	}
	stderrContent, err := os.ReadFile(stderrPath)
	if err != nil {
		slog.Error("reading stderr", "error", err)
	} else if len(stderrContent) != 0 {
		cnt, err := stderr.Write(stderrContent)
		if err != nil {
			slog.Error("writing stderr", "error", err, "content", string(stderrContent))
		} else {
			slog.Debug("stderr", "cnt", cnt, "content", string(stderrContent))
		}
	}
	return nil
}

func (runner WasmtimeRunner) Exists(conf SrvConfig, path string, ctx context.Context) (string, string, error) {
	return splitPathInfo(conf.BaseDir, path, conf.Suffix)
}

func init() {
	runnerMap["wasmtime"] = func(SrvConfig) Runner {
		return WasmtimeRunner{}
	}
}
