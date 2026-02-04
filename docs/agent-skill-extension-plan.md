# Agent Skill 功能扩展方案

本文档基于当前 agent-sdk-go 框架的功能与结构分析，给出实现 **Agent Skill** 的扩展方案，仅作设计与方案说明，不涉及具体代码修改。

---

## 一、当前框架功能与结构概览

### 1.1 核心包结构（pkg/）

| 包名 | 职责 | 与扩展相关点 |
|------|------|--------------|
| **agent** | Agent 编排、配置、创建、运行 | Agent 配置（YAML）、Option 模式、WithAgentConfig 应用 Tools/MCP/SubAgents |
| **interfaces** | 抽象接口（LLM、Tool、Memory、MCP、Tracer 等） | `Tool`、`ToolRegistry` 为能力扩展的契约 |
| **tools** | 工具实现与注册表 | `Registry`、各 builtin 工具、`NewAgentTool`；无“技能”层级 |
| **agentconfig** | 统一配置加载（本地/远程） | 与 AgentConfig 合并、变量解析 |
| **mcp** | MCP 服务端集成（HTTP/stdio） | 能力通过 LazyMCPTool 注入 Agent |
| **llm** | 多厂商 LLM | 与 Skill 无直接耦合 |
| **memory** | 对话记忆 | 与 Skill 无直接耦合 |
| **executionplan** | 计划与执行 | 可选：Skill 可约束或参与规划 |
| **guardrails** | 安全与限流 | 可选：Skill 可带策略 |

### 1.2 能力注入现状

- **Tools**
  - 接口：`interfaces.Tool`（Name, Description, Parameters, Run, Execute）。
  - 来源：① 代码中 `WithTools(tool...)`；② YAML 中 `config.Tools`（`ToolConfigYAML`）。
  - 创建：`ToolFactory` 按 `type` 分发：`builtin` / `custom` / `agent` / `mcp`；custom 通过 `RegisterCustomTool` 注册。
- **MCP**
  - 配置：`config.MCP.MCPServers`（或 LazyMCPConfig）；每个 server 可带 `AllowedTools`。
  - 注入：MCP 工具以 `LazyMCPTool` 形式加入 `Agent.tools`，与手写/内置工具一起参与调用。
- **SubAgents**
  - 配置：`config.SubAgents`（map[string]AgentConfig）。
  - 注入：每个子 Agent 用 `tools.NewAgentTool(subAgent)` 暴露为工具，并入主 Agent 的 `tools`。
- **System Prompt**
  - 由 `AgentConfig` 的 role / goal / backstory 拼成，无“按技能追加说明”的机制。

结论：当前只有「单工具」和「MCP/SubAgent 集合」的配置与注入，没有「可复用的能力包」（名称 + 多工具 + 可选提示词）这一层抽象。

---

## 二、Agent Skill 的定位与目标

### 2.1 定义

在本框架内，将 **Agent Skill** 定义为：

- **可复用的能力包**：一个 Skill 包含一个唯一标识（名称）、以及若干可选的子项：
  - **工具集合**：一组 `Tool`（或对 builtin/custom/agent/mcp 的引用配置）；
  - **提示词片段**（可选）：一段追加到 Agent 系统提示的说明（何时用、怎么用该技能）；
  - **MCP 引用**（可选）：引用已定义的 MCP server 名称或内联片段；
  - **元数据**：描述、版本、依赖等，便于发现与组合。

Agent 通过「引用 Skill 名称」获得该能力包，而不是在每处重复罗列同样的 tools / MCP / 提示。

### 2.2 与现有概念的区分

- **与 Tool**：Skill 是「一组 Tool + 可选提示 + 可选 MCP」的集合与命名包，Tool 仍是执行单元。
- **与 MCP**：MCP 是工具来源之一；Skill 可以「包含」对某 MCP 的引用，并可再叠加其他工具或说明。
- **与 SubAgent**：SubAgent 是子 Agent 配置；Skill 不替代 SubAgent，但一个 Skill 里可以引用「暴露为 agent 类型工具的远程/本地 Agent」。
- **与 Cursor SKILL.md**：Cursor 的 Skill 偏「指导文档」；本方案中的 Skill 偏「可执行能力包」（工具 + 可选说明），必要时可兼容「从文件加载说明」作为提示片段。

### 2.3 Skill 与「总 Agent + 子 Agent」模式的区别（重要）

实现 Skill 后，**服务端承载的并不是「一个总 AGENT + 多个子功能 SUBAGENT」**，而是**一个 Agent 拥有多组由 Skill 注入的能力（tools + prompt）**。二者架构不同：

| 维度 | Skill 模式 | SubAgent 模式（现有能力） |
|------|------------|----------------------------|
| **实体数量** | 只有 **一个** Agent 实例、一个 LLM、一条对话流 | **主 Agent + 多个子 Agent**，每个子 Agent 是独立实例（自有 LLM、memory、config） |
| **能力如何挂载** | Skill 把 tools 和 prompt 片段**合并进**该 Agent；运行时只是「工具列表变多、系统提示变长」 | 子 Agent 以**工具**形式暴露给主 Agent；主 Agent 在需要时**调用**子 Agent 对应的 tool，由子 Agent 自己跑一轮 Run |
| **谁做决策** | 同一个 LLM 决定是否调用某 tool（包括来自 skill 的 tool） | 主 Agent 的 LLM 决定「要不要调用某个子 Agent」；子 Agent 被调用后用自己的 LLM 再跑一轮 |
| **是否「委派」** | 否；没有独立的子执行体，只是当前 Agent 的工具更多了 | 是；主 Agent 把任务委派给子 Agent，子 Agent 独立执行 |

因此：

- **「总 Agent + 子功能 SubAgent」**：对应的是现有 **`sub_agents`** 配置。主 Agent 把子 Agent 当工具调，形成真正的层级委派。
- **Skill 方案**：是**同一个 Agent 的能力扩展**（更多 tools + 更多 system prompt），没有增加新的 Agent 实体，也就不是「总 + 子」的架构。

若希望 server 呈现「一个总 Agent 附带多个子功能子 Agent」的形态，应使用现有的 **sub_agents**（或远程 Agent 作为 agent 类型 tool）；Skill 用来表达「可复用的能力包」，挂在**任意**一个 Agent（可以是主 Agent，也可以是某个子 Agent）上，让该 Agent 的工具与说明更丰富，而不是再拆出一层子 Agent。

### 2.4 目标

1. **复用**：同一 Skill 可被多个 Agent 引用，避免重复配置。
2. **声明式**：在 Agent YAML 中通过 `skills: [name1, name2]` 或等价方式引用。
3. **可组合**：Agent 可同时引用多个 Skill，与直接配置的 tools / MCP 并存。
4. **可扩展**：支持内置 Skill 与用户/业务自定义 Skill（代码或 YAML 定义）。
5. **与现有配置兼容**：不破坏现有 `tools` / `mcp` / `sub_agents` 的用法，Skill 作为增量扩展。

---

## 三、扩展方案设计

### 3.1 抽象层：Skill 与 SkillRegistry

- **Skill 接口**（新，建议放在 `pkg/interfaces/skill.go` 或 `pkg/skill/`）  
  - 方法建议：`Name() string`、`Description() string`、`Tools() []Tool`、`PromptFragment() string`（可选）、`MCPRefs() []string` 或等价配置。  
  - 实现体可以是「从 YAML 解析的结构体」，也可以是代码中组装的逻辑 Skill。

- **SkillRegistry**（新）  
  - 行为：`Register(skill Skill)`、`Get(name string) (Skill, bool)`、`List() []Skill`。  
  - 与 `ToolRegistry` 并列，负责按名称解析 Skill，不直接替代 ToolRegistry。

- **Skill 解析结果到 Agent 的映射**  
  - 解析 Skill 后，将其 `Tools()` 并入 Agent 的 `tools` 列表（去重仍用现有 `deduplicateTools`）。  
  - 若存在 `PromptFragment()`，则在该 Agent 的 system prompt 组装处（如 `FormatSystemPromptFromConfig` 或 WithAgentConfig 内）追加一段。  
  - MCP 引用：若 Skill 只带 MCP server 名称，可映射到已有 `config.MCP`；若 Skill 内联 MCP 配置片段，需在解析时合并进 Agent 的 MCP 配置（或 LazyMCPConfig），再走现有 MCP 工具创建逻辑。

### 3.2 配置模型扩展

- **AgentConfig 扩展**（`pkg/agent/config.go` 概念层）  
  - 新增字段，例如：`Skills []string` 或 `Skills []SkillRefYAML`。  
  - `SkillRefYAML` 可为简单字符串（技能名）或带覆盖项的结构，例如：
    - `name: string`（必填）
    - `enabled: *bool`
    - `config_overrides: map[string]interface{}`（可选，用于该 Agent 下对 Skill 的微调）。

- **Skill 定义来源（三种可并存）**  
  1. **内置 Skill**：代码中注册的 Skill（如 `web_research`、`code_helpers`），实现体从内置工具/MCP 组装。  
  2. **YAML 技能定义文件**：例如 `skills/*.yaml` 或单文件内多文档，结构建议包含：
     - `name`, `description`
     - `tools`: 列表，元素与现有 `ToolConfigYAML` 兼容（type/name/description/config/...）
     - `prompt_fragment`: 字符串（可选）
     - `mcp`: 可引用已命名 MCP 或内联 `MCPConfiguration` 片段
  3. **从目录加载**：例如约定 `skills/` 目录下每个子目录或每个 yaml 文件为一个 Skill，由 `SkillLoader` 扫描并注册到 `SkillRegistry`。

### 3.3 加载与解析流程

- **技能定义加载**（建议独立模块，如 `pkg/skill/loader.go` 或放在 `pkg/agent`）  
  - 从文件/目录/远程（若未来支持）读取 Skill 定义。  
  - 将每个定义转为实现 `Skill` 接口的结构体，并 `SkillRegistry.Register(skill)`。  
  - 若 Skill 内包含 `tools` 列表，可用现有 `ToolFactory` 逐条 `CreateTool`，结果缓存在 Skill 实现体内（或每次 Resolve 时再创建，取决于是否需 per-Agent 覆盖）。

- **在 Agent 创建时解析 skills**（在 `WithAgentConfig` 或 NewAgentFromConfigObject 的等价路径中）  
  1. 若 `config.Skills` 为空，则保持现有行为。  
  2. 否则对 `config.Skills` 中每个引用：  
     - 用 `SkillRegistry.Get(name)` 取 Skill；  
     - 若不存在可打日志并跳过或返回错误（可配置策略）；  
     - 取到的 Skill：  
       - 将其 `Tools()` 与当前已收集的 `config.Tools` 一起交给现有「去重并追加到 Agent.tools」的逻辑；  
       - 若有 `PromptFragment()`，拼接到 system prompt；  
       - 若有 MCP 引用，按上面 3.1 合并/引用 MCP 配置。  
  3. 与现有 `config.Tools`、`config.MCP`、`config.SubAgents` 的处理顺序需约定：例如先处理 Skills（展开为 tools + prompt + MCP），再处理显式 `config.Tools` / `config.MCP`，便于覆盖与去重一致。

### 3.4 LLM 运行时如何调用 Skill

**结论：LLM 不会直接“调用 skill”这个抽象，而是像今天一样只看到「工具列表 + 系统提示」；Skill 在 Agent 创建时已被展开成这两部分。**

- **时机**  
  - **创建时**：`WithAgentConfig` 解析 `config.Skills`，把每个 Skill 的 `Tools()` 合并进 `Agent.tools`，把 `PromptFragment()` 拼进 `Agent.systemPrompt`。  
  - **运行时**：每次用户请求时，Agent 用现有的 `runLocalWithTracking` → `runWithoutExecutionPlanWithToolsTracked`（或带执行计划的路径），把 `a.tools` 和 `a.systemPrompt` 传给 LLM，没有任何“skill”对象再参与。

- **LLM 侧看到什么**  
  - **System prompt**：role/goal/backstory + 各 Skill 的 `prompt_fragment`（若有），即一段自然语言说明（例如“当用户问实时信息时优先使用 websearch”）。  
  - **Tools**：一个扁平的 `[]interfaces.Tool`，包含显式配置的 tools + 从各 Skill 展开得到的 tools；LLM 只看到每个 tool 的 `Name()`、`Description()`、`Parameters()`，不知道某个 tool 来自哪个 skill。

- **“调用”如何发生**  
  - 与现有机制一致：LLM 通过 `GenerateWithTools(ctx, prompt, tools, ...)` 做 tool calling——模型在回复中决定要调用哪些 tool、传什么参数；Agent 执行对应 `tool.Execute(ctx, args)` 并把结果塞回对话，再继续生成。  
  - 因此「LLM 调用 skill」= **LLM 在合适时机选择了由该 skill 注入的那些 tools 并发起调用**。Prompt fragment 的作用是提高模型“在何时用这些工具”的准确性，而不是再给模型一个“skill”层 API。

- **小结**  
  - Skill 是**配置与打包层**（复用一组 tools + 说明），不是运行时暴露给 LLM 的一等公民。  
  - LLM 调用的是**工具**；通过 system prompt 中的 skill 说明，模型更倾向于在正确场景下使用这些工具。

### 3.5 与 ToolFactory / MCP 的衔接

- **ToolFactory**  
  - Skill 内的 `tools` 列表若与现有 `ToolConfigYAML` 一致，则直接复用 `ToolFactory.CreateToolWithParentConfig(toolConfig, parentConfig)`，无需新类型。  
  - 若未来 Skill 需要「参数化」（例如同一 Skill 不同 Agent 传入不同 API Key），可通过 `SkillRefYAML.config_overrides` 或变量替换在创建 Tool 时注入。

- **MCP**  
  - Skill 仅引用 MCP 名称时：要求该名称已在 Agent 或全局 `config.MCP.MCPServers` 中定义，Skill 只做「启用这些 MCP 工具」的标记。  
  - Skill 内联 MCP 配置时：在解析 Skill 时将该片段合并到当前 Agent 的 MCP 配置中（注意命名冲突与 AllowedTools 的合并策略）。

### 3.6 目录与文件布局建议

- **接口与实现**  
  - `pkg/interfaces/skill.go`：定义 `Skill`、`SkillRegistry`。  
  - `pkg/skill/`（或 `pkg/agent/skill.go` + `pkg/agent/skill_loader.go`）：  
    - `registry.go`：SkillRegistry 实现；  
    - `definition.go`：YAML 对应结构体（SkillDefinitionYAML）；  
    - `loader.go`：从文件/目录加载并注册；  
    - 内置 Skill 可在 `builtin.go` 或各业务包中注册。

- **配置示例**  
  - Agent YAML 中：
    ```yaml
    skills:
      - web_research
      - name: code_review
        enabled: true
    ```
  - 独立 Skill 定义文件（如 `skills/web_research.yaml`）：
    ```yaml
    name: web_research
    description: Web search and summarization
    prompt_fragment: "When the user asks about current events or facts, use web search first."
    tools:
      - type: builtin
        name: websearch
        config: { ... }
    ```

### 3.7 依赖与初始化顺序

- **CLI / 应用入口**  
  - 在加载 Agent 配置之前：  
    1. 初始化默认或用户指定的 `SkillRegistry`；  
    2. 若有技能目录/文件，调用 `SkillLoader.LoadFromDir(...)` 等注册到 SkillRegistry；  
    3. 内置 Skill 在包 init 或显式 `RegisterBuiltinSkills(registry)` 中注册。  
  - 创建 Agent 时传入或全局可访问的 SkillRegistry，在 `WithAgentConfig` 中用于解析 `config.Skills`。

- **agentconfig 包**  
  - 若使用远程/合并配置，解析后的 YAML 中若含 `skills` 字段，与本地一致处理即可；Skill 定义本身可考虑从远程「技能库」拉取（二期）。

### 3.8 终端（CLI）如何调用 Agent Skill

Skill 是「挂在某个 Agent 上的能力包」，终端侧**不单独执行某个 skill**，而是**选用已配置了 skills 的 Agent** 来执行 run / chat / task。调用方式如下。

#### 现有命令下的用法（实现 Skill 后）

| 命令 | 是否用 YAML Agent 配置 | 如何“带上 Skill” |
|------|------------------------|------------------|
| **task** | 是（`--agent-config`） | Agent 的 YAML 里写 `skills: [web_research, ...]`，执行 task 时该 Agent 已包含这些技能，无需额外参数。 |
| **run** | 否（当前用 `loadConfig()`） | 若要让 run 使用带 skills 的 Agent，需为 run 增加 `--agent-config` / `--agent`，用 YAML 创建 Agent 再执行。 |
| **chat** | 否（同 run） | 同上，可选增加 `--agent-config` / `--agent` 进入“基于 YAML Agent”的对话。 |

- **task（当前即可对接）**  
  使用带 `skills` 的 `agents.yaml`，指定使用该 YAML 中的某个 agent 执行任务即可，例如：
  ```bash
  agent-cli task --agent-config=agents.yaml --task-config=tasks.yaml --task=research_task --topic="AI"
  ```
  若 `agents.yaml` 里对应 agent 配置了 `skills: [web_research]`，该 agent 创建时会自动展开 skill 的工具与提示，无需在终端再指定 skill 名称。

- **run / chat（需 CLI 扩展）**  
  若希望 run/chat 也使用 YAML 里带 skills 的 agent，可增加参数，例如：
  ```bash
  agent-cli run "你的问题" --agent-config=agents.yaml --agent=file_analyzer
  agent-cli chat --agent-config=agents.yaml --agent=file_analyzer
  ```
  这样创建的 Agent 会包含该 YAML 中配置的 tools、MCP 与 **skills**，用户通过「选 agent」间接使用 skill。

#### 可选 CLI 扩展（便于排查与发现）

- **列出已注册技能**（例如挂在 `list` 子命令下）：
  ```bash
  agent-cli list skills
  ```
  输出当前 SkillRegistry 中所有 skill 的名称与描述，方便确认技能是否加载、名称是否写对。

- **查看某 Agent 将使用的技能**（可选）：
  ```bash
  agent-cli list agents --agent-config=agents.yaml
  ```
  输出各 agent 名称及其引用的 `skills`、`tools` 等，便于确认“终端用的这个 agent 带了哪些 skill”。

#### 小结

- **终端不提供**类似 `agent-cli skill run web_research "query"` 的“按 skill 名直接执行”命令；skill 始终通过「使用配置了该 skill 的 Agent」来生效。
- **实际用法**：在 YAML 里给 agent 配置 `skills`，终端用 **task**（或扩展后的 **run/chat**）指定该 agent 即可间接调用 agent skill。

### 3.9 Agent 动态加载与卸载 Skill

当前方案里 Skill 仅在 **Agent 创建时** 通过 `config.Skills` 解析并合并进 `Agent.tools` 与 system prompt。若要在**运行时**动态增加或移除某技能，需要额外设计以下能力。

#### 目标

- **动态加载**：在 Agent 已创建后，通过技能名称从 SkillRegistry 取 Skill，将其 tools 与 prompt 片段加入当前 Agent，后续 `Run` 即可使用。
- **动态卸载**：按技能名称移除该技能带来的 tools 和 prompt 片段，后续 `Run` 不再具备该技能。
- **可选项**：支持「仅本次会话/本次请求启用某技能」而不永久改动 Agent 实例（见下「按请求覆盖」）。

#### 设计要点

1. **运行时追踪「技能 → 工具/提示」**  
   要能正确卸载，必须知道「哪些 tools、哪段 prompt 来自哪个 skill」。可选做法：
   - **方案 A**：在 Agent 上维护 `loadedSkills map[string]skillLoadState`，其中 `skillLoadState` 保存该 skill 贡献的 `[]Tool` 和 `promptFragment` 字符串。加载时从 Registry 取 Skill，解析出 tools 与 fragment，存入该 map 并合并进 `a.tools` 与 `a.systemPrompt`；卸载时根据 map 从 `a.tools` 与 `a.systemPrompt` 中移除对应项。
   - **方案 B**：不混入 `a.tools`，而是维护 `a.staticTools`（创建时或 YAML 的 tools）与 `a.dynamicSkillTools map[string][]Tool`，运行时 `getAllTools()` = staticTools + 所有 dynamicSkillTools 的并集。卸载时从 map 删除该 skill 即可。Prompt 同理：静态 systemPrompt + 按 skill 名索引的 dynamicPromptFragments，组装时拼接，卸载时从 map 删除。  
   推荐 **方案 B**，便于卸载时无需从扁平列表里“挖出”属于某 skill 的项，且与「创建时从 config.Skills 注入」一致：创建时也可写入 staticTools/staticPrompt 与 dynamicSkillTools/dynamicPromptFragments。

2. **Agent 新增 API（示例）**  
   - `LoadSkill(ctx context.Context, skillName string) error`  
     从 Agent 注入的或全局的 SkillRegistry 取 `skillName`，若未找到返回错误；若已加载则幂等 no-op。解析该 Skill 的 `Tools()` 与 `PromptFragment()`，加入上述动态存储并更新 `a.tools` / system prompt 的组装结果。
   - `UnloadSkill(ctx context.Context, skillName string) error`  
     从动态存储中移除该 skill 对应的 tools 与 prompt 片段，更新组装结果；若该 skill 未加载则 no-op 或返回 `ErrSkillNotLoaded`（可选）。
   - `LoadedSkills() []string`  
     返回当前已动态加载的技能名称列表，便于调试与 CLI 展示。

3. **并发与一致性**  
   - 若 Agent 被多 goroutine 并发调用 `Run`，且同时有 `LoadSkill`/`UnloadSkill`，需对「动态技能与 tools/prompt 的读写」加锁（或 copy-on-write），避免竞态。  
   - `Run` 执行中应使用当前时刻的 tools 与 prompt 快照，避免执行到一半因卸载导致工具列表变化。

4. **与创建时 skills 的关系**  
   - 创建时通过 YAML `config.Skills` 注入的技能，可视为「静态已加载」：同样写入 `staticTools`/staticPrompt（或在一开始就 populate 到 dynamic 结构并标记为 from-config，卸载时可选允许/禁止卸载 config 来的技能）。  
   - 动态加载的 skill 与静态技能在运行时都表现为「当前 Agent 的 tools + prompt」，LLM 侧无区别；仅管理方式不同（静态来自配置，动态来自 API）。

5. **按请求覆盖（可选）**  
   - 若希望「仅本次 Run 使用额外/更少的技能」而不改动 Agent 实例，可在 `Run(ctx, input)` 时支持从 context 或选项传入「本次启用的 skill 名称列表」或「本次额外加载的 skill」；该次执行用「基础 Agent tools + 本次指定的 skills 展开」作为传给 LLM 的 tools/prompt，执行完不写回 Agent。这需要 Run 内部或一层薄包装能接受「skill 覆盖」参数，与现有 `interfaces.ContextContentParts` 的用法类似（例如 `WithContextSkillOverrides(ctx, []string{"web_research"})`）。

#### 实现顺序建议

- **Phase 3 之后**（或并入 Phase 3）：在 Agent 上增加 `loadedSkills` / 动态 tools+prompt 存储、`LoadSkill` / `UnloadSkill` / `LoadedSkills()`，并保证与现有「创建时从 config.Skills 注入」兼容（创建后这些技能也落在可追踪结构中，便于统一管理）。  
- **CLI**：若需在交互式 chat 中动态加载/卸载，可增加 chat 子命令或特殊命令，例如 `:load_skill web_research`、`:unload_skill web_research`、`:skills` 列出当前已加载，由 CLI 调用上述 Agent API。

#### 小结

- **动态加载**：从 SkillRegistry 按名取 Skill，将其 tools 与 prompt 片段加入 Agent 的运行时能力集（需有结构区分「按 skill 存储」以便卸载）。  
- **动态卸载**：按 skill 名从上述结构中移除对应 tools 与 prompt，更新 Agent 的对外 tools/systemPrompt。  
- **实现关键**：运行时按 skill 维度存储 tools 与 prompt，而不是只维护一个扁平列表；Agent 提供 LoadSkill/UnloadSkill/LoadedSkills，并考虑并发安全与可选「按请求覆盖」。

### 3.10 Agent Skill 在服务端（Server）的使用方式

当前框架服务端形态包括：**HTTP Server**（`pkg/microservice` 的 `NewHTTPServer` / `NewHTTPServerWithUI`）与 **gRPC 微服务**（`CreateMicroservice(agent, config)`）。二者都是「接收一个已创建好的 `*agent.Agent`，对外暴露 run/stream 等接口」。因此 **Skill 在服务端的使用 = 在创建传给 Server 的 Agent 时带上 skills，并可选在 Server 层暴露与 skill 相关的 API**。

#### 1. 启动时：创建带 Skill 的 Agent 并交给 Server

- **SkillRegistry 的初始化**  
  在进程启动、创建 Agent 之前，先初始化 SkillRegistry（与 CLI 相同逻辑）：
  - 内置 Skill 注册（若有）；
  - 从技能目录/文件加载定义并 `Register`（如 `SkillLoader.LoadFromDir("skills")`）。
  - 若使用远程/合并配置（agentconfig），Skill 定义可从远程拉取或与本地合并（二期）。

- **从配置创建 Agent**  
  - 若从 **本地 YAML** 创建：`agent.LoadAgentConfigsFromFile(configPath)` → 取对应 agent 的 `AgentConfig`，该配置中包含 `skills: [web_research, ...]`；再用 `agent.NewAgentFromFileWithName(ctx, configPath, agentName, options...)` 或等价地先 `LoadAgentConfigsFromFile` 再 `NewAgentFromConfigObject`。创建 Agent 时需能访问上述 SkillRegistry（通过 Option 传入或全局单例），以便 `WithAgentConfig` 内解析 `config.Skills` 并展开为 tools + prompt。
  - 若从 **agentconfig 远程/双源** 创建：`agentconfig.LoadAgentAuto(ctx, agentName, environment)` 等返回的已是带解析后配置的 Agent；若合并后的 YAML 中含 `skills` 字段，则创建逻辑中同样需要 SkillRegistry，与本地 YAML 路径一致。

- **将 Agent 交给 HTTP 或 gRPC**  
  - HTTP：`httpServer := microservice.NewHTTPServer(agent, port)` 或 `NewHTTPServerWithUI(agent, port, uiConfig)`，然后启动监听。  
  - gRPC：`svc, _ := microservice.CreateMicroservice(agent, microservice.Config{Port: 8080})`，然后 `svc.Start()`。  
  之后所有对 `/api/v1/agent/run`、`/api/v1/agent/stream` 或 gRPC 的 `Run`/`RunStream` 的调用，使用的都是这个已带 skills 的 Agent（skills 已在创建时展开为 tools + system prompt）。

- **小结**  
  Server 端「使用」Skill 的方式与 CLI/task 一致：**在 Agent 的配置（YAML 或远程）里声明 `skills: [...]`，在创建 Agent 时通过 SkillRegistry 解析并注入**；Server 只持有该 Agent 实例，无需在每次请求里再处理 skill。

#### 2. 现有 API 行为（无需改协议即可用上 Skill）

- **POST /api/v1/agent/run**、**POST /api/v1/agent/stream**  
  请求体中的 `input` 会传给 `agent.Run` 或 `agent.RunStream`。Agent 内部使用的已是「静态 tools + 来自 config.Skills 展开的 tools + 拼接了 prompt_fragment 的 system prompt」，因此 LLM 会自动在合适时机调用这些技能对应的工具。**无需在请求体里传 skill 名称**，也无需改现有 HTTP/gRPC 协议。

- **GET /api/v1/agent/metadata**、**GET /api/v1/agent/config**（带 UI 时）  
  当前返回 agent 的 name、description、tools 等。实现 Skill 后，可在这类接口的响应中增加 **`loaded_skills`**（或 `skills`）字段，列出当前 Agent 已加载的技能名称（来自创建时 config.Skills + 若有动态加载则来自 `LoadedSkills()`），便于前端或运维查看「这个服务当前具备哪些技能」。

#### 3. 可选：服务端暴露「动态加载/卸载」与「技能列表」API

若实现了 **3.9 动态加载与卸载**（Phase 4），可在 HTTP Server 层增加与 skill 相关的端点，供运维或上层在运行时调整能力：

- **GET /api/v1/agent/skills**  
  返回当前 Agent 已加载的技能名称列表（调用 `agent.LoadedSkills()`）。可选同时返回 SkillRegistry 中所有可用的技能名与描述（需 SkillRegistry 在 Server 可访问）。

- **POST /api/v1/agent/skills/load**  
  请求体示例：`{"skill": "web_research"}`。服务端调用 `agent.LoadSkill(ctx, name)`，成功则返回 200，失败返回 4xx/5xx 及错误信息。

- **POST /api/v1/agent/skills/unload**  
  请求体示例：`{"skill": "web_research"}`。服务端调用 `agent.UnloadSkill(ctx, name)`，成功则返回 200。

- **鉴权与多租户**  
  若 Server 已具备 `withAuth`、`withOrgContext` 等中间件，上述 skill 端点应使用相同鉴权与租户上下文，避免未授权或跨租户修改 Agent 能力。

gRPC 微服务若需同样能力，可在 proto 中增加 `ListLoadedSkills`、`LoadSkill`、`UnloadSkill` 的 RPC，并在 AgentServer 内转发到同一 Agent 的 `LoadedSkills()` / `LoadSkill` / `UnloadSkill`。

#### 4. 部署与配置建议

- **技能定义文件**  
  与 CLI 类似，服务端进程需能读取技能定义（如 `skills/*.yaml` 或统一配置文件）。部署时确保该目录或文件随服务一起下发，或在镜像/环境中配置好路径；若使用远程 agentconfig，可约定技能定义从同一配置服务或单独「技能库」拉取。

- **多 Agent / 多服务**  
  若同一进程内启动多个 Agent（例如不同 YAML 或不同 agentName），每个 Agent 创建时都会根据自身 `config.Skills` 解析技能；它们可共享同一个 SkillRegistry 实例，但每个 Agent 的已加载技能列表独立。若使用 MicroserviceManager 管理多个 gRPC 服务，每个服务对应一个 Agent，各自 skills 互不影响。

#### 5. 小结

| 场景 | 做法 |
|------|------|
| **服务启动** | 初始化 SkillRegistry（内置 + 从目录/文件或远程加载）；用带 `skills` 的 YAML 或远程配置创建 Agent；将 Agent 传给 `NewHTTPServer` 或 `CreateMicroservice`。 |
| **run/stream 请求** | 无需改协议；现有请求直接使用已带 skills 的 Agent，LLM 会按需调用技能对应工具。 |
| **元数据/配置接口** | 在 metadata 或 config 响应中增加 `loaded_skills`，便于查看当前技能。 |
| **动态加载/卸载（Phase 4）** | 在 HTTP/gRPC 层增加 GET/POST skills 相关端点，转发到 Agent 的 LoadSkill/UnloadSkill/LoadedSkills。 |

---

## 四、实现阶段建议（仅规划，不写代码）

- **Phase 1（最小可行）**  
  - 定义 `Skill` 接口与内存版 `SkillRegistry`。  
  - 在 `AgentConfig` 中增加 `Skills []string`。  
  - 在 Agent 应用配置处：根据 `Skills` 从 Registry 取 Skill，仅处理 `Tools()`，合并进 Agent 的 tools（去重）。  
  - 内置 1～2 个示例 Skill（如把现有 websearch + calculator 打成一个 skill）。

- **Phase 2**  
  - Skill 的 `PromptFragment` 与 system prompt 拼接。  
  - 从 YAML 文件/目录加载 Skill 定义并注册。  
  - `SkillRefYAML` 支持 `enabled`、简单 `config_overrides`。

- **Phase 3**  
  - Skill 内 MCP 引用或内联 MCP 片段与现有 MCP 配置合并。  
  - 依赖/版本元数据与可选「技能发现」API。  
  - 与 agentconfig 远程配置的集成（若需要）。

- **Phase 4（动态加载/卸载）**  
  - Agent 运行时按 skill 维度存储 tools 与 prompt（见 3.9）。  
  - 实现 `LoadSkill` / `UnloadSkill` / `LoadedSkills()`，与创建时 `config.Skills` 注入兼容。  
  - 并发安全与可选「按请求 skill 覆盖」。  
  - CLI chat 下可选 `:load_skill` / `:unload_skill` / `:skills` 等命令。  
  - 服务端（见 3.10）：metadata/config 响应中增加 `loaded_skills`；可选暴露 GET/POST `/api/v1/agent/skills`（列表/加载/卸载）。

---

## 五、风险与注意事项

- **命名冲突**：多个 Skill 或 Skill 与显式 tools 提供同名 Tool 时，需约定去重策略（例如先 Skill 后显式 tools，或按配置顺序后者覆盖）。  
- **循环依赖**：Skill 不应引用 Agent，只引用 Tool/MCP；若 Skill 内引用「agent 类型 tool」，仅指向 URL/名称，避免在解析 Skill 时再创建 Agent 导致循环。  
- **向后兼容**：不引入 `skills` 的现有 YAML 与代码路径行为保持不变。  
- **测试**：新增 Skill 解析与合并的单元测试；用现有 examples 的 agent YAML 加 `skills` 做回归。

---

## 六、总结

当前框架已具备 **Tool + ToolRegistry**、**MCP 工具注入**、**SubAgent 即工具** 以及 **YAML AgentConfig** 的完整链路。实现 Agent Skill 的核心是：**增加 Skill 抽象与 SkillRegistry**，在 **AgentConfig 中增加 skills 引用**，并在 **WithAgentConfig（或等价的配置应用处）** 中解析引用、将 Skill 的 tools（及可选的 prompt、MCP）并入现有流程。本方案保持与现有 tools/mcp/sub_agents 兼容，并分阶段可落地。
