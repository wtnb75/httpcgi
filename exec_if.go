package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
)

// Runner is interface to run CGI
type Runner interface {
	Run(conf SrvConfig, cmdname string, envvar map[string]string,
		stdin io.ReadCloser, stdout io.Writer, stderr io.Writer,
		ctx context.Context) error
	Exists(conf SrvConfig, path string, ctx context.Context) (string, string, error)
}

// OutputFilter converts CGI output to http.ResponseWriter
func OutputFilter(stdout io.Reader, w http.ResponseWriter, wg *sync.WaitGroup) error {
	if wg != nil {
		defer wg.Done()
	}
	rd := bufio.NewReader(stdout)
	statusCode := http.StatusOK
	for {
		line, _, err := rd.ReadLine()
		if err != nil {
			slog.Error("read header error:", "error", err)
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
				slog.Info("status code update", "n", n, "status", statusCode)
			}
		} else {
			slog.Debug("add-header", "key", k, "val", v)
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(statusCode)
	olen, err := io.Copy(w, rd)
	if err != nil {
		slog.Error("write body error", "error", err)
		return err
	}
	slog.Debug("write body", "length", olen)
	return nil
}

func splitPathInfo(basedir string, path string, suffix string) (string, string, error) {
	ret := path
	for ret != "" && ret != "." && ret != "/" {
		slog.Debug("check", "path", path, "basedir", basedir, "cur", ret)
		if strings.HasSuffix(ret, suffix) {
			if fi, err := os.Stat(filepath.Join(basedir, ret)); err == nil {
				if fi.Mode().IsRegular() {
					return ret, path[len(ret):], nil
				}
			}
		}
		ret = filepath.Dir(ret)
	}
	slog.Warn("not found", "base", basedir, "path", path)
	return "", "", fmt.Errorf("not found %s", path)
}

// RunBy executes HTTP request
func RunBy(opts SrvConfig, runner Runner, w http.ResponseWriter, r *http.Request) error {
	ctx, span := otel.Tracer("").Start(r.Context(), "run")
	defer span.End()
	startTime := time.Now()
	httpStatus := http.StatusOK
	defer func() {
		slog.Info(
			"access-log", "method", r.Method, "url", r.URL,
			"remote-addr", r.RemoteAddr,
			"proto", r.Proto,
			"user-agent", r.UserAgent(),
			"status", httpStatus,
			"elapsed", time.Since(startTime),
		)
	}()
	bn := strings.TrimPrefix(r.URL.Path, opts.Prefix)
	host, port, err := net.SplitHostPort(opts.Addr)
	if err != nil {
		span.SetStatus(codes.Error, "split hostport")
		slog.Error("split host port", "error", err)
		return err
	}
	slog.Debug("memo", "host", host, "port", port)
	_, span1 := otel.Tracer("").Start(ctx, "exists")
	span1.SetAttributes(attribute.String("path", bn))
	bn2, rest, err := runner.Exists(opts, bn, ctx)
	span1.SetAttributes(attribute.String("script", bn2), attribute.String("pathinfo", rest))
	span1.End()
	if err != nil {
		slog.Error("not found", "error", err, "basename", bn)
		span.SetStatus(codes.Error, "not found")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintln(w, bn, "not found")
		return err
	}
	slog.Debug("memo(path)", "bn", bn, "bn2", bn2, "rest", rest)
	env := map[string]string{
		"SERVER_SOFTWARE":   "httpcgi/" + version,
		"SERVER_NAME":       host,
		"GATEWAY_INTERFACE": "CGI/1.1",
		"DOCUMENT_ROOT":     opts.BaseDir,
		"SERVER_PROTOCOL":   opts.Proto,
		"SERVER_PORT":       port,
		"REQUEST_METHOD":    r.Method,
		"PATH_INFO":         rest,
		"PATH_TRANSLATED":   filepath.Join(opts.BaseDir, rest),
		"SCRIPT_NAME":       bn2,
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
	pr, pw := io.Pipe()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		if err := OutputFilter(pr, w, &wg); err != nil {
			slog.Error("output filter", "error", err)
		}
		span.AddEvent("ofilter finished")
	}()
	_, span2 := otel.Tracer("").Start(ctx, "run")
	span2.SetAttributes(attribute.String("script", bn2))
	if span2.IsRecording() {
		carrier := propagation.MapCarrier{}
		otel.GetTextMapPropagator().Inject(ctx, carrier)
		for k, v := range carrier {
			escaped := strings.Replace(strings.ToUpper(k), "-", "_", -1)
			env[fmt.Sprintf("HTTP_%s", escaped)] = v
		}
	}
	err = runner.Run(opts, bn2, env, r.Body, pw, log.Writer(), ctx)
	span2.End()
	if err != nil {
		span.SetStatus(codes.Error, "exec error")
		httpStatus = http.StatusInternalServerError
		w.WriteHeader(httpStatus)
		fmt.Fprintf(w, "command error: %s", err)
	}
	pw.Close()
	wg.Wait()
	pr.Close()
	return nil
}

// DoPipe calls io.Copy() and wg.Done()
func DoPipe(input io.Reader, output io.Writer, wg *sync.WaitGroup) error {
	if wg != nil {
		defer wg.Done()
	}
	ilen, err := io.Copy(output, input)
	if err != nil {
		slog.Error("pipe error:", "error", err)
		return err
	}
	slog.Debug("pipe", "length", ilen)
	return nil
}
