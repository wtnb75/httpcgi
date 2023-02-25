//go:build docker
// +build docker

package main

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/golang/mock/gomock"
	"github.com/wtnb75/httpcgi/mock_client"
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
