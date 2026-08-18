package foundation

import (
	"context"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

type otlpTransport string

const (
	otlpTransportGRPC otlpTransport = "grpc"
	otlpTransportHTTP otlpTransport = "http/protobuf"
)

func (s *Service) initTracing() func() {
	otlpEndpoint := getTracingEndpoint()
	if otlpEndpoint == "" {
		s.Logger.Warn("OTEL_EXPORTER_OTLP_ENDPOINT is not set, skipping tracing initialization")
		return func() {}
	}

	otlpProtocol := getTracingTransport(otlpEndpoint)
	if strings.EqualFold(getTracingProtocolEnv(), "http/json") {
		s.Logger.Warn("OTEL_EXPORTER_OTLP_PROTOCOL=http/json is not supported, using OTLP HTTP/protobuf transport")
	}

	s.Logger.Infof("OTEL_EXPORTER_OTLP_PROTOCOL: %s", otlpProtocol)
	s.Logger.Infof("OTEL_EXPORTER_OTLP_ENDPOINT: %s", otlpEndpoint)

	// Create OTLP exporter
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	exporter, err := newTraceExporter(ctx, otlpEndpoint, otlpProtocol)
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
		// ParentBased matters: a bare ratio sampler re-decides on every hop, so
		// a request the gateway chose to sample would be dropped by the very
		// next service and the trace would arrive in pieces. Wrapped like this,
		// the head of the trace decides and everyone downstream honours it.
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(s.Name),
			semconv.ServiceVersionKey.String(Version),
			semconv.DeploymentEnvironmentKey.String(string(FoundationEnv())),
		)),
	)
	otel.SetTracerProvider(tp)

	// W3C Trace Context for the trace itself, plus Baggage so that correlation
	// values set upstream survive the hop.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Return a function to stop the tracer provider.
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			s.Logger.Errorf("failed to shut down tracer provider: %v", err)
		}
	}
}

func newTraceExporter(ctx context.Context, endpoint string, transport otlpTransport) (sdktrace.SpanExporter, error) {
	switch transport {
	case otlpTransportHTTP:
		return newHTTPTraceExporter(ctx, endpoint)
	default:
		return newGRPCTraceExporter(ctx, endpoint)
	}
}

func newGRPCTraceExporter(ctx context.Context, endpoint string) (sdktrace.SpanExporter, error) {
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithTimeout(2 * time.Second),
	}

	switch {
	case hasURLScheme(endpoint):
		opts = append(opts, otlptracegrpc.WithEndpointURL(endpoint))
		if isHTTPURL(endpoint) {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
	default:
		hostPort, _ := splitEndpointPath(endpoint)
		opts = append(opts,
			otlptracegrpc.WithEndpoint(hostPort),
			otlptracegrpc.WithInsecure(),
		)
	}

	return otlptracegrpc.New(ctx, opts...)
}

func newHTTPTraceExporter(ctx context.Context, endpoint string) (sdktrace.SpanExporter, error) {
	opts := []otlptracehttp.Option{
		otlptracehttp.WithTimeout(2 * time.Second),
	}

	switch {
	case hasURLScheme(endpoint):
		opts = append(opts, otlptracehttp.WithEndpointURL(endpoint))
		if isHTTPURL(endpoint) {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
	default:
		hostPort, path := splitEndpointPath(endpoint)
		opts = append(opts,
			otlptracehttp.WithEndpoint(hostPort),
			otlptracehttp.WithInsecure(),
		)
		if path != "" {
			opts = append(opts, otlptracehttp.WithURLPath(path))
		}
	}

	return otlptracehttp.New(ctx, opts...)
}

func getTracingEndpoint() string {
	if endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")); endpoint != "" {
		return endpoint
	}

	return strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
}

func getTracingProtocolEnv() string {
	return strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"))
}

func getTracingTransport(endpoint string) otlpTransport {
	switch strings.ToLower(getTracingProtocolEnv()) {
	case string(otlpTransportGRPC):
		return otlpTransportGRPC
	case string(otlpTransportHTTP), "http/json":
		return otlpTransportHTTP
	default:
		return inferTracingTransport(endpoint)
	}
}

func inferTracingTransport(endpoint string) otlpTransport {
	if strings.Contains(endpoint, "/v1/") {
		return otlpTransportHTTP
	}

	switch endpointPort(endpoint) {
	case "4318":
		return otlpTransportHTTP
	case "4317":
		return otlpTransportGRPC
	default:
		return otlpTransportGRPC
	}
}

func endpointPort(endpoint string) string {
	if hasURLScheme(endpoint) {
		u, err := url.Parse(endpoint)
		if err == nil {
			return u.Port()
		}
	}

	hostPort, _ := splitEndpointPath(endpoint)
	_, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		return ""
	}

	return port
}

func splitEndpointPath(endpoint string) (string, string) {
	idx := strings.Index(endpoint, "/")
	if idx == -1 {
		return endpoint, ""
	}

	return endpoint[:idx], endpoint[idx:]
}

func hasURLScheme(endpoint string) bool {
	return strings.Contains(endpoint, "://")
}

func isHTTPURL(endpoint string) bool {
	return strings.HasPrefix(strings.ToLower(endpoint), "http://")
}

// DefaultTracingRatio is the fraction of traces sampled when
// OTEL_TRACES_SAMPLER_RATIO is unset.
//
// It used to be 0, so configuring an OTLP endpoint produced a service that
// announced "Tracing ratio: 0.000000" and then exported nothing — while ENV.md
// documented the default as 1.0. Someone who goes to the trouble of pointing a
// service at a collector wants its traces.
const DefaultTracingRatio = 1.0

func getTracingRatio() float64 {
	raw := strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER_RATIO"))
	if raw == "" {
		return DefaultTracingRatio
	}

	ratio, err := strconv.ParseFloat(raw, 64)
	if err != nil || ratio < 0 || ratio > 1 {
		return DefaultTracingRatio
	}

	return ratio
}
