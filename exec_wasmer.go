//go:build wasmer
// +build wasmer

package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/wasmerio/wasmer-go/wasmer"
)

// WasmerRunner implements CGI Runner execute by wasmer
type WasmerRunner struct {
}

func (runner *WasmerRunner) pipeStdout(wasiEnv *wasmer.WasiEnvironment, output io.Writer, wg *sync.WaitGroup) error {
	if wg != nil {
		defer wg.Done()
	}
	data := wasiEnv.ReadStdout()
	slog.Info("stdout", "length", len(data))
	dlen, err := output.Write(data)
	if err != nil {
		slog.Error("output", "error", err)
		return err
	}
	slog.Debug("output", "length", dlen)
	return nil
}

func (runner *WasmerRunner) pipeStderr(wasiEnv *wasmer.WasiEnvironment, output io.Writer, wg *sync.WaitGroup) error {
	if wg != nil {
		defer wg.Done()
	}
	data := wasiEnv.ReadStderr()
	slog.Info("stderr", "length", len(data))
	dlen, err := output.Write(data)
	if err != nil {
		slog.Error("output(err)", "error", err)
		return err
	}
	slog.Debug("output(err)", "length", dlen)
	return nil
}

// Run implements Runner.Run
func (runner *WasmerRunner) Run(conf SrvConfig, cmdname string, envvar map[string]string,
	stdin io.ReadCloser, stdout io.Writer, stderr io.Writer, ctx context.Context) error {
	fn := filepath.Join(conf.BaseDir, cmdname)
	bytecode, err := os.ReadFile(fn)
	if err != nil {
		slog.Error("read bytecode", "error", err, "filename", fn)
		return err
	}
	slog.Debug("bytecode read", "length", len(bytecode), "filename", fn)
	bld := wasmer.NewWasiStateBuilder(cmdname)
	for k, v := range envvar {
		bld = bld.Environment(k, v)
	}
	wasiEnv, err := bld.MapDirectory(conf.BaseDir, ".").CaptureStdout().CaptureStderr().Finalize()
	if err != nil {
		slog.Error("build wasi", "error", err)
		return err
	}
	engine := wasmer.NewEngine()
	store := wasmer.NewStore(engine)
	module, err := wasmer.NewModule(store, bytecode)
	if err != nil {
		slog.Error("wasmer module", "error", err)
		return err
	}
	slog.Debug("wasi", "version", wasmer.GetWasiVersion(module).String())
	importObj, err := wasiEnv.GenerateImportObject(store, module)
	if err != nil {
		slog.Error("import object", "error", err)
		return err
	}
	instance, err := wasmer.NewInstance(module, importObj)
	if err != nil {
		slog.Error("new instance", "error", err)
		return err
	}
	start, err := instance.Exports.GetWasiStartFunction()
	if err != nil {
		slog.Error("wasi start", "error", err)
	}
	slog.Debug("run wasi")
	res, err := start()
	if err != nil {
		slog.Error("wasi function returns error", "error", err)
		return err
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go runner.pipeStdout(wasiEnv, stdout, &wg)
	wg.Add(1)
	go runner.pipeStderr(wasiEnv, stderr, &wg)
	if timeoutWait(&wg, conf.Timeout) {
		slog.Warn("timeout")
		return nil
	}
	if res != nil {
		slog.Debug("wasi success", "res", res)
	} else {
		slog.Debug("wasi success(empty)")
	}
	return nil
}

func (runner WasmerRunner) Exists(conf SrvConfig, path string, ctx context.Context) (string, string, error) {
	return splitPathInfo(conf.BaseDir, path, conf.Suffix)
}

func init() {
	runnerMap["wasmer"] = func(SrvConfig) Runner {
		return &WasmerRunner{}
	}
}
