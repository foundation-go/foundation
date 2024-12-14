package foundation

import (
	"context"
	"os"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

func (s *Service) initTracing() func() {
	otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if otlpEndpoint == "" {
		s.Logger.Warn("OTEL_EXPORTER_OTLP_ENDPOINT is not set, skipping tracing initialization")
		return func() {}
	}

	s.Logger.Infof("OTEL_EXPORTER_OTLP_ENDPOINT: %s", otlpEndpoint)

	// Create OTLP exporter
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(otlpEndpoint),
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithTimeout(2*time.Second),
	)
	if err != nil {
		s.Logger.Errorf("failed to create OTLP exporter: %v", err)
		return func() {}
	}

	// Create batch span processor
	bsp := sdktrace.NewBatchSpanProcessor(exporter)

	ratio := getTracingRatio()
	s.Logger.Infof("Tracing ratio: %f", ratio)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(bsp),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(ratio)),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(s.Name),
		)),
	)
	otel.SetTracerProvider(tp)

	// Set the global propagator to use W3C Trace Context.
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Return a function to stop the tracer provider.
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			s.Logger.Errorf("failed to shut down tracer provider: %v", err)
		}
	}
}

func getTracingRatio() float64 {
	ratio := os.Getenv("OTEL_TRACES_SAMPLER_RATIO")
	if ratio == "" {
		ratio = "0"
	}

	ratioFloat, err := strconv.ParseFloat(ratio, 64)
	if err != nil {
		return 0
	}

	return ratioFloat
}
