//go:build docker
// +build docker

package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"golang.org/x/exp/slog"
)

// DockerRunner implements CGI Runner execute by docker
type DockerRunner struct {
}

func (runner DockerRunner) multiPlex(mplex io.ReadCloser, stdout io.Writer, stderr io.Writer) error {
	for {
		hdr := make([]byte, 8)
		ilen, err := mplex.Read(hdr)
		if err == io.EOF {
			slog.Info("eof", "ilen", ilen)
			return io.EOF
		} else if err != nil {
			slog.Error("read header", err, "ilen", ilen)
			return err
		}
		olen := int(binary.BigEndian.Uint32(hdr[4:8]))
		slog.Info("type=%d, length=%d", int(hdr[0]), olen)
		out := stdout
		if hdr[0] == byte(2) {
			slog.Info("stderr")
			out = stderr
		} else {
			slog.Info("stdout")
		}
		wr, err := io.CopyN(out, mplex, int64(olen))
		if err != nil {
			slog.Error("output", err, "written", wr)
			return err
		}
		slog.Info("output", "wr", wr, "olen", olen)
	}
	return nil
}

func (runner DockerRunner) Run(conf SrvConfig, cmdname string, envvar map[string]string,
	stdin io.ReadCloser, stdout io.Writer, stderr io.Writer) error {
	slog.Info("TODO: run", "cmdname", cmdname, "envvar", envvar)
	return nil
}

func (runner DockerRunner) Exists(conf SrvConfig, path string) (string, string, error) {
	cl, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		slog.Error("docker client", err)
		return "", "", err
	}
	imgOpts := types.ImageListOptions{
		All:            false,
		ContainerCount: false,
	}
	imgs, err := cl.ImageList(context.Background(), imgOpts)
	if err != nil {
		slog.Error("image list", err)
		return "", "", err
	}
	var name, pathinfo string
	for _, i := range imgs {
		for _, t := range i.RepoTags {
			slog.Debug("test", "tag", t)
			if !strings.HasPrefix(t, conf.BaseDir) {
				slog.Debug("no prefix", "tag", t, "prefix", conf.BaseDir)
				continue
			}
			if !strings.HasSuffix(t, conf.Suffix) {
				slog.Debug("no suffix", "tag", t, "suffix", conf.Suffix)
				continue
			}
			namepart := t[len(conf.BaseDir) : len(conf.BaseDir)+len(conf.Suffix)]
			slog.Debug("namepart", "name", namepart)
			if namepart == path {
				return t, "", nil
			} else if strings.HasPrefix(path, namepart) {
				name = t
				pathinfo = path[len(namepart):]
			}
		}
	}
	if len(pathinfo) == 0 {
		return "", "", fmt.Errorf("image not found: %s", path)
	}
	return name, pathinfo, nil
}

func init() {
	runnerMap["docker"] = func(SrvConfig) Runner {
		return DockerRunner{}
	}
}
