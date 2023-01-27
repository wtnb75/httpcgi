#! /bin/sh

rustc hello.rs
rustc --target wasm32-wasi hello.rs
