//go:build wazero
// +build wazero

package main

import "testing"

func TestWazero(t *testing.T) {
	testWasmAll(t, &WazeroRunner{})
}
