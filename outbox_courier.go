package foundation

import (
	"context"
	"encoding/json"
	"time"

	fctx "github.com/foundation-go/foundation/context"
	ferr "github.com/foundation-go/foundation/errors"
	fkafka "github.com/foundation-go/foundation/kafka"
)

const (
	OutboxDefaultBatchSize = 100
	OutboxDefaultInterval  = time.Second * 1
)

type OutboxCourier struct {
	*SpinWorker
}

// OutboxCourierOptions represents the options for starting an outbox courier
type OutboxCourierOptions struct {
	Interval               time.Duration
	BatchSize              int32
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
		Interval:  OutboxDefaultInterval,
		BatchSize: OutboxDefaultBatchSize,
		ModeName:  "outbox_courier",
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

	startOpts := NewSpinWorkerOptions()
	startOpts.ModeName = outboxOpts.ModeName
	startOpts.ProcessFunc = o.newProcessFunc(outboxOpts.BatchSize)
	startOpts.Interval = outboxOpts.Interval
	startOpts.StartComponentsOptions = append(outboxOpts.StartComponentsOptions,
		WithKafkaProducer(),
	)

	o.SpinWorker.Start(startOpts)
}

func (o *OutboxCourier) newProcessFunc(batchSize int32) func(ctx context.Context) ferr.FoundationError {
	return func(ctx context.Context) ferr.FoundationError {
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

		if len(outboxEvents) == 0 {
			o.Logger.Debug("no outbox events to publish")
			return nil
		}

		maxId := outboxEvents[len(outboxEvents)-1].ID
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
		}

		if err = o.DeleteOutboxEvents(ctx, tx, maxId); err != nil {
			return ferr.NewInternalError(err, "failed to delete outbox events")
		}

		err = tx.Commit(ctx)
		if err != nil {
			return ferr.NewInternalError(err, "failed to commit transaction")
		}

		o.Logger.Debugf("%d outbox events have published successfully", len(outboxEvents))

		return nil
	}
}
