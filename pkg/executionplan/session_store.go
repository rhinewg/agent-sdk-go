package executionplan

import (
	"context"
	"sync"
)

// SessionKeyFunc returns a key that identifies the session (for example org_id:conversation_id).
// If it returns an empty string, all plans are stored in a single shared session partition.
type SessionKeyFunc func(context.Context) string

// ExecutionPlanSessionStore defines a session-scoped execution plan storage interface.
// A typical implementation uses a nested map: sessionKey -> taskID -> *ExecutionPlan.
type ExecutionPlanSessionStore interface {
	// StorePlan stores the plan under the current session.
	StorePlan(ctx context.Context, plan *ExecutionPlan)
	// GetPlanByTaskID retrieves a plan by taskID under the current session.
	GetPlanByTaskID(ctx context.Context, taskID string) (*ExecutionPlan, bool)
	// ListPlans lists all plans under the current session.
	ListPlans(ctx context.Context) []*ExecutionPlan
	// DeletePlan deletes a plan by taskID under the current session.
	DeletePlan(ctx context.Context, taskID string) bool
}

// DefaultExecutionPlanSessionStore is an in-memory implementation:
// sessionKey -> taskID -> *ExecutionPlan.
type DefaultExecutionPlanSessionStore struct {
	keyFunc SessionKeyFunc

	mu    sync.RWMutex
	plans map[string]map[string]*ExecutionPlan
}

// NewDefaultExecutionPlanSessionStore creates a new session-scoped plan store.
// keyFunc extracts the session key from context; when nil, a single shared session (empty key) is used.
func NewDefaultExecutionPlanSessionStore(keyFunc SessionKeyFunc) *DefaultExecutionPlanSessionStore {
	if keyFunc == nil {
		keyFunc = func(context.Context) string { return "" }
	}

	return &DefaultExecutionPlanSessionStore{
		keyFunc: keyFunc,
		plans:   make(map[string]map[string]*ExecutionPlan),
	}
}

// getSessionKeyFromContext returns the session key for the current request; empty means shared session.
func (s *DefaultExecutionPlanSessionStore) getSessionKeyFromContext(ctx context.Context) string {
	if s == nil || s.keyFunc == nil {
		return ""
	}
	return s.keyFunc(ctx)
}

// StorePlan stores the plan under the current session.
func (s *DefaultExecutionPlanSessionStore) StorePlan(ctx context.Context, plan *ExecutionPlan) {
	if plan == nil {
		return
	}

	key := s.getSessionKeyFromContext(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.plans[key] == nil {
		s.plans[key] = make(map[string]*ExecutionPlan)
	}
	s.plans[key][plan.TaskID] = plan
}

// GetPlanByTaskID retrieves a plan by taskID under the current session.
func (s *DefaultExecutionPlanSessionStore) GetPlanByTaskID(ctx context.Context, taskID string) (*ExecutionPlan, bool) {
	key := s.getSessionKeyFromContext(ctx)

	s.mu.RLock()
	defer s.mu.RUnlock()

	sessionPlans, ok := s.plans[key]
	if !ok {
		return nil, false
	}

	plan, exists := sessionPlans[taskID]
	return plan, exists
}

// ListPlans lists all plans under the current session.
func (s *DefaultExecutionPlanSessionStore) ListPlans(ctx context.Context) []*ExecutionPlan {
	key := s.getSessionKeyFromContext(ctx)

	s.mu.RLock()
	defer s.mu.RUnlock()

	sessionPlans, ok := s.plans[key]
	if !ok || len(sessionPlans) == 0 {
		return []*ExecutionPlan{}
	}

	plans := make([]*ExecutionPlan, 0, len(sessionPlans))
	for _, plan := range sessionPlans {
		plans = append(plans, plan)
	}
	return plans
}

// DeletePlan deletes a plan by taskID under the current session.
func (s *DefaultExecutionPlanSessionStore) DeletePlan(ctx context.Context, taskID string) bool {
	key := s.getSessionKeyFromContext(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()

	sessionPlans, ok := s.plans[key]
	if !ok {
		return false
	}

	if _, exists := sessionPlans[taskID]; !exists {
		return false
	}

	delete(sessionPlans, taskID)

	// If there are no plans left for this session, remove the session entry.
	if len(sessionPlans) == 0 {
		delete(s.plans, key)
	}

	return true
}

