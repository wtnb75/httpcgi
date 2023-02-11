package main

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/exporters/zipkin"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"golang.org/x/exp/slog"
)

func initSetup(exp sdktrace.SpanExporter) {
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp))
	otel.SetTracerProvider(tp)
}

func initOtelStdout() (func(), error) {
	exp, err := stdouttrace.New()
	if err != nil {
		slog.Error("stdouttrace", err)
		return nil, err
	}
	initSetup(exp)
	return func() {}, nil
}

func initOtelJaeger() (func(), error) {
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint())
	if err != nil {
		return nil, err
	}
	initSetup(exp)
	return func() {
		if err := exp.Shutdown(context.Background()); err != nil {
			slog.Error("shutdown", err)
		}
	}, nil
}

func initOtelZipkin() (func(), error) {
	exp, err := zipkin.New("")
	if err != nil {
		return nil, err
	}
	initSetup(exp)
	return func() {
		if err := exp.Shutdown(context.Background()); err != nil {
			slog.Error("shutdown", err)
		}
	}, nil
}

func initOtelOtlp() (func(), error) {
	exp, err := otlptracegrpc.New(context.Background())
	if err != nil {
		return nil, err
	}
	initSetup(exp)
	return func() {
		if err := exp.Shutdown(context.Background()); err != nil {
			slog.Error("shutdown", err)
		}
	}, nil
}
