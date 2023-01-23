package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jessevdk/go-flags"
	"golang.org/x/exp/slog"
)

var opts struct {
	Verbose     bool   `short:"v" long:"verbose"`
	Addr        string `short:"l" long:"listen" default:"localhost:"`
	Proto       string `long:"protocol" default:"tcp"`
	Prefix      string `short:"p" long:"prefix" default:"/"`
	BaseDir     string `short:"b" long:"base-dir"`
	Suffix      string `short:"s" long:"suffix"`
	JsonLog     bool   `long:"json-log"`
}

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

func getPipe(cmd *exec.Cmd) (stdin io.WriteCloser, stdout io.ReadCloser, stderr io.ReadCloser, err error) {
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

func processInput(stdin io.Writer, w http.ResponseWriter, r *http.Request, wg *sync.WaitGroup) error {
	defer wg.Done()
	ilen, err := io.Copy(stdin, r.Body)
	if err != nil {
		slog.Error("read body error:", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "command error: %s", err)
		return err
	}
	if ilen != 0 {
		slog.Info("read body", "length", ilen)
	}
	return nil
}

func processOutput(stdout io.Reader, w http.ResponseWriter, r *http.Request, wg *sync.WaitGroup) error {
	defer wg.Done()
	rd := bufio.NewReader(stdout)
	statusCode := http.StatusOK
	for {
		line, _, err := rd.ReadLine()
		if err != nil {
			slog.Error("read header error:", err)
			return err
		}
		if len(line) == 0 {
			slog.Info("header finished")
			break
		}
		linestr := string(line)
		idx := strings.Index(linestr, ":")
		if idx == -1 {
			slog.Warn("header format error", "line", linestr)
			return fmt.Errorf("invalid header format")
		}
		k := strings.TrimSpace(linestr[:idx])
		v := strings.TrimSpace(linestr[idx+1:])
		if strings.ToLower(k) == "status" {
			if n, err := fmt.Sscan(v, &statusCode); err != nil {
				slog.Warn("status code error", "line", linestr)
			} else {
				slog.Info("new status", "n", n, "status", statusCode)
			}
		} else {
			slog.Info("add-header", "key", k, "val", v)
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(statusCode)
	olen, err := io.Copy(w, rd)
	if err != nil {
		slog.Error("write body error", err)
		return err
	}
	slog.Info("write body", "length", olen)
	return nil
}

func processError(stderr io.Reader, w http.ResponseWriter, r *http.Request, wg *sync.WaitGroup) error {
	defer wg.Done()
	elen, err := io.Copy(log.Writer(), stderr)
	if err != nil {
		slog.Error("write error error", err)
		return err
	}
	if elen != 0 {
		slog.Info("write err", "length", elen)
	}
	return nil
}

func defaultRoute(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	http_status := http.StatusOK
	defer func() {
		slog.Info(
			"access-log", "method", r.Method, "url", r.URL,
			"remote-addr", r.RemoteAddr,
			"proto", r.Proto,
			"user-agent", r.UserAgent(),
			"status", http_status,
			"elapsed", time.Since(startTime),
		)
	}()
	bn := strings.TrimPrefix(r.URL.Path, opts.Prefix)
	fn := filepath.Join(opts.BaseDir, bn)
	slog.Debug("path", "full-path", fn)
	fi, err := os.Lstat(fn)
	if err != nil {
		slog.Error("lstat error", err)
		http_status = http.StatusNotFound
		w.WriteHeader(http_status)
		fmt.Fprintf(w, "lstat failed: %s", err)
		return
	}
	slog.Debug("stat", "fi", fi)
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
	cmd := exec.Command(fn)
	slog.Info("pid", "process", cmd.Process)
	stdin, stdout, stderr, err := getPipe(cmd)
	if err != nil {
		slog.Error("pipe error", err)
		http_status = http.StatusInternalServerError
		w.WriteHeader(http_status)
		fmt.Fprintf(w, "pipe error: %s", err)
		return
	}
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	slog.Info("starting command", "cmd", cmd)
	if err := cmd.Start(); err != nil {
		slog.Error("command start error", err)
		http_status = http.StatusInternalServerError
		w.WriteHeader(http_status)
		fmt.Fprintf(w, "command error: %s", err)
		return
	}
	slog.Info("pid", "process", cmd.Process)
	var wg sync.WaitGroup
	wg.Add(1)
	go processInput(stdin, w, r, &wg)
	wg.Add(1)
	go processError(stderr, w, r, &wg)
	wg.Add(1)
	go processOutput(stdout, w, r, &wg)
	wg.Wait()
	stdin.Close()
	stderr.Close()
}
