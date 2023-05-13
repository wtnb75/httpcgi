// httpcgi: serve legacy CGI
package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"

	"github.com/jessevdk/go-flags"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/exp/slog"
)

var (
	opts      SrvConfig
	runner    Runner
	runnerMap = map[string]interface{}{}
	version   = "dev"
	commit    = "none"
	date      = "unknown"
)

type cgiHandler struct{}

func main() {
	args, err := flags.ParseArgs(&opts, os.Args)
	if opts.Version {
		fmt.Println("httpcgi version", version, "commit", commit, "build", date)
		fmt.Println("runners:", reflect.ValueOf(runnerMap).MapKeys())
		return
	}
	var logopt slog.HandlerOptions
	if opts.Verbose {
		logopt = slog.HandlerOptions{Level: slog.LevelDebug, AddSource: true}
	} else if opts.Quiet {
		logopt = slog.HandlerOptions{Level: slog.LevelWarn}
	} else {
		logopt = slog.HandlerOptions{}
	}
	if opts.JSONLog {
		lh := slog.NewJSONHandler(os.Stdout, &logopt)
		slog.SetDefault(slog.New(lh))
	} else {
		lh := slog.NewTextHandler(os.Stdout, &logopt)
		slog.SetDefault(slog.New(lh))
	}
	slog.Debug("start0", "args", args, "opts", opts)
	if err != nil {
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
			slog.Error("basedir not found", "error", err)
		}
	}
	if opts.Runner != "docker" {
		opts.BaseDir, err = filepath.Abs(opts.BaseDir)
	}
	if err != nil {
		slog.Error("abs", "error", err)
	}
	if opts.OtelProvider == "stdout" {
		if fin, err := initOtelStdout(); err != nil {
			slog.Error("otel-stdout", "error", err)
		} else {
			defer fin()
		}
	} else if opts.OtelProvider == "jaeger" {
		if fin, err := initOtelJaeger(); err != nil {
			slog.Error("otel-jaeger", "error", err)
		} else {
			defer fin()
		}
	} else if opts.OtelProvider == "zipkin" {
		if fin, err := initOtelZipkin(); err != nil {
			slog.Error("otel-zipkin", "error", err)
		} else {
			defer fin()
		}
	} else if opts.OtelProvider == "otlp" {
		if fin, err := initOtelOtlp(); err != nil {
			slog.Error("otel-otlp", "error", err)
		} else {
			defer fin()
		}
	} else if opts.OtelProvider == "otlp-http" {
		if fin, err := initOtelOtlpHttp(); err != nil {
			slog.Error("otel-otlp-http", "error", err)
		} else {
			defer fin()
		}
	}
	var mux http.ServeMux
	var hdl http.Handler
	hdl = new(cgiHandler)
	mux.Handle("/", hdl)
	if opts.OtelProvider != "" {
		hdl = otelhttp.NewHandler(
			&mux, "httpcgi", otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents))
	}
	http.Handle(opts.Prefix, hdl)

	server := http.Server{
		Addr:    opts.Addr,
		Handler: nil,
	}
	l, err := net.Listen(opts.Proto, opts.Addr)
	if err != nil {
		slog.Error("listen", "error", err)
		return
	}
	slog.Info("listen", "addr", l.Addr(), "version", version, "commit", commit, "build-date", date)
	if err := server.Serve(l); err != nil {
		slog.Error("serve", "error", err)
		return
	}
}

func (h *cgiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := RunBy(opts, runner, w, r)
	if err != nil {
		slog.Error("runby", "error", err)
	}
}
