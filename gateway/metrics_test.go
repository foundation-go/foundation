package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	gwruntime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fmetrics "github.com/foundation-go/foundation/metrics"
)

// Labelling by request path would let any client mint a new time series per
// URL. Requests that never matched a route share a single series.
func TestWithMetricsLabelsUnmatchedRequests(t *testing.T) {
	before := testutil.ToFloat64(fmetrics.HTTPRequests.WithLabelValues(http.MethodGet, unmatchedRoute, "404"))

	handler := WithMetrics(http.NotFoundHandler())
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/chats/1", nil))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/chats/2", nil))

	after := testutil.ToFloat64(fmetrics.HTTPRequests.WithLabelValues(http.MethodGet, unmatchedRoute, "404"))

	assert.Equal(t, 2.0, after-before, "both requests belong to the same series")
}

func TestWithMetricsRecordsTheStatus(t *testing.T) {
	before := testutil.ToFloat64(fmetrics.HTTPRequests.WithLabelValues(http.MethodPost, unmatchedRoute, "418"))

	handler := WithMetrics(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/brew", nil))

	after := testutil.ToFloat64(fmetrics.HTTPRequests.WithLabelValues(http.MethodPost, unmatchedRoute, "418"))

	assert.Equal(t, 1.0, after-before)
}

// A handler that never calls WriteHeader still produced a 200.
func TestWithMetricsDefaultsToTwoHundred(t *testing.T) {
	before := testutil.ToFloat64(fmetrics.HTTPRequests.WithLabelValues(http.MethodGet, unmatchedRoute, "200"))

	handler := WithMetrics(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	after := testutil.ToFloat64(fmetrics.HTTPRequests.WithLabelValues(http.MethodGet, unmatchedRoute, "200"))

	assert.Equal(t, 1.0, after-before)
}

// The route pattern is only known after grpc-gateway has routed the request,
// which happens inside the mux — below the middleware. The annotator carries it
// back out.
func TestRouteAnnotatorReportsTheMatchedRoute(t *testing.T) {
	const route = "/v1/chats/{chat_id}"

	mux := gwruntime.NewServeMux(gwruntime.WithMetadata(RouteAnnotator))

	// Stand in for a generated handler: annotate with the route pattern the way
	// grpc-gateway's generated code does.
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := gwruntime.AnnotateContext(
			r.Context(), mux, r, "/chats.Chats/Get", gwruntime.WithHTTPPathPattern(route),
		)
		require.NoError(t, err)

		w.WriteHeader(http.StatusOK)
	})

	before := testutil.ToFloat64(fmetrics.HTTPRequests.WithLabelValues(http.MethodGet, route, "200"))

	WithMetrics(inner).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/chats/7", nil))

	after := testutil.ToFloat64(fmetrics.HTTPRequests.WithLabelValues(http.MethodGet, route, "200"))

	assert.Equal(t, 1.0, after-before, "the metric should be labelled by route, not by path")
}

// The annotator must be harmless when the metrics middleware is not installed.
func TestRouteAnnotatorWithoutTheMiddleware(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/chats/7", nil)

	assert.Nil(t, RouteAnnotator(req.Context(), req))
}

func TestWithMetricsTracksRequestsInFlight(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})

	handler := WithMetrics(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(entered)
		<-release
	}))

	before := testutil.ToFloat64(fmetrics.HTTPRequestsInFlight)

	go handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	<-entered
	assert.Equal(t, before+1, testutil.ToFloat64(fmetrics.HTTPRequestsInFlight))

	close(release)
}

// net/http accepts any RFC 7230 token as a method, so labelling by r.Method
// directly let a client mint a counter and a histogram — thirteen series — per
// request, held for the life of the process.
func TestWithMetricsBoundsTheMethodLabel(t *testing.T) {
	before := testutil.ToFloat64(fmetrics.HTTPRequests.WithLabelValues(otherMethod, unmatchedRoute, "404"))

	handler := WithMetrics(http.NotFoundHandler())

	for _, method := range []string{"EVIL0", "EVIL1", "A7f3k9"} {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Method = method

		handler.ServeHTTP(httptest.NewRecorder(), req)
	}

	after := testutil.ToFloat64(fmetrics.HTTPRequests.WithLabelValues(otherMethod, unmatchedRoute, "404"))

	assert.Equal(t, 3.0, after-before, "unknown methods must share one series")
}

func TestMethodLabel(t *testing.T) {
	for _, method := range []string{
		http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodConnect,
		http.MethodOptions, http.MethodTrace,
	} {
		assert.Equal(t, method, methodLabel(method))
	}

	assert.Equal(t, otherMethod, methodLabel("A7f3k9"))
	assert.Equal(t, otherMethod, methodLabel(""))
	assert.Equal(t, otherMethod, methodLabel("get"), "the comparison is exact, as HTTP methods are")
}
