// Package scheduler is the authoritative job runner when pg_cron is
// unavailable — which is the normal case for a pure connection-string
// Postgres (vanilla images and many managed instances do not ship pg_cron,
// as it requires shared_preload_libraries).
//
// Later phases register their jobs here (auto-release, warnings, nudges,
// expirations, chat freeze, review publishing, payout batching, PII purge).
// Intervals come from app_config rows, never hardcoded. Each job must be
// idempotent: running it twice has the same effect as running it once.
package scheduler

import (
	"context"
	"sync"
	"time"
)

// Job is one scheduled unit of work. Name is unique.
type Job struct {
	Name     string
	Interval time.Duration
	Run      func(ctx context.Context) error
}

// Scheduler holds the registry. Phase 0 seeds it with the Phase-0-level
// guarantee: the registry exists and intervals resolve from config.
type Scheduler struct {
	mu   sync.Mutex
	jobs map[string]Job
}

func New() *Scheduler { return &Scheduler{jobs: map[string]Job{}} }

// Register adds (or replaces) a job. Pure registry — no timers start here,
// so tests stay deterministic.
func (s *Scheduler) Register(j Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[j.Name] = j
}

// Names returns registered job names (used by the pg_cron-fallback test).
func (s *Scheduler) Names() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.jobs))
	for n := range s.jobs {
		out = append(out, n)
	}
	return out
}

// RunOnce executes a single registered job — the shape cron workers and
// tests use. Idempotency is the job's own contract (see package doc).
func (s *Scheduler) RunOnce(ctx context.Context, name string) error {
	s.mu.Lock()
	j, ok := s.jobs[name]
	s.mu.Unlock()
	if !ok {
		return errUnknown(name)
	}
	return j.Run(ctx)
}

type unknownJob string

func (e unknownJob) Error() string { return "scheduler: unknown job " + string(e) }

func errUnknown(name string) error { return unknownJob(name) }
