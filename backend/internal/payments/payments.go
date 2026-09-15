// Package payments defines the provider boundary (Phase 0 §9).
// Application code calls Adapter only. The ONLY implementation allowed to
// know provider-specific details is the concrete adapter (sandbox/mock for
// now — §26 Q3 provider selection is still open). Swapping providers later
// must not touch callers.
package payments

import (
	"context"
	"fmt"
)

// Money is minor units (pence). Never float.
type Money = int64

// Request carries the idempotency key every money op requires.
type Request struct {
	IdempotencyKey string
	TaskID         string
	Amount         Money
	PlatformFee    Money
	FeePayer       string
	ProviderRef    string // set on capture-side follow-ups
}

// Result is provider-agnostic.
type Result struct {
	ProviderRef string
	Status      string
}

// Adapter is the full escrow-capable surface the app needs.
type Adapter interface {
	Hold(ctx context.Context, r Request) (Result, error)
	Capture(ctx context.Context, r Request) (Result, error)
	Release(ctx context.Context, r Request) (Result, error)
	Refund(ctx context.Context, r Request) (Result, error)
	Transfer(ctx context.Context, r Request) (Result, error)
}

// Mock is the Phase 0 test double: predictable, no network, no
// provider-specific types leak through it.
type Mock struct {
	// FailOn lists operations that should error (for negative tests).
	FailOn map[string]error
	Calls  []string
}

func NewMock() *Mock { return &Mock{FailOn: map[string]error{}} }

func (m *Mock) call(op string, r Request) (Result, error) {
	m.Calls = append(m.Calls, op)
	if err, ok := m.FailOn[op]; ok {
		return Result{}, err
	}
	if r.Amount <= 0 {
		return Result{}, fmt.Errorf("payments(mock): amount must be positive")
	}
	if r.IdempotencyKey == "" {
		return Result{}, fmt.Errorf("payments(mock): idempotency key required")
	}
	return Result{ProviderRef: "mock_" + op + "_" + r.IdempotencyKey, Status: "succeeded"}, nil
}

func (m *Mock) Hold(ctx context.Context, r Request) (Result, error)     { return m.call("hold", r) }
func (m *Mock) Capture(ctx context.Context, r Request) (Result, error)  { return m.call("capture", r) }
func (m *Mock) Release(ctx context.Context, r Request) (Result, error)  { return m.call("release", r) }
func (m *Mock) Refund(ctx context.Context, r Request) (Result, error)   { return m.call("refund", r) }
func (m *Mock) Transfer(ctx context.Context, r Request) (Result, error) { return m.call("transfer", r) }

// Compile-time guard: application code depends on Adapter, never *Mock.
var _ Adapter = (*Mock)(nil)
