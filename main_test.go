package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestResolveBaseDirEmptyUsesGetwd(t *testing.T) {
	t.Parallel()
	getwd := func() (string, error) { return "/tmp/somewhere", nil }
	dir, err := resolveBaseDir("os", "", getwd)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if dir != "/tmp/somewhere" {
		t.Errorf("dir = %q, want /tmp/somewhere", dir)
	}
}

func TestResolveBaseDirGetwdError(t *testing.T) {
	t.Parallel()
	getwd := func() (string, error) { return "", fmt.Errorf("getwd boom") }
	_, err := resolveBaseDir("os", "", getwd)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if strings.Contains(err.Error(), "abs") {
		t.Errorf("error mislabeled as abs error: %s", err)
	}
	if !strings.Contains(err.Error(), "getwd boom") {
		t.Errorf("error does not wrap underlying cause: %s", err)
	}
}

func TestResolveBaseDirDockerSkipsAbs(t *testing.T) {
	t.Parallel()
	getwd := func() (string, error) { return "", fmt.Errorf("should not be called") }
	dir, err := resolveBaseDir("docker", "relative/path", getwd)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if dir != "relative/path" {
		t.Errorf("dir = %q, want relative/path unchanged", dir)
	}
}
