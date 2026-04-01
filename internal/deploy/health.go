package deploy

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// HealthChecker polls HTTP endpoints until they return 200 or time out.
type HealthChecker struct {
	Timeout  time.Duration
	Interval time.Duration
	Retries  int
}

// DefaultHealthChecker returns a HealthChecker with sensible defaults.
func DefaultHealthChecker() *HealthChecker {
	return &HealthChecker{
		Timeout:  60 * time.Second,
		Interval: 3 * time.Second,
		Retries:  20,
	}
}

// ServiceCheck describes a single service health check.
type ServiceCheck struct {
	Name string
	URL  string
}

// WaitForHealthy polls url until it returns HTTP 200 or the timeout expires.
func (h *HealthChecker) WaitForHealthy(url string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(h.Timeout)
	attempts := 0
	for time.Now().Before(deadline) && attempts < h.Retries {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		attempts++
		time.Sleep(h.Interval)
	}
	return fmt.Errorf("health check timed out after %s: %s", h.Timeout, url)
}

// CheckAll runs health checks for all services concurrently.
// Returns the first error encountered, or nil if all pass.
func (h *HealthChecker) CheckAll(services []ServiceCheck) error {
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	for _, svc := range services {
		wg.Add(1)
		go func(s ServiceCheck) {
			defer wg.Done()
			if err := h.WaitForHealthy(s.URL); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("%s: %w", s.Name, err))
				mu.Unlock()
			}
		}(svc)
	}
	wg.Wait()
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}
