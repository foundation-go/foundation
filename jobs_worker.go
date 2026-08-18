package foundation

import (
	"context"
	"fmt"
	"time"

	"github.com/gocraft/work"
	"github.com/robfig/cron"

	fctx "github.com/foundation-go/foundation/context"
	"github.com/foundation-go/foundation/jobs"
	fmetrics "github.com/foundation-go/foundation/metrics"
)

const (
	defaultConcurrency = 5
)

// jobsWorkerContext is the per-job context gocraft/work instantiates. It stays
// empty: everything a handler needs travels through the Go context instead, via
// JobOptions.HandlerWithContext.
type jobsWorkerContext struct {
}

type JobsWorker struct {
	*Service

	Options *JobsWorkerOptions
}

func InitJobsWorker(name string) *JobsWorker {
	return &JobsWorker{
		Service: Init(name),
	}
}

type JobOptions struct {
	// Handler runs the job. It receives no context, so it cannot be cancelled
	// and cannot carry a trace; prefer HandlerWithContext.
	Handler func(job *work.Job) error

	// HandlerWithContext runs the job with a context that is cancelled when the
	// service shuts down, and that carries the job's correlation ID.
	//
	// gocraft/work hands handlers nothing but the job, which left jobs outside
	// everything the rest of the framework does with a context: no
	// cancellation, no tracing, no correlation ID.
	HandlerWithContext func(ctx context.Context, job *work.Job) error

	Schedule string
	Options  *work.JobOptions
}

// CorrelationIDArg is the job argument Foundation reads the correlation ID
// from, so that work enqueued while handling a request stays attached to it.
const CorrelationIDArg = "correlation_id"

// JobsWorkerOptions represents the options for starting a jobs worker
type JobsWorkerOptions struct {
	// JobHandlers are the handlers to use for the jobs
	Jobs map[string]JobOptions
	// JobMiddlewares are the middlewares to use for all jobs
	Middlewares []func(job *work.Job, next work.NextMiddlewareFunc) error
	// Namespace is the redis namespace to use for the jobs
	Namespace string
	// Concurrency is the number of concurrent jobs to run
	Concurrency int
	// StartComponentsOptions are the options to start the components.
	StartComponentsOptions []StartComponentsOption
}

func NewJobsWorkerOptions() *JobsWorkerOptions {
	return &JobsWorkerOptions{
		Namespace:   jobs.DefaultNamespace,
		Concurrency: defaultConcurrency,
	}
}

// Start runs the worker that handles jobs
func (w *JobsWorker) Start(opts *JobsWorkerOptions) {
	w.Options = opts

	w.Service.Start(&StartOptions{
		ModeName:               "jobs_worker",
		StartComponentsOptions: append(w.Options.StartComponentsOptions, WithRedis()),
		ServiceFunc:            w.ServiceFunc,
	})
}

func (w *JobsWorker) ServiceFunc(ctx context.Context) error {
	// A JobsWorkerOptions built as a struct literal rather than through
	// NewJobsWorkerOptions leaves Concurrency at zero, and a pool of zero
	// workers processes nothing at all — silently.
	if w.Options.Concurrency <= 0 {
		w.Logger.Warnf("Concurrency is not set, defaulting to %d", defaultConcurrency)
		w.Options.Concurrency = defaultConcurrency
	}

	if w.Options.Namespace == "" {
		w.Options.Namespace = jobs.DefaultNamespace
	}

	redisPool, err := BuildRedisPool(w.Config.JobsEnqueuer.URL, w.Options.Concurrency)
	if err != nil {
		return fmt.Errorf("failed to build redis pool: %w", err)
	}
	defer redisPool.Close() //nolint:errcheck // best effort on shutdown

	// Guarded above, but spelled out so that the conversion is safe by
	// construction rather than by an analyser's inference.
	concurrency := uint(0)
	if w.Options.Concurrency > 0 {
		concurrency = uint(w.Options.Concurrency)
	}

	workerPool := work.NewWorkerPool(jobsWorkerContext{}, concurrency, w.Options.Namespace, redisPool)

	workerPool.Middleware(w.LoggingMiddleware)

	if w.Options.Middlewares != nil {
		for _, middleware := range w.Options.Middlewares {
			workerPool.Middleware(middleware)
		}
	}

	for jobName, jobOptions := range w.Options.Jobs {
		handler, err := w.jobHandler(ctx, jobName, jobOptions)
		if err != nil {
			return err
		}

		if jobOptions.Options != nil {
			workerPool.JobWithOptions(jobName, *jobOptions.Options, handler)
		} else {
			workerPool.Job(jobName, handler)
		}

		if jobOptions.Schedule != "" {
			if _, err := cron.Parse(jobOptions.Schedule); err != nil {
				return fmt.Errorf("job %s has an invalid schedule %q: %w", jobName, jobOptions.Schedule, err)
			}

			workerPool.PeriodicallyEnqueue(jobOptions.Schedule, jobName)
		}
	}

	workerPool.Start()

	<-ctx.Done()

	workerPool.Stop()

	return nil
}

// jobHandler resolves the handler for a job, adapting the context-aware form to
// what gocraft/work expects.
func (w *JobsWorker) jobHandler(ctx context.Context, name string, opts JobOptions) (func(*work.Job) error, error) {
	switch {
	case opts.HandlerWithContext != nil && opts.Handler != nil:
		return nil, fmt.Errorf("job %s declares both Handler and HandlerWithContext; pick one", name)

	case opts.HandlerWithContext != nil:
		handler := opts.HandlerWithContext

		return func(job *work.Job) error {
			// The correlation ID travels as a job argument, so work enqueued
			// while handling a request stays attached to it in the logs.
			jobCtx := fctx.WithCorrelationID(ctx, job.ArgString(CorrelationIDArg))
			jobCtx = fctx.WithLogger(jobCtx, w.Logger.WithField("job", job.Name))

			return handler(jobCtx, job)
		}, nil

	case opts.Handler != nil:
		return opts.Handler, nil

	default:
		return nil, fmt.Errorf("job %s has no handler", name)
	}
}

// LoggingMiddleware logs and measures every job run.
func (w *JobsWorker) LoggingMiddleware(job *work.Job, next work.NextMiddlewareFunc) error {
	w.Logger.Infof("Starting job %s", job.Name)

	started := time.Now()
	err := next()
	elapsed := time.Since(started)

	fmetrics.JobsProcessed.WithLabelValues(job.Name, fmetrics.ResultOf(err)).Inc()
	fmetrics.JobDuration.WithLabelValues(job.Name).Observe(elapsed.Seconds())

	if err != nil {
		// Jobs failing was previously visible only in the log; gocraft/work
		// retries them silently, so a job that always fails looked like
		// nothing at all.
		w.CaptureError(err, fmt.Sprintf("job %s failed", job.Name))

		return err
	}

	w.Logger.Infof("Job %s completed in %s", job.Name, elapsed)

	return nil
}
