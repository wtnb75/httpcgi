//go:build docker

package main

// SrvConfig is configuration. set by argument parser
type SrvConfig struct {
	SrvConfigBase
	DockerMounts  []string `long:"docker-volume"`
	DockerWorkDir string   `long:"docker-workdir"`
}
