# httpcgi is...

- HTTP Server
- supports legacy CGI program
- can run WASI binary with CGI interface
- only CGI
  - without static file serving
  - without reverse proxy
  - ...

## install

- go install github.com/wtnb75/httpcgi@latest
- supports WASI
    - with wasmer runtime: go install -tags wasmer github.com/wtnb75/httpcgi@latest
    - with wasmtime runtime: go install -tags wasmtime github.com/wtnb75/httpcgi@latest
- supports Docker
    - go install -tags docker github.com/wtnb75/httpcgi@latest

## run

- httpcgi -l :8080

options

```
Usage:
  httpcgi [OPTIONS]

Application Options:
  -v, --verbose               log verbose
  -q, --quiet                 log quiet
  -l, --listen=[host]:port
      --protocol=tcp/unix
  -p, --prefix=url-prefix
  -b, --base-dir=dirname
  -s, --suffix=.ext
      --json-log
      --runner=name

Help Options:
  -h, --help                  Show this help message
```

## docker

- docker run ghcr.io/wtnb75/httpcgi [options]...

## docker compose

- [example configuration](./examples/docker-compose.yml)
