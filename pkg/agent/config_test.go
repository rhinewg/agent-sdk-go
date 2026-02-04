package agent

import (
	"context"
	"testing"

	"github.com/Ingenimax/agent-sdk-go/pkg/interfaces"
	"github.com/stretchr/testify/assert"
)

func TestFormatSystemPromptFromConfig(t *testing.T) {
	// Create an agent config
	config := AgentConfig{
		Role:      "{topic} Senior Data Researcher",
		Goal:      "Uncover cutting-edge developments in {topic}",
		Backstory: "You're a seasoned researcher with a knack for uncovering the latest developments in {topic}.",
	}

	// Create variables
	variables := map[string]string{
		"topic": "Artificial Intelligence",
	}

	// Format the system prompt
	systemPrompt := FormatSystemPromptFromConfig(config, variables)

	// Assert that the prompt was formatted correctly
	assert.Contains(t, systemPrompt, "# Role\nArtificial Intelligence Senior Data Researcher")
	assert.Contains(t, systemPrompt, "# Goal\nUncover cutting-edge developments in Artificial Intelligence")
	assert.Contains(t, systemPrompt, "# Backstory\nYou're a seasoned researcher with a knack for uncovering the latest developments in Artificial Intelligence.")
}

func TestGetAgentForTask(t *testing.T) {
	// Create task configs
	taskConfigs := TaskConfigs{
		"research_task": TaskConfig{
			Description:    "Conduct research on {topic}",
			ExpectedOutput: "A report on {topic}",
			Agent:          "researcher",
		},
		"reporting_task": TaskConfig{
			Description:    "Create a report on {topic}",
			ExpectedOutput: "A detailed report on {topic}",
			Agent:          "reporting_analyst",
			OutputFile:     "report.md",
		},
	}

	// Test getting an existing agent
	agent, err := GetAgentForTask(taskConfigs, "research_task")
	assert.NoError(t, err)
	assert.Equal(t, "researcher", agent)

	// Test getting another existing agent
	agent, err = GetAgentForTask(taskConfigs, "reporting_task")
	assert.NoError(t, err)
	assert.Equal(t, "reporting_analyst", agent)

	// Test getting a non-existent agent
	_, err = GetAgentForTask(taskConfigs, "nonexistent_task")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in configuration")
}

func TestLoadAgentConfigsFromFile(t *testing.T) {
	// Test with structured output
	configs, err := LoadAgentConfigsFromFile("testdata/agents_with_structured_output.yaml")
	if err != nil {
		t.Fatalf("Failed to load agent configs: %v", err)
	}

	// Check that we have the expected agent
	config, exists := configs["researcher"]
	if !exists {
		t.Fatal("Expected 'researcher' agent not found")
	}

	// Check basic fields
	if config.Role == "" {
		t.Error("Expected role to be set")
	}
	if config.Goal == "" {
		t.Error("Expected goal to be set")
	}
	if config.Backstory == "" {
		t.Error("Expected backstory to be set")
	}

	// Check structured output configuration
	if config.ResponseFormat == nil {
		t.Fatal("Expected ResponseFormat to be set")
	}

	if config.ResponseFormat.Type != "json_object" {
		t.Errorf("Expected type 'json_object', got '%s'", config.ResponseFormat.Type)
	}

	if config.ResponseFormat.SchemaName != "ResearchResult" {
		t.Errorf("Expected schema name 'ResearchResult', got '%s'", config.ResponseFormat.SchemaName)
	}

	// Check that schema definition exists
	if config.ResponseFormat.SchemaDefinition == nil {
		t.Fatal("Expected SchemaDefinition to be set")
	}

	// Check that schema has the expected structure
	schema := config.ResponseFormat.SchemaDefinition
	if schema["type"] != "object" {
		t.Errorf("Expected schema type 'object', got '%v'", schema["type"])
	}

	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected properties to be a map")
	}

	if _, exists := properties["findings"]; !exists {
		t.Error("Expected 'findings' property in schema")
	}

	if _, exists := properties["summary"]; !exists {
		t.Error("Expected 'summary' property in schema")
	}
}

func TestConvertYAMLSchemaToResponseFormat(t *testing.T) {
	// Test with valid config
	config := &ResponseFormatConfig{
		Type:       "json_object",
		SchemaName: "TestSchema",
		SchemaDefinition: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"test_field": map[string]interface{}{
					"type": "string",
				},
			},
		},
	}

	responseFormat, err := ConvertYAMLSchemaToResponseFormat(config)
	if err != nil {
		t.Fatalf("Failed to convert schema: %v", err)
	}

	if responseFormat == nil {
		t.Fatal("Expected non-nil ResponseFormat")
		return // This return is never reached but helps linter understand
	}

	if responseFormat.Type != interfaces.ResponseFormatJSON {
		t.Errorf("Expected ResponseFormatJSON, got %v", responseFormat.Type)
	}

	if responseFormat.Name != "TestSchema" {
		t.Errorf("Expected name 'TestSchema', got '%s'", responseFormat.Name)
	}

	// Test with nil config
	responseFormat, err = ConvertYAMLSchemaToResponseFormat(nil)
	if err != nil {
		t.Fatalf("Expected no error for nil config, got: %v", err)
	}

	if responseFormat != nil {
		t.Fatal("Expected nil ResponseFormat for nil config")
	}
}

func TestLoadAgentConfigsWithSkills(t *testing.T) {
	configs, err := LoadAgentConfigsFromFile("testdata/agents_with_skills.yaml")
	assert.NoError(t, err)
	config, ok := configs["test_agent"]
	assert.True(t, ok)
	assert.Len(t, config.Skills, 2)
	// First: string form -> Name only
	assert.Equal(t, "calculator", config.Skills[0].Name)
	assert.Nil(t, config.Skills[0].Enabled)
	// Second: object form
	assert.Equal(t, "optional_skill", config.Skills[1].Name)
	assert.NotNil(t, config.Skills[1].Enabled)
	assert.False(t, *config.Skills[1].Enabled)
}

func TestAgentWithSkillRegistry(t *testing.T) {
	registry := NewSkillRegistry()
	RegisterBuiltinSkills(registry)
	skill, ok := registry.Get("calculator")
	assert.True(t, ok)
	assert.Equal(t, "calculator", skill.Name())
	assert.NotEmpty(t, skill.Description())
	assert.Len(t, skill.Tools(), 1)
	assert.Equal(t, "calculator", skill.Tools()[0].Name())
	assert.NotEmpty(t, skill.PromptFragment())
}

func TestAgentLoadSkillUnloadSkill(t *testing.T) {
	registry := NewSkillRegistry()
	RegisterBuiltinSkills(registry)
	llm := &TestMockLLM{llmName: "test"}
	a, err := NewAgent(
		WithLLM(llm),
		WithName("test"),
		WithSkillRegistry(registry),
		WithSystemPrompt("You are a helper."),
	)
	assert.NoError(t, err)
	ctx := context.Background()

	// Initially no dynamic skills
	assert.Empty(t, a.LoadedSkills())

	// LoadSkill adds the skill
	err = a.LoadSkill(ctx, "calculator")
	assert.NoError(t, err)
	assert.Len(t, a.LoadedSkills(), 1)
	assert.Contains(t, a.LoadedSkills(), "calculator")
	tools := a.GetTools()
	toolNames := make([]string, len(tools))
	for i, tl := range tools {
		toolNames[i] = tl.Name()
	}
	assert.Contains(t, toolNames, "calculator")
	assert.Contains(t, a.GetSystemPrompt(), "Skills")
	assert.Contains(t, a.GetSystemPrompt(), "calculator")

	// UnloadSkill removes the skill (default skill tools load_skill/unload_skill/list_loaded_skills remain)
	err = a.UnloadSkill(ctx, "calculator")
	assert.NoError(t, err)
	assert.Empty(t, a.LoadedSkills())
	tools2 := a.GetTools()
	// Still have the 3 default skill tools
	assert.GreaterOrEqual(t, len(tools2), 3)
	assert.NotContains(t, a.GetSystemPrompt(), "# Skills")
}
