package foundation

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/gocraft/work"
	"github.com/gomodule/redigo/redis"
)

const (
	defaultConcurrency = 5
	defaultNamespace   = "foundation_jobs_worker"
)

// JobsWorkerContext base context for workers to use. Could be customized by the user with embedding.
type JobsWorkerContext struct {
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

// JobsWorkerOptions represents the options for starting an events worker
type JobsWorkerOptions struct {
	// JobHandlers are the handlers to use for the jobs
	JobHandlers map[string]func(job *work.Job) error
	// JobMiddlewares are the middlewares to use for all jobs
	JobMiddlewares []func(job *work.Job, next work.NextMiddlewareFunc) error
	// CronJobs are the scheduling for the jobs from JobHandlers
	CronJobs map[string]string
	// Namespace is the redis namespace to use for the jobs
	Namespace string
	// Concurrency is the number of concurrent jobs to run
	Concurrency int
	// StartComponentsOptions are the options to start the components.
	StartComponentsOptions []StartComponentsOption
}

func NewJobsWorkerOptions() *JobsWorkerOptions {
	return &JobsWorkerOptions{
		Namespace:   defaultNamespace,
		Concurrency: defaultConcurrency,
	}
}

// Start runs the worker that handles events
func (w *JobsWorker) Start(opts *JobsWorkerOptions) {
	w.Options = opts

	w.Service.Start(&StartOptions{
		ModeName:               "jobs_worker",
		StartComponentsOptions: w.Options.StartComponentsOptions,
		ServiceFunc:            w.ServiceFunc,
	})
}

func (w *JobsWorker) ServiceFunc(ctx context.Context) error {
	redisUrl := GetEnvOrString("JOBS_REDIS_URL", "")
	if redisUrl == "" {
		return fmt.Errorf("JOBS_REDIS_URL is required")
	}

	var redisPool = &redis.Pool{
		MaxActive: w.Options.Concurrency,
		MaxIdle:   w.Options.Concurrency,
		Wait:      true,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", redisUrl)
		},
	}

	workerPool := work.NewWorkerPool(JobsWorkerContext{}, uint(w.Options.Concurrency), w.Options.Namespace, redisPool)

	workerPool.Middleware(w.Logger)

	if w.Options.JobMiddlewares != nil {
		for _, middleware := range w.Options.JobMiddlewares {
			workerPool.Middleware(middleware)
		}
	}

	if w.Options.CronJobs != nil {
		for jobName, cronSpec := range w.Options.CronJobs {
			if w.Options.JobHandlers[jobName] == nil {
				return fmt.Errorf("cron job %s has no handler", jobName)
			}

			workerPool.PeriodicallyEnqueue(cronSpec, jobName)
		}
	}

	for jobName, handler := range w.Options.JobHandlers {
		workerPool.Job(jobName, handler)
	}

	workerPool.Start()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<-signalChan

	workerPool.Stop()

	return nil
}

func (w *JobsWorker) Logger(job *work.Job, next work.NextMiddlewareFunc) error {
	w.Service.Logger.Infof("Starting job %s", job.Name)
	err := next()
	if err != nil {
		w.Service.Logger.Errorf("Job %s failed: %v", job.Name, err)
	} else {
		w.Service.Logger.Infof("Job %s completed", job.Name)
	}

	return err
}
