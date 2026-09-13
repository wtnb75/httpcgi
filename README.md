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
    - with wazero runtime: go install -tags wazero github.com/wtnb75/httpcgi@latest
- supports Docker
    - go install -tags docker github.com/wtnb75/httpcgi@latest

## run

- httpcgi -l :8080

options

```
Usage:
  httpcgi [OPTIONS]

Application Options:
  -v, --verbose                                      log verbose
  -q, --quiet                                        log quiet
  -l, --listen=[host]:port
      --protocol=tcp/unix
  -p, --prefix=url-prefix
  -b, --base-dir=dirname
  -s, --suffix=.ext
      --json-log
      --runner=name
  -V, --version
      --opentelemetry=[stdout|otlp|otlp-http]
  -t, --timeout=
      --rlimit-cpu=seconds
      --rlimit-mem=bytes
  -e, --env=KEY=VALUE
      --env-file=path

Help Options:
  -h, --help                                         Show this help message
```

## custom environment variables

- `-e KEY=VALUE` / `--env=KEY=VALUE` sets an extra environment variable for the CGI process. Repeatable.
- `--env-file=path` loads extra environment variables from a `.env`-style file (comments, blank lines, quoted values, `export` prefix, etc. — parsed with [joho/godotenv](https://github.com/joho/godotenv)). Repeatable; applied before `--env`.
- refuses to start the request (500 error) if a key collides with a standard CGI variable (`REQUEST_METHOD`, `SCRIPT_NAME`, `HTTP_*`, etc.) already set by httpcgi, or with a key from an earlier `--env-file`/`--env`.

## docker

- docker run ghcr.io/wtnb75/httpcgi [options]...

## docker compose

- [example configuration](./examples/docker-compose.yml)

## resource limits

- `os` runner (Linux only): `--rlimit-cpu=seconds` / `--rlimit-mem=bytes` apply `setrlimit`/`prlimit` (`RLIMIT_CPU` / `RLIMIT_AS`) to the CGI process right after it starts. Ignored (with a warning) on non-Linux platforms.
- `docker` runner: `--docker-memory=bytes` / `--docker-cpus=cpus` map to the container's native `Memory` / `NanoCPUs` resource limits.
- wasm runners (`wasmtime`/`wazero`/`wasmer`) are not covered yet; tracked separately in #147.

## known limitations

- WASI stdin (request body / POST data)
    - supported by the `wasmtime` and `wazero` runners
    - **not supported** by the `wasmer` runner: the `wasmer-go` binding currently in use (v1.0.4) has no API to feed an `io.Reader` as WASI stdin (only `InheritStdin()`, which inherits the host process's own stdin). The request body is silently discarded for this runner. (#9)
- orphaned child processes
    - the `os` runner reaps the CGI process it starts directly, but it does **not** reap grandchild processes that a CGI script spawns and backgrounds (e.g. `some-daemon &`)
    - if httpcgi runs as PID 1 (common in a minimal container), such orphans are reparented to it and can accumulate as zombies since nothing ever calls `wait()` for them
    - recommendation: run httpcgi under an init process that reaps orphans (e.g. `docker run --init`, [tini](https://github.com/krallin/tini), or [dumb-init](https://github.com/Yelp/dumb-init)), or avoid backgrounding processes from CGI scripts (#84)
