# Skill Agent Example

This example shows how to use **Agent Skills** (reusable capability bundles) with the SDK. Skills are compatible with common agent/skill formats (Claude, OpenAI, Cursor-style).

## Quick start

1. Set `OPENAI_API_KEY`.
2. From this directory: `go run .`
3. Or build and run: `go build -o skill_agent . && ./skill_agent`

## How it works

1. **Skill registry** – Create a registry and register builtin skills and/or load from YAML:
   ```go
   registry := agent.NewSkillRegistry()
   agent.RegisterBuiltinSkills(registry)
   // Optional: agent.LoadSkillsFromDir(registry, "skills", toolFactory, nil)
   ```

2. **Agent config** – In `agents.yaml`, reference skills by name:
   ```yaml
   skills:
     - calculator
     - name: my_skill
       enabled: true
   ```

3. **Create agent** – Pass the registry so `config.Skills` are resolved:
   ```go
   agent.NewAgentFromConfig("math_helper", configs, nil,
       agent.WithLLM(llm),
       agent.WithSkillRegistry(registry),
   )
   ```

## Skill definition format (YAML)

Files under `skills/` use a universal format:

- **name**, **description** – Required.
- **prompt_fragment** or **instructions** – Optional; appended to system prompt (Claude/Cursor-style).
- **tools** – List of tools; same schema as agent config (`type`, `name`, `config`, etc.).

See `skills/calculator_skill.yaml` for an example.
