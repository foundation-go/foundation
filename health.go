package foundation

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	fmetrics "github.com/foundation-go/foundation/metrics"
)

// DefaultHealthCheckTimeout bounds how long the readiness probe spends checking
// components before answering.
const DefaultHealthCheckTimeout = 2 * time.Second

// healthStatus is the JSON body returned by the health endpoints.
type healthStatus struct {
	Status string `json:"status"`
	// Components maps a component name to the reason it is unhealthy. Only
	// unhealthy components appear.
	Components map[string]string `json:"components,omitempty"`
}

// livenessHandler reports whether the process is running.
//
// It deliberately checks nothing else. Kubernetes restarts a pod whose liveness
// probe fails, and restarting a service because its database blipped turns a
// dependency outage into an outage of everything that depends on the service
// too. Dependencies belong in the readiness probe.
func (s *Service) livenessHandler(w http.ResponseWriter, _ *http.Request) {
	writeHealthStatus(w, http.StatusOK, healthStatus{Status: "ok"})
}

// readinessHandler reports whether the service can serve traffic right now:
// it is not shutting down, and every component reports itself healthy.
func (s *Service) readinessHandler(w http.ResponseWriter, r *http.Request) {
	if s.IsDraining() {
		writeHealthStatus(w, http.StatusServiceUnavailable, healthStatus{Status: "draining"})

		return
	}

	// Components are checked concurrently, each with its own budget.
	//
	// Sharing one deadline across a sequential loop meant the first slow
	// component consumed it and every component after that was reported down
	// with "context deadline exceeded" — one unreachable database made the
	// probe accuse Redis and Kafka too, and paged somebody for a three-way
	// outage that was not happening.
	timeout := s.healthCheckTimeout()

	var (
		mu     sync.Mutex
		wg     sync.WaitGroup
		status = healthStatus{Status: "ok"}
	)

	for _, component := range s.Components {
		wg.Add(1)

		go func(component Component) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			started := time.Now()
			err := checkComponentHealth(ctx, component)

			fmetrics.SetComponentUp(component.Name(), err == nil)

			if err == nil {
				s.Logger.Debugf("Health check for `%s` took %dms", component.Name(), time.Since(started).Milliseconds())

				return
			}

			// Probes run every few seconds. Reporting each failure to Sentry
			// would bury the project in duplicates for the length of any
			// outage, so this only logs; the readiness metric is the signal.
			s.Logger.Warnf("Health check failed for `%s`: %v", component.Name(), err)

			mu.Lock()
			defer mu.Unlock()

			if status.Components == nil {
				status.Components = make(map[string]string)
			}

			status.Components[component.Name()] = err.Error()
			status.Status = "unavailable"
		}(component)
	}

	wg.Wait()

	if status.Status != "ok" {
		writeHealthStatus(w, http.StatusServiceUnavailable, status)

		return
	}

	writeHealthStatus(w, http.StatusOK, status)
}

// healthHandler serves the historical `/health` endpoint, which has readiness
// semantics. Point liveness probes at `/live` and readiness probes at `/ready`.
func (s *Service) healthHandler(w http.ResponseWriter, r *http.Request) {
	s.readinessHandler(w, r)
}

// IsDraining reports whether the service has started shutting down and should
// no longer be sent new work.
func (s *Service) IsDraining() bool {
	return s.draining.Load()
}

// healthCheckTimeout returns the budget for a single readiness probe.
func (s *Service) healthCheckTimeout() time.Duration {
	if s.Config != nil && s.Config.HealthCheckTimeout > 0 {
		return s.Config.HealthCheckTimeout
	}

	return DefaultHealthCheckTimeout
}

// drainDelay returns how long the service keeps serving after failing readiness
// but before it starts shutting anything down.
func (s *Service) drainDelay() time.Duration {
	if s.Config != nil && s.Config.DrainDelay > 0 {
		return s.Config.DrainDelay
	}

	return 0
}

func writeHealthStatus(w http.ResponseWriter, code int, status healthStatus) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	_ = json.NewEncoder(w).Encode(status)
}
