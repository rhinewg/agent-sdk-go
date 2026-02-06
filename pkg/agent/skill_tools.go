package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Ingenimax/agent-sdk-go/pkg/interfaces"
)

// loadSkillTool is a built-in tool that lets the LLM load a skill by name.
type loadSkillTool struct{ agent *Agent }

// NewLoadSkillTool returns a tool that calls agent.LoadSkill when executed.
func NewLoadSkillTool(a *Agent) interfaces.Tool {
	return &loadSkillTool{agent: a}
}

func (t *loadSkillTool) Name() string { return "load_skill" }

func (t *loadSkillTool) Description() string {
	return "Load a skill by name so the agent can use its tools. Use when you need a capability that is not yet loaded. Pass the skill name (e.g. calculator, web_research)."
}

func (t *loadSkillTool) Parameters() map[string]interfaces.ParameterSpec {
	return map[string]interfaces.ParameterSpec{
		"skill_name": {
			Type:        "string",
			Description: "Name of the skill to load (e.g. calculator)",
			Required:    true,
		},
	}
}

func (t *loadSkillTool) Run(ctx context.Context, input string) (string, error) {
	return t.Execute(ctx, input)
}

func (t *loadSkillTool) Execute(ctx context.Context, args string) (string, error) {
	name := parseSkillNameArg(args)
	if name == "" {
		return "", fmt.Errorf("skill_name is required")
	}
	if t.agent.skillSessionStore != nil {
		// Per-session: only this user/conversation gets the skill (shared agent isolation)
		if t.agent.skillRegistry != nil {
			if _, ok := t.agent.skillRegistry.Get(name); !ok {
				return fmt.Sprintf("Skill %q not found in registry.", name), fmt.Errorf("skill %q not found", name)
			}
		}
		t.agent.skillSessionStore.AddSkill(ctx, name)
		return fmt.Sprintf("Skill %q loaded for this conversation. You can now use its tools.", name), nil
	}
	if err := t.agent.LoadSkill(ctx, name); err != nil {
		return fmt.Sprintf("Failed to load skill %q: %v", name, err), err
	}
	return fmt.Sprintf("Skill %q loaded successfully. You can now use its tools.", name), nil
}

// unloadSkillTool is a built-in tool that lets the LLM unload a skill by name.
type unloadSkillTool struct{ agent *Agent }

// NewUnloadSkillTool returns a tool that calls agent.UnloadSkill when executed.
func NewUnloadSkillTool(a *Agent) interfaces.Tool {
	return &unloadSkillTool{agent: a}
}

func (t *unloadSkillTool) Name() string { return "unload_skill" }

func (t *unloadSkillTool) Description() string {
	return "Unload a skill by name so the agent no longer uses its tools. Pass the skill name to remove."
}

func (t *unloadSkillTool) Parameters() map[string]interfaces.ParameterSpec {
	return map[string]interfaces.ParameterSpec{
		"skill_name": {
			Type:        "string",
			Description: "Name of the skill to unload",
			Required:    true,
		},
	}
}

func (t *unloadSkillTool) Run(ctx context.Context, input string) (string, error) {
	return t.Execute(ctx, input)
}

func (t *unloadSkillTool) Execute(ctx context.Context, args string) (string, error) {
	name := parseSkillNameArg(args)
	if name == "" {
		return "", fmt.Errorf("skill_name is required")
	}
	if t.agent.skillSessionStore != nil {
		t.agent.skillSessionStore.RemoveSkill(ctx, name)
		return fmt.Sprintf("Skill %q unloaded for this conversation.", name), nil
	}
	if err := t.agent.UnloadSkill(ctx, name); err != nil {
		return fmt.Sprintf("Failed to unload skill %q: %v", name, err), err
	}
	return fmt.Sprintf("Skill %q unloaded successfully.", name), nil
}

// listLoadedSkillsTool is a built-in tool that returns the list of currently loaded skill names.
type listLoadedSkillsTool struct{ agent *Agent }

// NewListLoadedSkillsTool returns a tool that returns agent.LoadedSkills() when executed.
func NewListLoadedSkillsTool(a *Agent) interfaces.Tool {
	return &listLoadedSkillsTool{agent: a}
}

func (t *listLoadedSkillsTool) Name() string { return "list_loaded_skills" }

func (t *listLoadedSkillsTool) Description() string {
	return "List the names of skills currently loaded. Use this to see which capabilities are available before calling load_skill or unload_skill."
}

func (t *listLoadedSkillsTool) Parameters() map[string]interfaces.ParameterSpec {
	return map[string]interfaces.ParameterSpec{}
}

func (t *listLoadedSkillsTool) Run(ctx context.Context, input string) (string, error) {
	return t.Execute(ctx, input)
}

func (t *listLoadedSkillsTool) Execute(ctx context.Context, args string) (string, error) {
	var names []string
	if t.agent.skillSessionStore != nil {
		names = t.agent.skillSessionStore.GetLoadedSkills(ctx)
	} else {
		names = t.agent.LoadedSkills()
	}
	if len(names) == 0 {
		return "No skills are currently loaded.", nil
	}
	return fmt.Sprintf("Loaded skills: %s", strings.Join(names, ", ")), nil
}

// listAvailableSkillsTool is a built-in tool that returns all skills in the registry (name + description).
// The LLM can use this to discover what skills exist and then call load_skill with a name.
type listAvailableSkillsTool struct{ agent *Agent }

// NewListAvailableSkillsTool returns a tool that returns registry.List() with name and description when executed.
func NewListAvailableSkillsTool(a *Agent) interfaces.Tool {
	return &listAvailableSkillsTool{agent: a}
}

func (t *listAvailableSkillsTool) Name() string { return "list_available_skills" }

func (t *listAvailableSkillsTool) Description() string {
	return "List all skills that can be loaded from the registry (name and short description). Use this to discover which skills exist before calling load_skill. Returns both currently loaded and not-yet-loaded skills."
}

func (t *listAvailableSkillsTool) Parameters() map[string]interfaces.ParameterSpec {
	return map[string]interfaces.ParameterSpec{}
}

func (t *listAvailableSkillsTool) Run(ctx context.Context, input string) (string, error) {
	return t.Execute(ctx, input)
}

func (t *listAvailableSkillsTool) Execute(ctx context.Context, args string) (string, error) {
	if t.agent.skillRegistry == nil {
		return "No skill registry configured. No skills are available.", nil
	}
	list := t.agent.skillRegistry.List()
	if len(list) == 0 {
		return "No skills are registered in the registry.", nil
	}
	// Build name: description lines for each skill
	lines := make([]string, 0, len(list))
	for _, s := range list {
		desc := s.Description()
		if desc == "" {
			desc = "(no description)"
		}
		desc = strings.ReplaceAll(desc, "\n", " ")
		lines = append(lines, fmt.Sprintf("- %s: %s", s.Name(), desc))
	}
	return "Available skills (call load_skill with the name to load):\n" + strings.Join(lines, "\n"), nil
}

// parseSkillNameArg extracts skill_name from JSON (e.g. {"skill_name":"calculator"}) or returns trimmed args as name.
func parseSkillNameArg(args string) string {
	s := strings.TrimSpace(args)
	if s == "" {
		return ""
	}
	var v struct {
		SkillName string `json:"skill_name"`
	}
	if err := json.Unmarshal([]byte(args), &v); err == nil && v.SkillName != "" {
		return strings.TrimSpace(v.SkillName)
	}
	return s
}

// DefaultSkillTools returns the built-in tools for dynamic skill load/unload when the agent has a skill registry.
// Register these so the LLM can call load_skill, unload_skill, list_loaded_skills, and list_available_skills.
func DefaultSkillTools(a *Agent) []interfaces.Tool {
	return []interfaces.Tool{
		NewLoadSkillTool(a),
		NewUnloadSkillTool(a),
		NewListLoadedSkillsTool(a),
		NewListAvailableSkillsTool(a),
	}
}
