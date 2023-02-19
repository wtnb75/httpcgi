//go:build wasmer
// +build wasmer

package main

import "testing"

func TestWasmer(t *testing.T) {
	testWasmAll(t, &WasmerRunner{})
}
