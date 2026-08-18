package postgresql

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	fmetrics "github.com/foundation-go/foundation/metrics"
)

// tracerName identifies the instrumentation in the emitted spans.
const tracerName = "github.com/foundation-go/foundation/postgresql"

// QueryTracer records a span and a duration observation for every query pgx
// runs.
//
// Without it the database is a blind spot: a request trace jumps straight from
// the handler to the response with no indication of how much of the time went
// on SQL, and a slow query looks identical to a slow handler.
type QueryTracer struct{}

type queryTraceKey struct{}

type queryTrace struct {
	span    trace.Span
	started time.Time
	op      string
}

// TraceQueryStart implements pgx.QueryTracer.
func (QueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	op := queryOperation(data.SQL)

	ctx, span := otel.Tracer(tracerName).Start(ctx, "postgresql "+op,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", op),
			// The statement, not the arguments: arguments are the values a
			// query is about, and those routinely include personal data.
			attribute.String("db.statement", data.SQL),
		),
	)

	return context.WithValue(ctx, queryTraceKey{}, &queryTrace{
		span:    span,
		started: time.Now(),
		op:      op,
	})
}

// TraceQueryEnd implements pgx.QueryTracer.
func (QueryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	qt, ok := ctx.Value(queryTraceKey{}).(*queryTrace)
	if !ok {
		return
	}

	fmetrics.DatabaseQueryDuration.WithLabelValues(qt.op).Observe(time.Since(qt.started).Seconds())
	fmetrics.DatabaseQueries.WithLabelValues(qt.op, fmetrics.ResultOf(data.Err)).Inc()

	if data.Err != nil {
		qt.span.RecordError(data.Err)
		qt.span.SetStatus(codes.Error, data.Err.Error())
	}

	qt.span.End()
}

// queryOperation extracts the leading SQL verb, which is what a metric can
// safely be labelled by. Labelling by statement would create a time series per
// distinct query text.
func queryOperation(sql string) string {
	sql = strings.TrimSpace(sql)

	// sqlc prefixes generated queries with a `-- name: X :many` comment.
	for strings.HasPrefix(sql, "--") {
		_, rest, found := strings.Cut(sql, "\n")
		if !found {
			return "unknown"
		}

		sql = strings.TrimSpace(rest)
	}

	verb, _, _ := strings.Cut(sql, " ")

	verb = strings.ToUpper(strings.TrimSpace(verb))
	switch verb {
	case "SELECT", "INSERT", "UPDATE", "DELETE", "BEGIN", "COMMIT", "ROLLBACK",
		"CREATE", "ALTER", "DROP", "WITH", "COPY", "TRUNCATE", "SET":
		return verb
	case "":
		return "unknown"
	default:
		// Anything unrecognised collapses into one series rather than being
		// echoed back as a label value.
		return "other"
	}
}

// poolStatsCollector exposes pgxpool's own counters to Prometheus.
//
// Connection pool exhaustion is a common and confusing production failure: the
// service looks idle, every request is slow, and nothing in the logs says why.
type poolStatsCollector struct {
	component *Component
}

// Describe implements prometheus.Collector.
func (c poolStatsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- fmetrics.DatabasePoolTotalConns
	ch <- fmetrics.DatabasePoolIdleConns
	ch <- fmetrics.DatabasePoolAcquiredConns
	ch <- fmetrics.DatabasePoolMaxConns
}

// Collect implements prometheus.Collector.
func (c poolStatsCollector) Collect(ch chan<- prometheus.Metric) {
	pool := c.component.Connection
	if pool == nil {
		return
	}

	stat := pool.Stat()

	ch <- prometheus.MustNewConstMetric(fmetrics.DatabasePoolTotalConns, prometheus.GaugeValue, float64(stat.TotalConns()))
	ch <- prometheus.MustNewConstMetric(fmetrics.DatabasePoolIdleConns, prometheus.GaugeValue, float64(stat.IdleConns()))
	ch <- prometheus.MustNewConstMetric(fmetrics.DatabasePoolAcquiredConns, prometheus.GaugeValue, float64(stat.AcquiredConns()))
	ch <- prometheus.MustNewConstMetric(fmetrics.DatabasePoolMaxConns, prometheus.GaugeValue, float64(stat.MaxConns()))
}
