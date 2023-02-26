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
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"golang.org/x/exp/slog"
)

// DockerRunner implements CGI Runner execute by docker
type DockerRunner struct {
	cli client.APIClient
}

func (runner DockerRunner) Run(conf SrvConfig, cmdname string, envvar map[string]string,
	stdin io.ReadCloser, stdout io.Writer, stderr io.Writer, ctx context.Context) error {
	_, span2 := otel.Tracer("").Start(ctx, "docker-run")
	defer span2.End()
	env := []string{}
	for k, v := range envvar {
		env = append(env, k+"="+v)
	}
	contConfig := container.Config{
		Image:      cmdname,
		Env:        env,
		Tty:        false,
		WorkingDir: conf.DockerWorkDir,
	}
	mounts := []mount.Mount{}
	for _, v := range conf.DockerMounts {
		sp := strings.Split(v, ":")
		rdonly := false
		mountType := mount.TypeBind
		src := ""
		tgt := ""
		if len(sp) >= 2 {
			src = sp[0]
			tgt = sp[1]
		}
		for _, opt := range strings.Split(tgt, ",") {
			switch opt {
			case "ro":
				rdonly = true
			case "rw":
				rdonly = false
			case "volume":
				mountType = mount.TypeVolume
			case "tmpfs":
				mountType = mount.TypeTmpfs
			case "npipe":
				mountType = mount.TypeNamedPipe
			case "cluster":
				mountType = mount.TypeCluster
			}
		}
		slog.Debug("docker-volume", "type", mountType, "src", src, "tgt", tgt, "rdonly", rdonly)
		mounts = append(mounts, mount.Mount{
			Type:     mountType,
			Source:   src,
			Target:   tgt,
			ReadOnly: rdonly,
		})
	}
	hostConfig := container.HostConfig{
		Mounts: mounts,
	}
	cres, err := runner.cli.ContainerCreate(ctx, &contConfig, &hostConfig, nil, nil, "")
	span2.AddEvent("done docker-create")
	if err != nil {
		slog.Error("containerCreate", err)
		return err
	}
	defer runner.cli.ContainerRemove(ctx, cres.ID, types.ContainerRemoveOptions{})
	slog.Debug("docker-start")
	if err = runner.cli.ContainerStart(ctx, cres.ID, types.ContainerStartOptions{}); err != nil {
		slog.Error("containerStart", err)
		return err
	}
	span2.AddEvent("done docker-start")
	slog.Debug("docker-wait")
	stCh, errCh := runner.cli.ContainerWait(ctx, cres.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			slog.Error("execute error", err)
			span2.AddEvent("execute error")
			return err
		}
	case <-stCh:
		slog.Debug("docker-done")
	}
	span2.AddEvent("done docker-wait")

	slog.Debug("docker-logs")
	out, err := runner.cli.ContainerLogs(ctx, cres.ID, types.ContainerLogsOptions{ShowStdout: true})
	span2.AddEvent("done docker-logs")
	if err != nil {
		slog.Error("logs error", err)
		return err
	}

	slog.Debug("docker-stdcopy")
	stdcopy.StdCopy(stdout, stderr, out)
	span2.AddEvent("done docker-stdcopy")
	return nil
}

func (runner DockerRunner) Exists(conf SrvConfig, path string, ctx context.Context) (string, string, error) {
	_, span2 := otel.Tracer("").Start(ctx, "docker-exists")
	defer span2.End()
	imgOpts := types.ImageListOptions{
		All:            false,
		ContainerCount: false,
	}
	imgs, err := runner.cli.ImageList(context.Background(), imgOpts)
	span2.AddEvent("done imagelist")
	if err != nil {
		span2.SetStatus(codes.Error, "image list")
		slog.Error("image list", err)
		return "", "", err
	}
	var name, pathinfo string
	for _, i := range imgs {
		for _, t := range i.RepoTags {
			if !strings.HasPrefix(t, conf.BaseDir) {
				slog.Log(slog.Level(-8), "no prefix", "tag", t, "prefix", conf.BaseDir)
				continue
			}
			if !strings.HasSuffix(t, conf.Suffix) {
				slog.Log(slog.Level(-8), "no suffix", "tag", t, "suffix", conf.Suffix)
				continue
			}
			namepart := t[len(conf.BaseDir) : len(t)-len(conf.Suffix)]
			slog.Debug("namepart", "tag", t, "name", namepart)
			if namepart == path {
				span2.SetStatus(codes.Ok, "found")
				span2.SetAttributes(attribute.String("tag", t))
				return t, "", nil
			} else if strings.HasPrefix(path, namepart+"/") {
				name = t
				pathinfo = path[len(namepart):]
			}
		}
	}
	if len(pathinfo) == 0 {
		span2.SetStatus(codes.Error, "not found")
		return "", "", fmt.Errorf("image not found: %s", path)
	}
	span2.SetAttributes(attribute.String("tag", name), attribute.String("pathinfo", pathinfo))
	return name, pathinfo, nil
}

func init() {
	runnerMap["docker"] = func(SrvConfig) Runner {
		cl, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			slog.Error("docker client", err)
			panic(fmt.Sprintf("docker client error: %s", err))
		}
		return DockerRunner{
			cli: cl,
		}
	}
}
