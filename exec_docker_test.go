//go:build docker

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/golang/mock/gomock"
	"github.com/wtnb75/httpcgi/mock_client"
)

func TestDockerExists(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cli := mock_client.NewMockAPIClient(ctrl)
	images := []image.Summary{
		image.Summary{
			RepoTags: []string{"base/tag123:v1.0.0", "xyz/tag234:latest", "base/path1:latest"},
		},
	}
	runner := DockerRunner{cli: cli}
	conf := SrvConfig{}
	conf.Timeout = time.Duration(1000_000_000)
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

func TestDockerExistsOverlappingPrefixSuffix(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cli := mock_client.NewMockAPIClient(ctrl)
	images := []image.Summary{
		image.Summary{RepoTags: []string{"aab"}},
	}
	cli.EXPECT().ImageList(gomock.Any(), gomock.Any()).Return(images, nil)
	runner := DockerRunner{cli: cli}
	conf := SrvConfig{}
	conf.BaseDir = "aa"
	conf.Suffix = "ab"
	name, path, err := runner.Exists(conf, "x", context.Background())
	if err == nil {
		t.Errorf("expected error, got name=%s path=%s", name, path)
	}
}

type ctxKeyType string

const ctxTestKey ctxKeyType = "test-key"

type ctxHasValueMatcher struct {
	key ctxKeyType
	val any
}

func (m ctxHasValueMatcher) Matches(x any) bool {
	c, ok := x.(context.Context)
	if !ok {
		return false
	}
	return c.Value(m.key) == m.val
}

func (m ctxHasValueMatcher) String() string {
	return fmt.Sprintf("context with value %s=%v", m.key, m.val)
}

func TestDockerExistsUsesRequestContext(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cli := mock_client.NewMockAPIClient(ctrl)
	images := []image.Summary{}
	ctx := context.WithValue(context.Background(), ctxTestKey, "hello")
	cli.EXPECT().ImageList(ctxHasValueMatcher{key: ctxTestKey, val: "hello"}, gomock.Any()).Return(images, nil)
	runner := DockerRunner{cli: cli}
	conf := SrvConfig{}
	_, _, _ = runner.Exists(conf, "path1", ctx)
}

func TestDockerExistsAPIError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cli := mock_client.NewMockAPIClient(ctrl)
	images := []image.Summary{}
	cli.EXPECT().ImageList(gomock.Any(), gomock.Any()).Return(images, fmt.Errorf("error"))
	runner := DockerRunner{cli: cli}
	conf := SrvConfig{}
	conf.Timeout = time.Duration(1000_000_000)
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

func TestDockerRunStdCopyError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cli := mock_client.NewMockAPIClient(ctrl)
	runner := DockerRunner{cli: cli}
	conf := SrvConfig{}
	conf.Timeout = time.Duration(1000_000_000)
	envs := map[string]string{}
	stdin := io.NopCloser(bytes.NewBufferString(""))
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cres := container.CreateResponse{ID: "id123"}
	cli.EXPECT().ContainerCreate(gomock.Any(), gomock.Any(), gomock.Any(), nil, nil, "").Return(cres, nil)
	cli.EXPECT().ContainerRemove(gomock.Any(), "id123", gomock.Any()).Return(nil)
	cli.EXPECT().ContainerStart(gomock.Any(), "id123", gomock.Any()).Return(nil)
	ch_exit := make(chan container.WaitResponse, 1)
	ch_err := make(chan error, 1)
	cli.EXPECT().ContainerWait(gomock.Any(), "id123", container.WaitConditionNotRunning).Return(ch_exit, ch_err)
	// malformed multiplexed stream: unrecognized stdcopy stream-type byte (valid values are 0-3)
	badFrame := []byte{99, 0, 0, 0, 0, 0, 0, 0}
	output := io.NopCloser(bytes.NewBuffer(badFrame))
	cli.EXPECT().ContainerLogs(gomock.Any(), "id123", gomock.Any()).Return(output, nil)
	ch_exit <- container.WaitResponse{}
	err := runner.Run(conf, "path1", envs, stdin, stdout, stderr, context.Background())
	if err == nil {
		t.Error("expected error from malformed docker log stream, got nil")
	}
}

func TestDockerRunTimeout(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cli := mock_client.NewMockAPIClient(ctrl)
	runner := DockerRunner{cli: cli}
	conf := SrvConfig{}
	conf.Timeout = time.Duration(10 * time.Millisecond)
	envs := map[string]string{}
	stdin := io.NopCloser(bytes.NewBufferString(""))
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cres := container.CreateResponse{ID: "id123"}
	cli.EXPECT().ContainerCreate(gomock.Any(), gomock.Any(), gomock.Any(), nil, nil, "").Return(cres, nil)
	cli.EXPECT().ContainerRemove(gomock.Any(), "id123", gomock.Any()).Return(nil)
	cli.EXPECT().ContainerStart(gomock.Any(), "id123", gomock.Any()).Return(nil)
	// never signaled: simulates a hung container
	ch_exit := make(chan container.WaitResponse)
	ch_err := make(chan error)
	cli.EXPECT().ContainerWait(gomock.Any(), "id123", container.WaitConditionNotRunning).Return(ch_exit, ch_err)
	cli.EXPECT().ContainerKill(gomock.Any(), "id123", gomock.Any()).Return(nil)
	err := runner.Run(conf, "path1", envs, stdin, stdout, stderr, context.Background())
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

type hostConfigNoMountsMatcher struct{}

func (hostConfigNoMountsMatcher) Matches(x any) bool {
	hc, ok := x.(*container.HostConfig)
	if !ok {
		return false
	}
	return len(hc.Mounts) == 0
}

func (hostConfigNoMountsMatcher) String() string {
	return "host config with no mounts"
}

func TestDockerRunInvalidVolumeSpecIgnored(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cli := mock_client.NewMockAPIClient(ctrl)
	runner := DockerRunner{cli: cli}
	conf := SrvConfig{}
	conf.Timeout = time.Duration(1000_000_000)
	conf.DockerMounts = []string{"onlyonepart"}
	envs := map[string]string{}
	stdin := io.NopCloser(bytes.NewBufferString(""))
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cres := container.CreateResponse{ID: "id123"}
	cli.EXPECT().ContainerCreate(gomock.Any(), gomock.Any(), hostConfigNoMountsMatcher{}, nil, nil, "").Return(cres, nil)
	cli.EXPECT().ContainerRemove(gomock.Any(), "id123", gomock.Any()).Return(nil)
	cli.EXPECT().ContainerStart(gomock.Any(), "id123", gomock.Any()).Return(nil)
	ch_exit := make(chan container.WaitResponse, 1)
	ch_err := make(chan error, 1)
	cli.EXPECT().ContainerWait(gomock.Any(), "id123", container.WaitConditionNotRunning).Return(ch_exit, ch_err)
	output := io.NopCloser(bytes.NewBuffer(nil))
	cli.EXPECT().ContainerLogs(gomock.Any(), "id123", gomock.Any()).Return(output, nil)
	ch_exit <- container.WaitResponse{}
	err := runner.Run(conf, "path1", envs, stdin, stdout, stderr, context.Background())
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
}

func TestDockerRun(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cli := mock_client.NewMockAPIClient(ctrl)
	runner := DockerRunner{cli: cli}
	conf := SrvConfig{}
	conf.Timeout = time.Duration(1000_000_000)
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
