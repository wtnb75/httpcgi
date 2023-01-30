// httpcgi: serve legacy CGI
package main

import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"

	"github.com/jessevdk/go-flags"
	"golang.org/x/exp/slog"
)

var opts SrvConfig
var runner Runner
var runnerMap map[string]interface{} = map[string]interface{}{}

func main() {
	args, err := flags.ParseArgs(&opts, os.Args)
	var logopt slog.HandlerOptions
	if opts.Verbose {
		logopt = slog.HandlerOptions{Level: slog.LevelDebug, AddSource: true}
	} else {
		logopt = slog.HandlerOptions{}
	}
	if opts.JSONLog {
		lh := logopt.NewJSONHandler(os.Stdout)
		slog.SetDefault(slog.New(lh))
	} else {
		lh := logopt.NewTextHandler(os.Stdout)
		slog.SetDefault(slog.New(lh))
	}
	slog.Debug("start0", "args", args, "opts", opts)
	if err != nil {
		slog.Error("flag parse", err)
		return
	}
	runnerFn, ok := runnerMap[opts.Runner]
	if !ok {
		slog.Warn("unknown runner", "runner", opts.Runner, "available", reflect.ValueOf(runnerMap).MapKeys())
		return
	}
	runner = runnerFn.(func(SrvConfig) Runner)(opts)
	slog.Info("runner", "name", opts.Runner, "type", reflect.TypeOf(runner), "val", runner)
	if opts.BaseDir == "" {
		opts.BaseDir, err = os.Getwd()
		if err != nil {
			slog.Error("basedir not found", err)
		}
	}
	opts.BaseDir, err = filepath.Abs(opts.BaseDir)
	if err != nil {
		slog.Error("abs", err)
	}
	http.HandleFunc(opts.Prefix, defaultRoute)

	server := http.Server{
		Addr:    opts.Addr,
		Handler: nil,
	}
	l, err := net.Listen(opts.Proto, opts.Addr)
	if err != nil {
		slog.Error("listen", err)
		return
	}
	slog.Info("listen", "addr", l.Addr())
	if err := server.Serve(l); err != nil {
		slog.Error("serve", err)
		return
	}
}

func defaultRoute(w http.ResponseWriter, r *http.Request) {
	err := RunBy(opts, runner, w, r)
	if err != nil {
		slog.Error("runby", err)
	}
}
