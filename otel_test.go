package main

import "testing"

func TestStdout(t *testing.T) {
	t.Parallel()
	fn, err := initOtelStdout()
	if err != nil {
		t.Errorf("error %s", err)
	}
	fn()
}

func TestJaeger(t *testing.T) {
	t.Parallel()
	fn, err := initOtelJaeger()
	if err != nil {
		t.Errorf("error %s", err)
	}
	fn()
}

func TestZipkin(t *testing.T) {
	t.Parallel()
	fn, err := initOtelZipkin()
	if err != nil {
		t.Errorf("error %s", err)
	}
	fn()
}

func TestOtlp(t *testing.T) {
	t.Parallel()
	fn, err := initOtelOtlp()
	if err != nil {
		t.Errorf("error %s", err)
	}
	fn()
}

func TestOtlpHttp(t *testing.T) {
	t.Parallel()
	fn, err := initOtelOtlpHttp()
	if err != nil {
		t.Errorf("error %s", err)
	}
	fn()
}
