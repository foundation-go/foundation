package foundation

import "testing"

func TestGetTracingEndpointPrefersTracesEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", " http://localhost:4318/v1/traces ")

	if got := getTracingEndpoint(); got != "http://localhost:4318/v1/traces" {
		t.Fatalf("expected traces endpoint to win, got %q", got)
	}
}

func TestGetTracingEndpointFallsBackToGenericEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", " http://localhost:4317 ")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")

	if got := getTracingEndpoint(); got != "http://localhost:4317" {
		t.Fatalf("expected generic endpoint, got %q", got)
	}
}

func TestGetTracingTransportPrefersExplicitProtocol(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http/protobuf")

	if got := getTracingTransport("http://localhost:4317"); got != otlpTransportHTTP {
		t.Fatalf("expected http transport, got %q", got)
	}
}

func TestGetTracingTransportInfersHTTPFromPort4318(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "")

	if got := getTracingTransport("http://localhost:4318"); got != otlpTransportHTTP {
		t.Fatalf("expected http transport, got %q", got)
	}
}

func TestGetTracingTransportInfersHTTPFromTracesPath(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "")

	if got := getTracingTransport("http://localhost:9999/v1/traces"); got != otlpTransportHTTP {
		t.Fatalf("expected http transport, got %q", got)
	}
}

func TestGetTracingTransportDefaultsToGRPC(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "")

	if got := getTracingTransport("http://localhost:4317"); got != otlpTransportGRPC {
		t.Fatalf("expected grpc transport, got %q", got)
	}
}

// The sampler defaulted to 0, so configuring an OTLP endpoint produced a
// service that announced "Tracing ratio: 0.000000" and exported nothing — while
// ENV.md documented the default as 1.0.
func TestGetTracingRatioDefaultsToSamplingEverything(t *testing.T) {
	t.Setenv("OTEL_TRACES_SAMPLER_RATIO", "")

	if got := getTracingRatio(); got != DefaultTracingRatio {
		t.Fatalf("expected %v, got %v", DefaultTracingRatio, got)
	}
}

func TestGetTracingRatio(t *testing.T) {
	tests := []struct {
		raw  string
		want float64
	}{
		{raw: "0", want: 0},
		{raw: "0.1", want: 0.1},
		{raw: "1", want: 1},
		{raw: " 0.25 ", want: 0.25},
		// Nonsense and out-of-range values fall back rather than silently
		// disabling tracing.
		{raw: "not-a-number", want: DefaultTracingRatio},
		{raw: "-1", want: DefaultTracingRatio},
		{raw: "2", want: DefaultTracingRatio},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			t.Setenv("OTEL_TRACES_SAMPLER_RATIO", tt.raw)

			if got := getTracingRatio(); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

// Tracing must be a no-op, not a failure, when no collector is configured.
func TestInitTracingWithoutAnEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")

	svc := newTestService()

	shutdown := svc.initTracing()
	if shutdown == nil {
		t.Fatal("initTracing must always return a shutdown function")
	}

	shutdown()
}
