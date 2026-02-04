# 使用 bin/agent-cli 测试 Agent Skill 功能

以下命令均在**仓库根目录**执行，使用 `./bin/agent-cli`。先构建 CLI：`make build-cli`。

## 1. 列出技能（无需 API Key）

```bash
# 仅内置技能（calculator）
./bin/agent-cli list skills

# 内置 + 指定目录下的 YAML 技能
./bin/agent-cli list skills --skills-dir=examples/skill_agent/skills
```

预期：输出 `calculator` 及描述；带 `--skills-dir` 时会有目录中的技能（若同名则覆盖/合并，以实际注册为准）。

---

## 2. run：用 YAML Agent + 技能跑单次对话

需要配置 LLM（如 `~/.agent-cli/config.json` 或环境变量 `OPENAI_API_KEY` 等）。使用示例里的 `math_helper` 和技能目录：

```bash
./bin/agent-cli run "3 加 5 等于多少？" \
  --agent-config=examples/skill_agent/agents.yaml \
  --agent=math_helper \
  --skills-dir=examples/skill_agent/skills
```

预期：Agent 使用 calculator 技能并返回 8。

---

## 3. chat：交互式对话 + 按会话技能隔离

同样使用 YAML Agent 和技能目录，会话内 load/unload 仅影响当前会话：

```bash
./bin/agent-cli chat \
  --agent-config=examples/skill_agent/agents.yaml \
  --agent=math_helper \
  --skills-dir=examples/skill_agent/skills
```

进入 chat 后可输入数学题测试计算器，或输入 `help` 查看命令。

---

## 4. task：执行任务并指定技能目录

需要同时提供 agent 配置、任务配置和任务名。例如用现有 research 任务并挂上技能目录（任务本身可能不引用技能，仅验证 `--skills-dir` 被正确传入）：

```bash
./bin/agent-cli task \
  --agent-config=examples/agent_config_yaml/agents.yaml \
  --task-config=examples/agent_config_yaml/tasks.yaml \
  --task=research_task \
  --topic=AI \
  --skills-dir=examples/skill_agent/skills
```

若希望用带技能的 agent（如 `math_helper`）跑任务，需要为该 agent 编写对应的 `tasks.yaml`（含 `agent: math_helper` 的 task），再指定该 task 与上述 `--agent-config` / `--skills-dir`。

---

## 5. 不指定 YAML 时的默认行为

不传 `--agent-config` / `--agent` 时，run 和 chat 使用默认 CLI 配置（`createAgent(config)`），不会加载 YAML 中的 skills：

```bash
./bin/agent-cli run "Hello"
./bin/agent-cli chat
```

---

## 6. 帮助与版本

```bash
./bin/agent-cli --help
./bin/agent-cli list skills --help   # 若支持则显示；否则用 list skills 即可
./bin/agent-cli version
```

---

## 快速自检清单

| 项目           | 命令 |
|----------------|------|
| 列出技能       | `./bin/agent-cli list skills`、`./bin/agent-cli list skills --skills-dir=examples/skill_agent/skills` |
| run + 技能     | `./bin/agent-cli run "3+5?" --agent-config=examples/skill_agent/agents.yaml --agent=math_helper --skills-dir=examples/skill_agent/skills` |
| chat + 技能    | `./bin/agent-cli chat --agent-config=examples/skill_agent/agents.yaml --agent=math_helper --skills-dir=examples/skill_agent/skills` |
| task + 技能目录 | `./bin/agent-cli task --agent-config=... --task-config=... --task=research_task --skills-dir=examples/skill_agent/skills` |

首次 run/chat/task 前若未初始化，可先执行：`./bin/agent-cli init`，并按提示配置 Provider 和 API Key。
