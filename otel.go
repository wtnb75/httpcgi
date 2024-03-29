package main

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/contrib/propagators/autoprop"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

func initSetup(exp sdktrace.SpanExporter) {
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL, semconv.ServiceNameKey.String("httpcgi"),
			semconv.ServiceVersionKey.String(version))),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(autoprop.NewTextMapPropagator())
}

func initOtelStdout() (func(), error) {
	exp, err := stdouttrace.New()
	if err != nil {
		slog.Error("stdouttrace", "error", err)
		return nil, err
	}
	initSetup(exp)
	return func() {}, nil
}

func initOtelZipkin() (func(), error) {
	exp, err := zipkin.New("")
	if err != nil {
		return nil, err
	}
	initSetup(exp)
	return func() {
		if err := exp.Shutdown(context.Background()); err != nil {
			slog.Error("shutdown", "error", err)
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
			slog.Error("shutdown", "error", err)
		}
	}, nil
}

func initOtelOtlpHttp() (func(), error) {
	exp, err := otlptracehttp.New(context.Background())
	if err != nil {
		return nil, err
	}
	initSetup(exp)
	return func() {
		if err := exp.Shutdown(context.Background()); err != nil {
			slog.Error("shutdown", "error", err)
		}
	}, nil
}
