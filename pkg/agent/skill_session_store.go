package agent

import (
	"context"
	"sync"

	"github.com/Ingenimax/agent-sdk-go/pkg/interfaces"
	"github.com/Ingenimax/agent-sdk-go/pkg/memory"
	"github.com/Ingenimax/agent-sdk-go/pkg/multitenancy"
)

// SessionKeyFunc returns a key that identifies the session for skill isolation (e.g. per user or per conversation).
// Return empty string to disable per-session isolation for that request.
type SessionKeyFunc func(context.Context) string

// DefaultSessionKey returns org_id:conversation_id when both are set, else conversation_id, else "".
// Use this so different orgs and conversations get isolated skill sets when sharing one agent.
func DefaultSessionKey(ctx context.Context) string {
	org, _ := multitenancy.GetOrgID(ctx)
	conv, ok := memory.GetConversationID(ctx)
	if !ok || conv == "" {
		return ""
	}
	if org != "" {
		return org + ":" + conv
	}
	return conv
}

// DefaultSkillSessionStore is an in-memory per-session skill store.
// It uses keyFunc to get the session key from context; load_skill/unload_skill are scoped per key.
type DefaultSkillSessionStore struct {
	keyFunc     SessionKeyFunc
	mu          sync.RWMutex
	skills      map[string]map[string]struct{} // sessionKey -> set of skill names
	initialized map[string]bool                // sessionKey -> whether auto-load has happened (prevents re-auto-load after user unloads)
}

// NewDefaultSkillSessionStore creates a session store that uses keyFunc(ctx) as the session key.
// Pass DefaultSessionKey so that org_id and conversation_id from context isolate skills per conversation.
func NewDefaultSkillSessionStore(keyFunc SessionKeyFunc) *DefaultSkillSessionStore {
	if keyFunc == nil {
		keyFunc = DefaultSessionKey
	}
	return &DefaultSkillSessionStore{
		keyFunc:     keyFunc,
		skills:      make(map[string]map[string]struct{}),
		initialized: make(map[string]bool),
	}
}

// GetLoadedSkills implements interfaces.SkillSessionStore
func (s *DefaultSkillSessionStore) GetLoadedSkills(ctx context.Context) []string {
	key := s.keyFunc(ctx)
	if key == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	set, ok := s.skills[key]
	if !ok || len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	return out
}

// AddSkill implements interfaces.SkillSessionStore
func (s *DefaultSkillSessionStore) AddSkill(ctx context.Context, skillName string) {
	key := s.keyFunc(ctx)
	if key == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.skills[key] == nil {
		s.skills[key] = make(map[string]struct{})
	}
	s.skills[key][skillName] = struct{}{}
	// Mark as initialized when a skill is added (either auto-loaded or manually loaded)
	s.initialized[key] = true
}

// IsInitialized returns whether the session has been initialized (auto-load has happened or user has loaded/unloaded skills).
// Used to prevent re-auto-loading config.Skills after user has explicitly unloaded them.
func (s *DefaultSkillSessionStore) IsInitialized(ctx context.Context) bool {
	key := s.keyFunc(ctx)
	if key == "" {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.initialized[key]
}

// RemoveSkill implements interfaces.SkillSessionStore
func (s *DefaultSkillSessionStore) RemoveSkill(ctx context.Context, skillName string) {
	key := s.keyFunc(ctx)
	if key == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.skills[key], skillName)
	// Mark as initialized when a skill is removed (user explicitly unloaded)
	s.initialized[key] = true
}

// Ensure DefaultSkillSessionStore implements SkillSessionStore
var _ interfaces.SkillSessionStore = (*DefaultSkillSessionStore)(nil)
