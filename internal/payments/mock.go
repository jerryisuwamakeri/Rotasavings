package payments

import (
	"context"

	"rotasavings/internal/domain"
)

// MockProvider is a dev payment rail that always succeeds and fabricates
// references. It moves no real money. Swap for a real adapter in production.
type MockProvider struct{}

func NewMockProvider() *MockProvider { return &MockProvider{} }

func (MockProvider) Name() string { return "mock" }

func (MockProvider) Charge(_ context.Context, req ChargeRequest) (Result, error) {
	ref := domain.Hash("charge", req.GroupID, req.UserID, req.Source)
	return Result{Reference: ref, Success: true, Message: "charged (mock)"}, nil
}

func (MockProvider) Disburse(_ context.Context, req DisburseRequest) (Result, error) {
	ref := domain.Hash("disburse", req.GroupID, req.PayeeUserID, req.Destination)
	return Result{Reference: ref, Success: true, Message: "disbursed (mock)"}, nil
}
