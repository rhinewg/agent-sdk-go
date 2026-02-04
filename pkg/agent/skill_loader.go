package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Ingenimax/agent-sdk-go/pkg/interfaces"
	"gopkg.in/yaml.v3"
)

// LoadSkillFromDefinition builds a Skill from a YAML definition using the given ToolFactory.
// Instructions is used as prompt_fragment when prompt_fragment is empty (Claude/Cursor-style compatibility).
func LoadSkillFromDefinition(def SkillDefinitionYAML, toolFactory *ToolFactory, parentConfig *AgentConfig) (interfaces.Skill, error) {
	if def.Name == "" {
		return nil, fmt.Errorf("skill definition missing name")
	}
	promptFragment := strings.TrimSpace(def.PromptFragment)
	if promptFragment == "" {
		promptFragment = strings.TrimSpace(def.Instructions)
	}
	tools := make([]interfaces.Tool, 0, len(def.Tools))
	for _, tc := range def.Tools {
		if tc.Enabled != nil && !*tc.Enabled {
			continue
		}
		tool, err := toolFactory.CreateToolWithParentConfig(tc, parentConfig)
		if err != nil {
			return nil, fmt.Errorf("skill %q tool %q: %w", def.Name, tc.Name, err)
		}
		tools = append(tools, tool)
	}
	return NewSkill(def.Name, def.Description, tools, promptFragment), nil
}

// LoadSkillFromFile loads a single skill from a YAML file and registers it.
// File format: single SkillDefinitionYAML (name, description, prompt_fragment/instructions, tools).
func LoadSkillFromFile(registry interfaces.SkillRegistry, path string, toolFactory *ToolFactory, parentConfig *AgentConfig) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read skill file: %w", err)
	}
	var def SkillDefinitionYAML
	if err := yaml.Unmarshal(data, &def); err != nil {
		return fmt.Errorf("parse skill YAML: %w", err)
	}
	skill, err := LoadSkillFromDefinition(def, toolFactory, parentConfig)
	if err != nil {
		return err
	}
	registry.Register(skill)
	return nil
}

// LoadSkillsFromDir loads all .yaml/.yml files in dir as skill definitions and registers them.
// Each file should contain a single SkillDefinitionYAML (name, description, tools, prompt_fragment/instructions).
func LoadSkillsFromDir(registry interfaces.SkillRegistry, dir string, toolFactory *ToolFactory, parentConfig *AgentConfig) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read skills dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "" || (len(name) > 0 && name[0] == '.') {
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		path := filepath.Join(dir, name)
		if err := LoadSkillFromFile(registry, path, toolFactory, parentConfig); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return nil
}
