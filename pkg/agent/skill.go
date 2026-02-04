package agent

import (
	"sync"

	"github.com/Ingenimax/agent-sdk-go/pkg/interfaces"
	"github.com/Ingenimax/agent-sdk-go/pkg/tools/calculator"
)

// skillImpl implements interfaces.Skill from a resolved set of tools and optional prompt fragment
type skillImpl struct {
	name           string
	description    string
	tools          []interfaces.Tool
	promptFragment string
}

// Name implements interfaces.Skill
func (s *skillImpl) Name() string { return s.name }

// Description implements interfaces.Skill
func (s *skillImpl) Description() string { return s.description }

// Tools implements interfaces.Skill
func (s *skillImpl) Tools() []interfaces.Tool { return s.tools }

// PromptFragment implements interfaces.Skill
func (s *skillImpl) PromptFragment() string { return s.promptFragment }

// NewSkill creates a Skill from name, description, tools and optional prompt fragment.
// Used for builtin skills and for skills loaded from YAML (after tools are created via ToolFactory).
func NewSkill(name, description string, tools []interfaces.Tool, promptFragment string) interfaces.Skill {
	return &skillImpl{
		name:           name,
		description:    description,
		tools:          tools,
		promptFragment: promptFragment,
	}
}

// SkillRegistryImpl implements interfaces.SkillRegistry
type SkillRegistryImpl struct {
	mu     sync.RWMutex
	skills map[string]interfaces.Skill
}

// NewSkillRegistry creates a new skill registry
func NewSkillRegistry() *SkillRegistryImpl {
	return &SkillRegistryImpl{skills: make(map[string]interfaces.Skill)}
}

// Register implements interfaces.SkillRegistry
func (r *SkillRegistryImpl) Register(skill interfaces.Skill) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills[skill.Name()] = skill
}

// Get implements interfaces.SkillRegistry
func (r *SkillRegistryImpl) Get(name string) (interfaces.Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.skills[name]
	return s, ok
}

// List implements interfaces.SkillRegistry
func (r *SkillRegistryImpl) List() []interfaces.Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]interfaces.Skill, 0, len(r.skills))
	for _, s := range r.skills {
		out = append(out, s)
	}
	return out
}

// RegisterBuiltinSkills registers built-in skills (e.g. calculator) into the given registry.
// Compatible with common agent/skill formats; use WithSkillRegistry(registry) when creating agents.
func RegisterBuiltinSkills(registry interfaces.SkillRegistry) {
	registry.Register(NewSkill(
		"calculator",
		"Perform mathematical calculations (add, subtract, multiply, divide, exponents)",
		[]interfaces.Tool{calculator.New()},
		"When the user asks for a numeric calculation or math expression, use the calculator tool.",
	))
}
