package foundation

import (
	"context"
	"encoding/json"
	"time"

	fctx "github.com/foundation-go/foundation/context"
	ferr "github.com/foundation-go/foundation/errors"
	fkafka "github.com/foundation-go/foundation/kafka"
	fmetrics "github.com/foundation-go/foundation/metrics"
)

const (
	OutboxDefaultBatchSize = 100
	OutboxDefaultInterval  = time.Second * 1

	// OutboxDefaultBatchTimeout bounds one publish-and-delete cycle.
	//
	// It is deliberately separate from the shutdown budget. Reusing that budget
	// made every steady-state batch inherit it, so a deployment with a short
	// SHUTDOWN_TIMEOUT and a slow broker could not finish a batch at all: the
	// events reached Kafka, the COMMIT that deletes them timed out, and the
	// next run published the very same events again, forever.
	OutboxDefaultBatchTimeout = 5 * time.Minute
)

// OutboxCourier publishes events from the outbox table to Kafka.
//
// Several replicas can run at once: each takes a batch nobody else holds. That
// parallelism costs ordering — a replica skips rows another has locked, so two
// events for the same key can reach Kafka out of order, and the Hash balancer
// puts them on the same partition where consumers will see the swap. Run a
// single courier where per-key ordering matters.
type OutboxCourier struct {
	*SpinWorker
}

// OutboxCourierOptions represents the options for starting an outbox courier
type OutboxCourierOptions struct {
	Interval  time.Duration
	BatchSize int32
	// BatchTimeout bounds one publish-and-delete cycle. A batch that exceeds it
	// is rolled back and retried, so it has to be generous enough for the
	// slowest batch the broker will ever serve.
	BatchTimeout           time.Duration
	ModeName               string
	StartComponentsOptions []StartComponentsOption
}

func InitOutboxCourier(name string) *OutboxCourier {
	return &OutboxCourier{
		SpinWorker: InitSpinWorker(name),
	}
}

func NewOutboxCourierOptions() *OutboxCourierOptions {
	return &OutboxCourierOptions{
		Interval:     OutboxDefaultInterval,
		BatchSize:    OutboxDefaultBatchSize,
		BatchTimeout: OutboxDefaultBatchTimeout,
		ModeName:     "outbox_courier",
	}
}

// Start runs the outbox courier
func (o *OutboxCourier) Start(outboxOpts *OutboxCourierOptions) {
	if outboxOpts.BatchSize == 0 {
		outboxOpts.BatchSize = OutboxDefaultBatchSize
	}

	if outboxOpts.ModeName == "" {
		outboxOpts.ModeName = "outbox_courier"
	}

	if outboxOpts.Interval == 0 {
		outboxOpts.Interval = OutboxDefaultInterval
	}

	if outboxOpts.BatchTimeout <= 0 {
		outboxOpts.BatchTimeout = OutboxDefaultBatchTimeout
	}

	startOpts := NewSpinWorkerOptions()
	startOpts.ModeName = outboxOpts.ModeName
	startOpts.ProcessFunc = o.newProcessFunc(outboxOpts.BatchSize, outboxOpts.BatchTimeout)
	startOpts.Interval = outboxOpts.Interval
	startOpts.StartComponentsOptions = append(outboxOpts.StartComponentsOptions,
		WithKafkaProducer(),
	)

	o.SpinWorker.Start(startOpts)
}

func (o *OutboxCourier) newProcessFunc(batchSize int32, batchTimeout time.Duration) func(ctx context.Context) ferr.FoundationError {
	return func(ctx context.Context) ferr.FoundationError {
		// Once a batch has been written to Kafka it has to reach the COMMIT
		// that deletes it from the outbox — otherwise the next run publishes
		// the same events again. A shutdown arriving mid-batch must therefore
		// not cancel the batch; detach from the signal and bound the work with
		// a deadline of its own instead. The worker loop waits for the
		// iteration to finish before components are stopped.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), batchTimeout)
		defer cancel()

		started := time.Now()

		pool := o.GetPostgreSQL()
		tx, err := pool.Begin(ctx)
		if err != nil {
			return ferr.NewInternalError(err, "failed to begin transaction")
		}

		defer tx.Rollback(ctx) // nolint: errcheck

		outboxEvents, err := o.ListOutboxEvents(ctx, tx, batchSize)
		if err != nil {
			return ferr.NewInternalError(err, "failed to list outbox events")
		}

		// The real backlog, not the size of the batch just read: an alert on
		// "pending events" has to see two million when two million are waiting,
		// and the batch size never exceeds the limit it was read with.
		if pending, countErr := o.CountOutboxEvents(ctx, tx); countErr == nil {
			fmetrics.OutboxPendingEvents.Set(float64(pending))
		}

		if len(outboxEvents) == 0 {
			o.Logger.Debug("no outbox events to publish")
			fmetrics.OutboxOldestEventAge.Set(0)

			return nil
		}

		// The age of the oldest event is what tells a courier that is merely
		// busy apart from one that has stopped making progress.
		if oldest := outboxEvents[0].CreatedAt; oldest.Valid {
			fmetrics.OutboxOldestEventAge.Set(time.Since(oldest.Time).Seconds())
		}

		publishedIDs := make([]int64, 0, len(outboxEvents))

		for _, outboxEvent := range outboxEvents {
			headers := make(map[string]string)
			if err = json.Unmarshal(outboxEvent.Headers, &headers); err != nil {
				return ferr.NewInternalError(err, "failed to unmarshal headers")
			}

			event := &Event{
				Topic:     outboxEvent.Topic,
				Key:       outboxEvent.Key,
				Payload:   outboxEvent.Payload,
				ProtoName: headers[fkafka.HeaderProtoName],
				CreatedAt: outboxEvent.CreatedAt.Time,
				Headers:   headers,
			}

			if err = o.PublishEvent(fctx.WithCorrelationID(ctx, headers[fkafka.HeaderCorrelationID]), event, tx); err != nil {
				return ferr.NewInternalError(err, "failed to publish event")
			}

			publishedIDs = append(publishedIDs, outboxEvent.ID)
		}

		if err = o.DeleteOutboxEvents(ctx, tx, publishedIDs); err != nil {
			return ferr.NewInternalError(err, "failed to delete outbox events")
		}

		err = tx.Commit(ctx)
		if err != nil {
			return ferr.NewInternalError(err, "failed to commit transaction")
		}

		fmetrics.OutboxPublishedEvents.Add(float64(len(outboxEvents)))
		fmetrics.OutboxBatchDuration.Observe(time.Since(started).Seconds())

		o.Logger.Debugf("%d outbox events have published successfully", len(outboxEvents))

		return nil
	}
}
