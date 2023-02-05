//go:build docker
// +build docker

package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"golang.org/x/exp/slog"
)

// DockerRunner implements CGI Runner execute by docker
type DockerRunner struct {
}

func (runner DockerRunner) Run(conf SrvConfig, cmdname string, envvar map[string]string,
	stdin io.ReadCloser, stdout io.Writer, stderr io.Writer) error {
	slog.Info("TODO: run", "cmdname", cmdname, "envvar", envvar)
	cl, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		slog.Error("client", err)
		return err
	}
	defer cl.Close()
	env := []string{}
	for k, v := range envvar {
		env = append(env, k+"="+v)
	}
	contConfig := container.Config{
		Image: cmdname,
		Env:   env,
		Tty:   false,
	}
	ctx := context.Background()
	cres, err := cl.ContainerCreate(ctx, &contConfig, nil, nil, nil, "")
	if err != nil {
		slog.Error("containerCreate", err)
		return err
	}
	defer cl.ContainerRemove(ctx, cres.ID, types.ContainerRemoveOptions{})
	if err = cl.ContainerStart(ctx, cres.ID, types.ContainerStartOptions{}); err != nil {
		slog.Error("containerStart", err)
		return err
	}
	stCh, errCh := cl.ContainerWait(ctx, cres.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			slog.Error("execute error", err)
			return err
		}
	case <-stCh:
	}

	out, err := cl.ContainerLogs(ctx, cres.ID, types.ContainerLogsOptions{ShowStdout: true})
	if err != nil {
		slog.Error("logs error", err)
		return err
	}

	stdcopy.StdCopy(stdout, stderr, out)
	return nil
}

func (runner DockerRunner) Exists(conf SrvConfig, path string) (string, string, error) {
	cl, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		slog.Error("docker client", err)
		return "", "", err
	}
	defer cl.Close()
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
			namepart := t[len(conf.BaseDir) : len(t)-len(conf.Suffix)]
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
