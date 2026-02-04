package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/Ingenimax/agent-sdk-go/pkg/agent"
	"github.com/Ingenimax/agent-sdk-go/pkg/llm/openai"
)

func main() {
	// 1. Create skill registry and register builtin skills (or load from YAML)
	registry := agent.NewSkillRegistry()
	agent.RegisterBuiltinSkills(registry)

	// Optional: load skills from directory (universal YAML format)
	skillsDir := "skills"
	if _, err := os.Stat(skillsDir); err == nil {
		tf := agent.NewToolFactory()
		if err := agent.LoadSkillsFromDir(registry, skillsDir, tf, nil); err != nil {
			log.Printf("Load skills from %q: %v (using builtin only)", skillsDir, err)
		}
	}

	// 2. Create LLM and load agent config
	llm := openai.NewClient(os.Getenv("OPENAI_API_KEY"))
	configPath := "agents.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}
	configPath = filepath.Clean(configPath)
	configs, err := agent.LoadAgentConfigsFromFile(configPath)
	if err != nil {
		log.Fatalf("Load config: %v", err)
	}

	// 3. Create agent with skill registry so config.Skills are resolved
	agentInstance, err := agent.NewAgentFromConfig("math_helper", configs, nil,
		agent.WithLLM(llm),
		agent.WithSkillRegistry(registry),
	)
	if err != nil {
		log.Fatalf("NewAgentFromConfig: %v", err)
	}

	// 4. Run
	ctx := context.Background()
	query := "What is (123 + 456) * 2?"
	if len(os.Args) > 2 {
		query = os.Args[2]
	}
	fmt.Println("Query:", query)
	result, err := agentInstance.Run(ctx, query)
	if err != nil {
		log.Fatalf("Run: %v", err)
	}
	fmt.Println("Result:", result)
}
