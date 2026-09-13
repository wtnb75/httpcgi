package main

import "time"

type SrvConfigBase struct {
	Verbose      bool          `short:"v" long:"verbose" description:"log verbose"`
	Quiet        bool          `short:"q" long:"quiet" description:"log quiet"`
	Addr         string        `short:"l" long:"listen" default:"localhost:" value-name:"[host]:port"`
	Proto        string        `long:"protocol" default:"tcp" value-name:"tcp/unix"`
	Prefix       string        `short:"p" long:"prefix" default:"/" value-name:"url-prefix"`
	BaseDir      string        `short:"b" long:"base-dir" default:"." value-name:"dirname"`
	Suffix       string        `short:"s" long:"suffix" value-name:".ext"`
	JSONLog      bool          `long:"json-log"`
	Runner       string        `long:"runner" default:"os" value-name:"name"`
	Version      bool          `short:"V" long:"version"`
	OtelProvider string        `long:"opentelemetry" choice:"stdout" choice:"otlp" choice:"otlp-http"`
	Timeout      time.Duration `short:"t" long:"timeout" default:"1m"`
	RlimitCPU    int64         `long:"rlimit-cpu" value-name:"seconds" description:"os runner: CPU time limit for the CGI process (0=unlimited, Linux only)"`
	RlimitMem    int64         `long:"rlimit-mem" value-name:"bytes" description:"os runner: address space limit for the CGI process (0=unlimited, Linux only)"`
	Env          []string      `short:"e" long:"env" value-name:"KEY=VALUE" description:"extra environment variable for the CGI process (repeatable, cannot override standard CGI variables)"`
	EnvFile      []string      `long:"env-file" value-name:"path" description:"load extra environment variables for the CGI process from a .env file (repeatable, applied before --env)"`
}
