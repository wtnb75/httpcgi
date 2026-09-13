//go:build docker

package main

// SrvConfig is configuration. set by argument parser
type SrvConfig struct {
	SrvConfigBase
	DockerMounts  []string `long:"docker-volume"`
	DockerWorkDir string   `long:"docker-workdir"`
	DockerMemory  int64    `long:"docker-memory" value-name:"bytes" description:"memory limit for the container (0=unlimited)"`
	DockerCPUs    float64  `long:"docker-cpus" value-name:"cpus" description:"CPU limit in number of CPUs, e.g. 1.5 (0=unlimited)"`
}
