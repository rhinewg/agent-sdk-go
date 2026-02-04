# Agent Skills

Agent Skills are **reusable capability bundles**: a named set of tools plus an optional prompt fragment. They are resolved when applying agent config and merged into the agent's tools and system prompt. The format is compatible with common agent/skill schemas (Claude, OpenAI, Cursor-style).

## Agent config

Reference skills in `agents.yaml` by name:

```yaml
my_agent:
  role: ...
  goal: ...
  backstory: ...
  skills:
    - calculator
    - name: web_research
      enabled: true
```

- **String form**: `- calculator` (name only, enabled by default).
- **Object form**: `- name: web_research` with optional `enabled` and `config_overrides`.

## Skill registry

Create a registry and pass it when building the agent so `config.Skills` can be resolved:

```go
registry := agent.NewSkillRegistry()
agent.RegisterBuiltinSkills(registry)
// Optional: load from directory
agent.LoadSkillsFromDir(registry, "skills", agent.NewToolFactory(), nil)

agentInstance, err := agent.NewAgentFromConfig(agentName, configs, nil,
    agent.WithLLM(llm),
    agent.WithSkillRegistry(registry),
)
```

If `WithSkillRegistry` is not set, `config.Skills` is ignored (no error).

## Skill definition (YAML)

Skill definition files (e.g. under `skills/*.yaml`) use this schema:

| Field             | Required | Description |
|-------------------|----------|-------------|
| `name`            | Yes      | Unique skill id |
| `description`     | Yes      | Short description |
| `prompt_fragment` | No       | Text appended to system prompt (when to use this skill) |
| `instructions`    | No       | Alias for `prompt_fragment` (Claude/Cursor compatibility) |
| `tools`           | No       | List of tool configs (same as agent `tools`: `type`, `name`, `config`, etc.) |

Example:

```yaml
name: calculator
description: Perform mathematical calculations
prompt_fragment: When the user asks for a numeric calculation, use the calculator tool.
tools:
  - type: builtin
    name: calculator
    enabled: true
```

## Dynamic load and unload

### Programmatic API

```go
err := agentInstance.LoadSkill(ctx, "web_research")
err := agentInstance.UnloadSkill(ctx, "web_research")
names := agentInstance.LoadedSkills()
```

### Built-in tools (LLM can call)

When the agent is created with `WithSkillRegistry(registry)`, three default tools are registered so the LLM can load/unload skills during the conversation:

| Tool                 | Description |
|----------------------|-------------|
| **load_skill**       | Load a skill by name. Parameter: `skill_name` (e.g. `calculator`). |
| **unload_skill**     | Unload a skill by name. Parameter: `skill_name`. |
| **list_loaded_skills** | List currently loaded skill names (no parameters). |

The model can call these like any other tool (e.g. when the user asks to “add calculator” or “remove web search”). Skills from `config.Skills` are stored the same way and can be unloaded via the **unload_skill** tool or `UnloadSkill()`.

## Shared agent: per-user / per-conversation isolation

When multiple users share one agent, global LoadSkill/UnloadSkill would affect everyone. To keep each user or conversation independent, set a per-session skill store:

```go
store := agent.NewDefaultSkillSessionStore(agent.DefaultSessionKey)
agentInstance, err := agent.NewAgent(...,
    agent.WithSkillRegistry(registry),
    agent.WithSkillSessionStore(store),
)
```

- **Session key**: The default key is `org_id:conversation_id` (or `conversation_id` only when `org_id` is absent). Pass **conversation_id** (and optionally **org_id**) in the request context so each conversation gets its own loaded skills. HTTP: set `conversation_id` and `org_id` in the request (microservice already puts them in context). Code: `ctx = memory.WithConversationID(ctx, "conv-123"); ctx = multitenancy.WithOrgID(ctx, "org-1")` before `agent.Run(ctx, input)`.
- **Behavior**: When `WithSkillSessionStore` is set, load_skill and unload_skill update only the store for the current session. Each Run uses static agent config plus that session's loaded skills, so different users/conversations do not affect each other.
- **Without store**: If you do not set `WithSkillSessionStore`, load_skill/unload_skill change the agent's global state (all users share it).

## CLI usage

The CLI supports agent skills via optional flags and a new list subcommand.

### task

Use `--skills-dir=<path>` to load YAML skill definitions from a directory (in addition to builtin skills). The registry is passed to the agent so `skills` in the agent config are resolved.

```bash
agent-cli task --agent-config=agents.yaml --task-config=tasks.yaml --task=research_task --topic "AI" --skills-dir=./skills
```

### run

Use `--agent-config=<path>`, `--agent=<name>`, and optionally `--skills-dir=<path>` to run with an agent defined in YAML (with skills). Without these flags, the default CLI agent is used.

```bash
agent-cli run "Calculate 2+2" --agent-config=agents.yaml --agent=my_agent --skills-dir=./skills
```

### chat

Same flags as `run`. When using a YAML agent, the CLI enables **per-conversation skill isolation** via `WithSkillSessionStore`, so load_skill/unload_skill and effective tools are scoped to the current chat session (using `conversation_id` from config).

```bash
agent-cli chat --agent-config=agents.yaml --agent=my_agent --skills-dir=./skills
```

### list skills

List registered skills (builtin plus any loaded from `--skills-dir`).

```bash
agent-cli list skills
agent-cli list skills --skills-dir=./skills
```

## Builtin skills

- **calculator** – Registered via `agent.RegisterBuiltinSkills(registry)`; provides the calculator tool and a short prompt fragment.

### MCP 技能与工具命名建议

使用 MCP 搜索类技能时，建议：

- **技能名与提示词一致**：在技能定义中明确使用实际技能名（如 `mcp_web_search`），并在 `prompt_fragment` 里直接引用这个名字，避免模型臆造诸如 `web_research` 之类不存在的技能。
- **明确工具名**：在提示词中注明 MCP 工具的真实名称（例如从 AliyunBailianMCP_WebSearch 发现的 `bailian_web_search`），引导模型直接调用该工具，而不是虚构新的工具名。
- **禁止虚构能力**：在技能提示中显式说明“不要创建新的技能名或工具名（如 `web_research`），只能使用配置中实际存在的技能和 MCP 工具”。

## Compatibility

- **Claude / OpenAI**: Tools use the same shape as agent tool config (name, description, parameters/config). Prompt fragment maps to system/instruction text.
- **Cursor SKILL.md**: Use `instructions` or `prompt_fragment` for the instruction body; define `tools` separately in YAML.

See [agent-skill-extension-plan.md](agent-skill-extension-plan.md) for the full design.
