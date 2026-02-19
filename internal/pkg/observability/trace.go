package observability

import (
	"context"
	"log"
	"service-platform/internal/config"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

var globalTracerProvider *trace.TracerProvider

// InitTracer initializes the OpenTelemetry tracer provider
// Returns a cleanup function that must be deferred to properly shutdown the tracer
// Returns nil if Tempo is disabled
func InitTracer(ctx context.Context) (func(context.Context) error, error) {
	cfg := config.ServicePlatform.Get()

	// Check if Tempo is enabled
	if !cfg.Observability.Tempo.Enabled {
		log.Println("⚠️ Tempo tracing is disabled in configuration")
		return func(context.Context) error { return nil }, nil
	}

	// Create OTLP gRPC exporter
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.Observability.Tempo.OTLPGRPCEndpoint),
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithTimeout(time.Duration(cfg.Observability.Tempo.ExportTimeoutMs)*time.Millisecond),
	)
	if err != nil {
		log.Printf("⚠️ Failed to create OTLP exporter: %v (tracing disabled)", err)
		return func(context.Context) error { return nil }, err
	}

	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.Monitoring.ServiceName),
			semconv.ServiceVersion(cfg.App.VersionCode),
			semconv.DeploymentEnvironment(getEnvironmentFromConfig()),
		),
	)
	if err != nil {
		log.Printf("⚠️ Failed to create resource: %v", err)
		return func(context.Context) error { return nil }, err
	}

	// Create tracer provider
	batchProcessor := trace.NewBatchSpanProcessor(exporter)
	globalTracerProvider = trace.NewTracerProvider(
		trace.WithResource(res),
		trace.WithSpanProcessor(batchProcessor),
		trace.WithSampler(trace.TraceIDRatioBased(cfg.Observability.Tempo.SampleRate)),
	)

	// Set the global tracer provider
	otel.SetTracerProvider(globalTracerProvider)

	log.Printf("✅ Tracer initialized successfully with Tempo: %s", cfg.Observability.Tempo.OTLPGRPCEndpoint)

	// Return shutdown function
	return func(shutdownCtx context.Context) error {
		return globalTracerProvider.Shutdown(shutdownCtx)
	}, nil
}

// GetTracerProvider returns the global tracer provider
func GetTracerProvider() *trace.TracerProvider {
	if globalTracerProvider == nil {
		return trace.NewTracerProvider()
	}
	return globalTracerProvider
}

// ShutdownTracer gracefully shuts down the tracer provider
func ShutdownTracer(ctx context.Context) error {
	if globalTracerProvider == nil {
		return nil
	}

	timeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return globalTracerProvider.Shutdown(timeout)
}

// getEnvironmentFromConfig returns the current environment
func getEnvironmentFromConfig() string {
	env := config.ServicePlatform.Get().App.LogLevel
	if env == "DEBUG" || env == "debug" {
		return "development"
	}
	return "production"
}
