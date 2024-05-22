package foundation

import (
	"context"
	"fmt"

	"github.com/foundation-go/foundation/jobs"

	"github.com/gocraft/work"
)

const (
	defaultConcurrency = 5
)

// jobsWorkerContext base context for workers to use
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
	Handler  func(job *work.Job) error
	Schedule string
	Options  *work.JobOptions
}

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
	redisPool, err := BuildRedisPool(w.Config.JobsEnqueuer.URL, w.Options.Concurrency)
	if err != nil {
		return fmt.Errorf("failed to build redis pool: %w", err)
	}

	workerPool := work.NewWorkerPool(jobsWorkerContext{}, uint(w.Options.Concurrency), w.Options.Namespace, redisPool)

	workerPool.Middleware(w.LoggingMiddleware)

	if w.Options.Middlewares != nil {
		for _, middleware := range w.Options.Middlewares {
			workerPool.Middleware(middleware)
		}
	}

	for jobName, jobOptions := range w.Options.Jobs {
		if jobOptions.Handler == nil {
			return fmt.Errorf("job %s has no handler", jobName)
		}

		if jobOptions.Options != nil {
			workerPool.JobWithOptions(jobName, *jobOptions.Options, jobOptions.Handler)
		} else {
			workerPool.Job(jobName, jobOptions.Handler)
		}

		if jobOptions.Schedule != "" {
			workerPool.PeriodicallyEnqueue(jobOptions.Schedule, jobName)
		}
	}

	workerPool.Start()

	<-ctx.Done()

	workerPool.Stop()

	return nil
}

func (w *JobsWorker) LoggingMiddleware(job *work.Job, next work.NextMiddlewareFunc) error {
	w.Logger.Infof("Starting job %s", job.Name)

	err := next()
	if err != nil {
		w.Logger.Errorf("Job %s failed: %v", job.Name, err)
	} else {
		w.Logger.Infof("Job %s completed", job.Name)
	}

	return err
}
