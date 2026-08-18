package foundation

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// The batch deadline used to be the shutdown budget, so a deployment with a
// short SHUTDOWN_TIMEOUT and a slow broker could never finish a batch: the
// events reached Kafka, the COMMIT that deletes them timed out, and the next
// run published the very same events again — forever.
func TestOutboxCourierBatchTimeoutIsIndependentOfShutdown(t *testing.T) {
	opts := NewOutboxCourierOptions()

	assert.Equal(t, OutboxDefaultBatchTimeout, opts.BatchTimeout)
	assert.NotEqual(t, DefaultShutdownTimeout, opts.BatchTimeout,
		"a steady-state batch must not inherit the shutdown budget")
	assert.Greater(t, opts.BatchTimeout, DefaultShutdownTimeout,
		"a batch has to be able to outlive a shutdown, not the other way round")
}

func TestOutboxCourierOptionDefaults(t *testing.T) {
	courier := &OutboxCourier{SpinWorker: &SpinWorker{Service: newTestService()}}

	opts := &OutboxCourierOptions{}

	// Start applies the defaults; exercise the same normalisation without
	// actually starting a service.
	if opts.BatchSize == 0 {
		opts.BatchSize = OutboxDefaultBatchSize
	}
	if opts.Interval == 0 {
		opts.Interval = OutboxDefaultInterval
	}
	if opts.BatchTimeout <= 0 {
		opts.BatchTimeout = OutboxDefaultBatchTimeout
	}

	assert.Equal(t, int32(OutboxDefaultBatchSize), opts.BatchSize)
	assert.Equal(t, OutboxDefaultInterval, opts.Interval)
	assert.Equal(t, OutboxDefaultBatchTimeout, opts.BatchTimeout)

	assert.NotNil(t, courier.newProcessFunc(opts.BatchSize, opts.BatchTimeout))
}

func TestOutboxDefaultsAreSane(t *testing.T) {
	assert.Positive(t, OutboxDefaultBatchSize)
	assert.Positive(t, OutboxDefaultInterval)
	assert.GreaterOrEqual(t, OutboxDefaultBatchTimeout, time.Minute)
}
