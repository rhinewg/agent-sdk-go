package interfaces

import "context"

// Skill represents a reusable capability bundle: named set of tools plus optional
// prompt fragment. Compatible with common agent/skill formats (e.g. Claude,
// OpenAI tool use, Cursor-style instructions).
type Skill interface {
	// Name returns the unique skill identifier
	Name() string
	// Description returns a short description for discovery
	Description() string
	// Tools returns the tools this skill provides (merged into agent's tools)
	Tools() []Tool
	// PromptFragment returns optional text appended to system prompt (when to use this skill)
	PromptFragment() string
}

// SkillRegistry registers and resolves skills by name
type SkillRegistry interface {
	Register(skill Skill)
	Get(name string) (Skill, bool)
	List() []Skill
}

// SkillSessionStore stores loaded skill names per session so multiple users sharing
// one agent do not affect each other. The implementation derives a session key from
// context (e.g. org_id + conversation_id). When set on the agent, load_skill/unload_skill
// tools and effective tools/prompt are session-scoped.
type SkillSessionStore interface {
	GetLoadedSkills(ctx context.Context) []string
	AddSkill(ctx context.Context, skillName string)
	RemoveSkill(ctx context.Context, skillName string)
}
