//go:build wasmtime

package main

import "testing"

func TestWasmtime(t *testing.T) {
	testWasmAll(t, &WasmtimeRunner{})
}
