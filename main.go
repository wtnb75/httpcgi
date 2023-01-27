package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/jessevdk/go-flags"
	"golang.org/x/exp/slog"
)

var opts SrvConfig
var runner Runner

func main() {
	args, err := flags.ParseArgs(&opts, os.Args)
	var logopt slog.HandlerOptions
	if opts.Verbose {
		logopt = slog.HandlerOptions{Level: slog.LevelDebug}
	} else {
		logopt = slog.HandlerOptions{}
	}
	if opts.JsonLog {
		lh := logopt.NewJSONHandler(os.Stdout)
		slog.SetDefault(slog.New(lh))
	} else {
		lh := logopt.NewTextHandler(os.Stdout)
		slog.SetDefault(slog.New(lh))
	}
	slog.Info("start0", "args", args, "opts", opts)
	if err != nil {
		slog.Error("flag parse", err)
		return
	}
	if opts.Wasm {
		runner = &WasmerRunner{}
	} else {
		runner = &OsRunner{}
	}
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
	bn := strings.TrimPrefix(r.URL.Path, opts.Prefix)
	host, port, err := net.SplitHostPort(opts.Addr)
	if err != nil {
		slog.Error("split host port", err)
		return
	}
	slog.Debug("memo", "host", host, "port", port)
	env := map[string]string{
		"SERVER_SOFTWARE":   "httpcgi/1.0",
		"SERVER_NAME":       host,
		"GATEWAY_INTERFACE": "CGI/1.1",
		"DOCUMENT_ROOT":     opts.BaseDir,
		"SERVER_PROTOCOL":   opts.Proto,
		"SERVER_PORT":       port,
		"REQUEST_METHOD":    r.Method,
		"PATH_INFO":         "",
		"PATH_TRANSLATED":   "",
		"SCRIPT_NAME":       bn,
		"QUERY_STRING":      r.URL.RawQuery,
		"REMOTE_ADDR":       r.RemoteAddr,
		"CONTENT_TYPE":      r.Header.Get("Content-Type"),
		"CONTENT_LENGTH":    fmt.Sprintf("%d", r.ContentLength),
	}
	user, _, ok := r.BasicAuth()
	if ok {
		env["REMOTE_USER"] = user
		env["AUTH_TYPE"] = "Basic"
	}
	for k, v := range r.Header {
		envname := fmt.Sprintf("HTTP_%s", strings.ReplaceAll(strings.ToUpper(k), "-", "_"))
		env[envname] = strings.Join(v, ";")
	}
	err = RunBy(opts, runner, w, r)
	if err != nil {
		slog.Error("runby", err)
	}
}
