//go:build docker
// +build docker

package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/golang/mock/gomock"
	"github.com/wtnb75/httpcgi/mock_client"
	"io"
	"testing"
)

func TestDockerExists(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cli := mock_client.NewMockAPIClient(ctrl)
	images := []types.ImageSummary{
		types.ImageSummary{
			RepoTags: []string{"base/tag123:v1.0.0", "xyz/tag234:latest", "base/path1:latest"},
		},
	}
	runner := DockerRunner{cli: cli}
	conf := SrvConfig{}
	conf.BaseDir = "base/"
	conf.Suffix = ":latest"
	t.Run("pathinfo", func(t *testing.T) {
		cli.EXPECT().ImageList(gomock.Any(), gomock.Any()).Return(images, nil)
		name, path, err := runner.Exists(conf, "path1/info", context.Background())
		if name != "base/path1:latest" {
			t.Error("name", name)
		}
		if path != "/info" {
			t.Error("path", path)
		}
		if err != nil {
			t.Error("err", err)
		}
	})
	t.Run("strict", func(t *testing.T) {
		cli.EXPECT().ImageList(gomock.Any(), gomock.Any()).Return(images, nil)
		name, path, err := runner.Exists(conf, "path1", context.Background())
		if name != "base/path1:latest" {
			t.Error("name", name)
		}
		if path != "" {
			t.Error("path", path)
		}
		if err != nil {
			t.Error("err", err)
		}
	})

	t.Run("not-exists", func(t *testing.T) {
		cli.EXPECT().ImageList(gomock.Any(), gomock.Any()).Return(images, nil)
		name, path, err := runner.Exists(conf, "path2", context.Background())
		if name != "" {
			t.Error("name", name)
		}
		if path != "" {
			t.Error("path", path)
		}
		if err == nil {
			t.Error("err", err)
		}
	})
	return
}

func TestDockerExistsAPIError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cli := mock_client.NewMockAPIClient(ctrl)
	images := []types.ImageSummary{}
	cli.EXPECT().ImageList(gomock.Any(), gomock.Any()).Return(images, fmt.Errorf("error"))
	runner := DockerRunner{cli: cli}
	conf := SrvConfig{}
	path := "path1"
	name, path, err := runner.Exists(conf, "path1/info", context.Background())
	if name != "" {
		t.Error("name", name)
	}
	if path != "" {
		t.Error("path", path)
	}
	if err == nil {
		t.Error("no err", err)
	}
	return
}

func TestDockerRun(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cli := mock_client.NewMockAPIClient(ctrl)
	runner := DockerRunner{cli: cli}
	conf := SrvConfig{}
	conf.DockerMounts = []string{"dir_from1:dir_to2:ro", "dir_from2:dir_to2", "tmp:tmp:tmpfs,rw"}
	envs := map[string]string{"hello": "world"}
	stdin := io.NopCloser(bytes.NewBufferString("hello"))
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cres := container.CreateResponse{ID: "id123"}
	cli.EXPECT().ContainerCreate(gomock.Any(), gomock.Any(), gomock.Any(), nil, nil, "").Return(cres, nil)
	cli.EXPECT().ContainerRemove(gomock.Any(), "id123", gomock.Any()).Return(nil)
	cli.EXPECT().ContainerStart(gomock.Any(), "id123", gomock.Any()).Return(nil)
	ch_exit := make(chan container.WaitResponse, 1)
	ch_err := make(chan error, 1)
	cli.EXPECT().ContainerWait(gomock.Any(), "id123", container.WaitConditionNotRunning).Return(ch_exit, ch_err)
	buf := []byte{}
	output := io.NopCloser(bytes.NewBuffer(buf))
	cli.EXPECT().ContainerLogs(gomock.Any(), "id123", gomock.Any()).Return(output, nil)
	ch_exit <- container.WaitResponse{}
	// ch_err <- fmt.Errorf("hello %s", "world")
	err := runner.Run(conf, "path1", envs, stdin, stdout, stderr, context.Background())
	if err != nil {
		t.Error("err", err)
	}
	return
}
