package main

type SrvConfigBase struct {
	Verbose      bool   `short:"v" long:"verbose" description:"log verbose"`
	Quiet        bool   `short:"q" long:"quiet" description:"log quiet"`
	Addr         string `short:"l" long:"listen" default:"localhost:" value-name:"[host]:port"`
	Proto        string `long:"protocol" default:"tcp" value-name:"tcp/unix"`
	Prefix       string `short:"p" long:"prefix" default:"/" value-name:"url-prefix"`
	BaseDir      string `short:"b" long:"base-dir" default:"." value-name:"dirname"`
	Suffix       string `short:"s" long:"suffix" value-name:".ext"`
	JSONLog      bool   `long:"json-log"`
	Runner       string `long:"runner" default:"os" value-name:"name"`
	Version      bool   `short:"V" long:"version"`
	OtelProvider string `long:"opentelemetry" choice:"stdout" choice:"zipkin" choice:"otlp" choice:"otlp-http"`
}
