# MCP (Model Context Protocol) 完整用法指南

本文档详细说明如何在 agent-sdk-go 中使用 MCP (Model Context Protocol) 连接外部工具和数据源。

## 目录

1. [MCP 概述](#mcp-概述)
2. [快速开始](#快速开始)
3. [配置方式详解](#配置方式详解)
4. [实际使用示例](#实际使用示例)
5. [与原生工具对比](#与原生工具对比)
6. [高级配置](#高级配置)
7. [错误处理](#错误处理)
8. [最佳实践](#最佳实践)

## MCP 概述

### 什么是 MCP？

Model Context Protocol (MCP) 是一个开放标准，允许 AI Agent 安全地连接到外部工具和数据源。它提供了标准化的方式来：

- 访问外部 API 和服务
- 执行工具和命令
- 检索上下文信息
- 维护安全的连接和认证

### MCP vs 原生工具

| 特性 | MCP | 原生工具（如 GitHub Tool） |
|------|-----|---------------------------|
| **配置方式** | URL/Preset/Builder | 代码中直接创建 |
| **标准化** | ✅ 统一协议 | ❌ 每个工具不同 |
| **动态发现** | ✅ 自动发现工具 | ❌ 需要手动注册 |
| **远程服务** | ✅ 支持 HTTP/stdio | ❌ 通常仅本地 |
| **配置管理** | ✅ 支持 JSON/YAML | ❌ 硬编码在代码中 |
| **适用场景** | 外部服务、标准化工具 | 特定业务逻辑、自定义工具 |

### 核心优势

1. **标准化接口**：所有 MCP 服务器使用相同的协议
2. **动态工具发现**：Agent 自动发现可用工具，无需手动注册
3. **灵活部署**：支持本地 stdio 和远程 HTTP 服务
4. **配置驱动**：通过配置文件管理，便于环境切换

## 快速开始

### 方式 1: URL 配置（最简单）

```go
package main

import (
    "context"
    "github.com/Ingenimax/agent-sdk-go/pkg/agent"
    "github.com/Ingenimax/agent-sdk-go/pkg/llm/openai"
    "github.com/Ingenimax/agent-sdk-go/pkg/memory"
)

func main() {
    llm := openai.NewClient(os.Getenv("OPENAI_API_KEY"))
    
    // 使用 URL 字符串配置 MCP 服务器
    agent, err := agent.NewAgent(
        agent.WithLLM(llm),
        agent.WithMemory(memory.NewConversationBuffer()),
        
        // 添加 MCP 服务器
        agent.WithMCPURLs(
            "stdio://filesystem/usr/local/bin/mcp-filesystem",
            "stdio://time/usr/local/bin/mcp-time",
            "http://localhost:8080/mcp",
            "https://api.example.com/mcp?token=your-token",
        ),
    )
    
    // 使用 Agent
    response, _ := agent.Run(context.Background(), "列出当前目录的文件")
    fmt.Println(response)
}
```

### 方式 2: 使用预设（推荐）

```go
agent, err := agent.NewAgent(
    agent.WithLLM(llm),
    agent.WithMemory(memory.NewConversationBuffer()),
    
    // 使用预设配置
    agent.WithMCPPresets(
        "filesystem",  // 文件系统操作
        "github",      // GitHub API（需要 GITHUB_TOKEN）
        "time",        // 时间/日期操作
        "fetch",       // HTTP 请求
    ),
)
```

### 方式 3: Builder 模式（最灵活）

```go
import "github.com/Ingenimax/agent-sdk-go/pkg/mcp"

builder := mcp.NewBuilder().
    WithRetry(3, time.Second).           // 重试配置
    WithTimeout(30*time.Second).          // 超时配置
    WithHealthCheck(true).               // 健康检查
    AddStdioServer("filesystem", "/usr/local/bin/mcp-filesystem").
    AddHTTPServerWithAuth("api", "https://api.example.com/mcp", "token").
    AddPreset("github")

servers, lazyConfigs, err := builder.Build(context.Background())

agent, err := agent.NewAgent(
    agent.WithLLM(llm),
    agent.WithMCPServers(servers),
    agent.WithLazyMCPConfigs(lazyConfigs),
)
```

## 配置方式详解

### 1. URL 格式说明

#### Stdio 服务器
```
stdio://server-name/path/to/executable?arg1=value1&arg2=value2
```

示例：
```go
"stdio://filesystem/usr/local/bin/mcp-filesystem"
"stdio://filesystem/usr/local/bin/mcp-filesystem?root=/home/user"
```

#### HTTP 服务器
```
http://host:port/path
https://host:port/path?token=your-token
```

示例：
```go
"http://localhost:8080/mcp"
"https://api.example.com/mcp?token=abc123"
```

#### 预设服务器
```
mcp://preset-name
```

示例：
```go
"mcp://github"  // 使用预定义的 GitHub 配置
```

### 2. 可用预设列表

| 预设名称 | 描述 | 必需环境变量 |
|---------|------|------------|
| `filesystem` | 文件系统操作 | 无 |
| `github` | GitHub API 操作 | `GITHUB_TOKEN` |
| `git` | Git 仓库操作 | 无 |
| `postgres` | PostgreSQL 数据库 | `DATABASE_URL` |
| `slack` | Slack 集成 | `SLACK_BOT_TOKEN`, `SLACK_TEAM_ID` |
| `gdrive` | Google Drive | `GOOGLE_CREDENTIALS` |
| `puppeteer` | Web 自动化 | 无 |
| `memory` | 知识管理 | 无 |
| `fetch` | HTTP 请求 | 无 |
| `brave-search` | Brave 搜索 API | `BRAVE_API_KEY` |
| `time` | 日期/时间操作 | 无 |
| `aws` | AWS 操作 | `AWS_REGION`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` |

### 3. 配置文件方式（JSON/YAML）

#### JSON 配置示例

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "/usr/local/bin/mcp-filesystem",
      "args": ["--root", "/home/user"]
    },
    "github": {
      "command": "/usr/local/bin/mcp-github"
    },
    "api-server": {
      "url": "https://api.example.com/mcp",
      "token": "your-token-here"
    }
  },
  "global": {
    "timeout": "30s",
    "retry_attempts": 3,
    "health_check": true,
    "enable_resources": true,
    "enable_prompts": true,
    "enable_sampling": true,
    "log_level": "info"
  }
}
```

#### YAML 配置示例

```yaml
mcpServers:
  filesystem:
    command: /usr/local/bin/mcp-filesystem
    args:
      - --root
      - /home/user
  github:
    command: /usr/local/bin/mcp-github
  api-server:
    url: https://api.example.com/mcp
    token: your-token-here

global:
  timeout: 30s
  retry_attempts: 3
  health_check: true
  enable_resources: true
  enable_prompts: true
  enable_sampling: true
  log_level: info
```

#### 从配置文件加载

```go
// 从 JSON 文件加载
agent, err := agent.NewAgent(
    agent.WithLLM(llm),
    agent.WithMCPConfigFromJSON("config/mcp.json"),
)

// 从 YAML 文件加载
agent, err := agent.NewAgent(
    agent.WithLLM(llm),
    agent.WithMCPConfigFromYAML("config/mcp.yaml"),
)

// 从配置对象加载
config := &agent.MCPConfiguration{
    MCPServers: map[string]agent.MCPServerConfig{
        "filesystem": {
            Command: "/usr/local/bin/mcp-filesystem",
        },
    },
}
agent, err := agent.NewAgent(
    agent.WithLLM(llm),
    agent.WithMCPConfig(config),
)
```

## 实际使用示例

### 示例 1: 文件系统操作

```go
agent, _ := agent.NewAgent(
    agent.WithLLM(llm),
    agent.WithMCPPresets("filesystem"),
)

// Agent 现在可以读写文件
response, _ := agent.Run(ctx, "创建一个名为 hello.txt 的文件，内容为 'Hello World'")
response, _ := agent.Run(ctx, "列出当前目录的所有 .go 文件")
response, _ := agent.Run(ctx, "读取 README.md 文件的内容")
```

### 示例 2: GitHub 操作

```go
// 设置环境变量
os.Setenv("GITHUB_TOKEN", "your-github-token")

agent, _ := agent.NewAgent(
    agent.WithLLM(llm),
    agent.WithMCPPresets("github"),
)

// Agent 现在可以操作 GitHub
response, _ := agent.Run(ctx, "列出用户 octocat 的所有仓库")
response, _ := agent.Run(ctx, "获取仓库 owner/repo 的 README 内容")
response, _ := agent.Run(ctx, "搜索包含 'terraform' 的仓库")
```

### 示例 3: 数据库查询

```go
os.Setenv("DATABASE_URL", "postgresql://user:pass@localhost/db")

agent, _ := agent.NewAgent(
    agent.WithLLM(llm),
    agent.WithMCPPresets("postgres"),
)

// Agent 现在可以查询数据库
response, _ := agent.Run(ctx, "显示 users 表中的所有用户")
response, _ := agent.Run(ctx, "统计每个部门的员工数量")
```

### 示例 4: 多服务集成

```go
agent, _ := agent.NewAgent(
    agent.WithLLM(llm),
    agent.WithMCPPresets("filesystem", "github", "slack"),
    agent.WithMCPURLs(
        "http://localhost:8080/custom-api",
        "stdio://custom-tool/path/to/tool",
    ),
)

// Agent 现在可以访问多个服务
response, _ := agent.Run(ctx, 
    "读取 README.md 文件，根据最近的提交更新它，并将摘要发布到 Slack")
```

### 示例 5: 与原生工具混合使用

```go
import (
    "github.com/Ingenimax/agent-sdk-go/pkg/tools/github"
    toolsregistry "github.com/Ingenimax/agent-sdk-go/pkg/tools"
)

// 创建原生 GitHub 工具（用于特定业务逻辑）
githubTool, _ := github.NewGitHubContentExtractorTool(token)
toolRegistry := toolsregistry.NewRegistry()
toolRegistry.Register(githubTool)

// 同时使用 MCP 和原生工具
agent, _ := agent.NewAgent(
    agent.WithLLM(llm),
    agent.WithTools(toolRegistry.List()...),  // 原生工具
    agent.WithMCPPresets("filesystem", "time"), // MCP 工具
)
```

## 与原生工具对比

### 原生工具示例（GitHub Extractor）

```go
// 原生工具：需要手动创建和注册
githubTool, err := github.NewGitHubContentExtractorTool(cfg.Tools.GitHub.Token)
if err != nil {
    logger.Error(ctx, "Failed to create GitHub tool", ...)
    return
}
toolRegistry.Register(githubTool)

agent, err := agent.NewAgent(
    agent.WithLLM(openaiClient),
    agent.WithTools(toolRegistry.List()...),  // 手动注册
    agent.WithSystemPrompt(`...`),
)
```

**特点：**
- ✅ 完全控制工具实现
- ✅ 可以添加自定义业务逻辑
- ❌ 需要为每个工具编写代码
- ❌ 工具发现需要手动管理
- ❌ 配置硬编码在代码中

### MCP 工具示例

```go
// MCP 工具：通过配置自动发现
agent, err := agent.NewAgent(
    agent.WithLLM(openaiClient),
    agent.WithMCPPresets("github"),  // 自动发现所有 GitHub 工具
)
```

**特点：**
- ✅ 零代码配置
- ✅ 自动工具发现
- ✅ 标准化接口
- ✅ 支持配置文件
- ❌ 需要 MCP 服务器支持
- ❌ 自定义逻辑有限

### 何时使用哪种方式？

**使用原生工具：**
- 需要特定业务逻辑
- 需要深度定制
- 工具是项目核心功能
- 需要与现有代码深度集成

**使用 MCP：**
- 使用标准化的外部服务
- 需要快速集成多个服务
- 希望配置驱动而非代码驱动
- 需要支持多种环境配置

## 高级配置

### Builder 模式完整示例

```go
import (
    "context"
    "time"
    "github.com/Ingenimax/agent-sdk-go/pkg/mcp"
)

ctx := context.Background()

builder := mcp.NewBuilder().
    // 全局配置
    WithRetry(5, 2*time.Second).        // 重试 5 次，初始延迟 2 秒
    WithTimeout(60*time.Second).        // 超时 60 秒
    WithHealthCheck(true).              // 启用健康检查
    
    // 添加服务器
    AddStdioServer("filesystem", "/usr/local/bin/mcp-filesystem", "--root", "/home/user").
    AddHTTPServer("api", "https://api.example.com/mcp").
    AddHTTPServerWithAuth("secure-api", "https://secure.example.com/mcp", "token-123").
    AddPreset("github").
    AddPreset("time")

// 构建配置
servers, lazyConfigs, err := builder.Build(ctx)
if err != nil {
    log.Fatal(err)
}

// 使用配置
agent, err := agent.NewAgent(
    agent.WithLLM(llm),
    agent.WithMCPServers(servers),           // 立即初始化的服务器
    agent.WithLazyMCPConfigs(lazyConfigs),   // 延迟初始化的配置
)
```

### 延迟初始化（Lazy Initialization）

MCP 服务器默认使用延迟初始化，只在首次使用时启动：

```go
// 服务器不会立即启动
agent, _ := agent.NewAgent(
    agent.WithLLM(llm),
    agent.WithMCPURLs("stdio://slow-server/path/to/server"),
)

// 服务器在第一次调用工具时启动
response, _ := agent.Run(ctx, "使用 slow-server 的工具")
```

### 环境变量配置

```bash
# GitHub
export GITHUB_TOKEN="your-token"

# Database
export DATABASE_URL="postgresql://user:pass@localhost/db"

# AWS
export AWS_REGION="us-east-1"
export AWS_ACCESS_KEY_ID="your-key"
export AWS_SECRET_ACCESS_KEY="your-secret"

# Brave Search
export BRAVE_API_KEY="your-key"

# Slack
export SLACK_BOT_TOKEN="your-token"
export SLACK_TEAM_ID="your-team-id"
```

## 错误处理

### 错误类型

```go
import "github.com/Ingenimax/agent-sdk-go/pkg/mcp"

response, err := agent.Run(ctx, "some query")
if err != nil {
    if mcpErr, ok := err.(*mcp.MCPError); ok {
        fmt.Printf("错误类型: %s\n", mcpErr.ErrorType)
        fmt.Printf("可重试: %t\n", mcpErr.IsRetryable())
        fmt.Printf("友好消息: %s\n", mcp.FormatUserFriendlyError(mcpErr))
        
        // 处理特定错误类型
        switch mcpErr.ErrorType {
        case mcp.MCPErrorTypeConnection:
            fmt.Println("检查网络连接")
        case mcp.MCPErrorTypeAuthentication:
            fmt.Println("检查 API 密钥和凭据")
        case mcp.MCPErrorTypeToolNotFound:
            fmt.Println("请求的工具不可用")
        }
    }
}
```

### 常见错误及解决方案

#### 1. 命令未找到（Stdio 服务器）

**错误**: `server not found` 或 `command not found`

**解决方案**:
```go
// 使用绝对路径
builder.AddStdioServer("tool", "/usr/local/bin/mcp-tool")
```

#### 2. 连接被拒绝（HTTP 服务器）

**错误**: `connection refused`

**解决方案**:
```bash
# 测试连接
curl -v http://localhost:8080/mcp
```

#### 3. 认证失败

**错误**: `authentication failed` 或 `unauthorized`

**解决方案**:
```bash
# 检查环境变量
echo $GITHUB_TOKEN
echo $API_KEY
```

#### 4. 超时问题

**错误**: `timeout` 或 `deadline exceeded`

**解决方案**:
```go
builder := mcp.NewBuilder().
    WithTimeout(60*time.Second).  // 增加超时时间
    WithRetry(3, 5*time.Second)  // 配置重试
```

## 最佳实践

### 1. 环境变量管理

```go
// ✅ 好的做法：从环境变量读取
token := os.Getenv("GITHUB_TOKEN")
if token == "" {
    log.Fatal("GITHUB_TOKEN environment variable is required")
}

// ❌ 不好的做法：硬编码
token := "hardcoded-token-123"
```

### 2. 配置文件管理

```go
// 开发环境
devConfig := "config/mcp.dev.yaml"

// 生产环境
prodConfig := "config/mcp.prod.yaml"

// 根据环境选择配置
configFile := os.Getenv("MCP_CONFIG_FILE")
if configFile == "" {
    configFile = devConfig
}

agent, _ := agent.NewAgent(
    agent.WithLLM(llm),
    agent.WithMCPConfigFromYAML(configFile),
)
```

### 3. 错误处理

```go
agent, err := agent.NewAgent(
    agent.WithLLM(llm),
    agent.WithMCPPresets("github"),
)
if err != nil {
    log.Fatalf("创建 Agent 失败: %v", err)
}

response, err := agent.Run(ctx, query)
if err != nil {
    // 检查是否是 MCP 错误
    if mcpErr, ok := err.(*mcp.MCPError); ok {
        log.Printf("MCP 错误: %s", mcp.FormatUserFriendlyError(mcpErr))
        // 根据错误类型采取不同策略
        if mcpErr.IsRetryable() {
            // 重试逻辑
        }
    } else {
        log.Printf("其他错误: %v", err)
    }
}
```

### 4. 资源清理

```go
// 对于需要清理的资源，确保在完成后关闭
defer func() {
    if agent != nil {
        // Agent 会自动清理 MCP 连接
    }
}()
```

### 5. 日志记录

```go
import "github.com/Ingenimax/agent-sdk-go/pkg/logging"

logger := logging.New().WithLevel("debug")

// MCP 会自动使用 Agent 的 logger
agent, _ := agent.NewAgent(
    agent.WithLLM(llm),
    agent.WithLogger(logger),
    agent.WithMCPPresets("github"),
)
```

### 6. 性能优化

```go
// 禁用健康检查以加快启动速度
builder := mcp.NewBuilder().
    WithHealthCheck(false).
    AddPreset("github")

// 配置适当的超时
builder.WithTimeout(30*time.Second)  // 根据服务响应时间调整
```

## 总结

### 选择建议

1. **快速原型/简单集成** → 使用 `WithMCPPresets()`
2. **生产环境/复杂配置** → 使用配置文件（JSON/YAML）
3. **需要精细控制** → 使用 Builder 模式
4. **混合场景** → MCP + 原生工具

### 关键要点

- ✅ MCP 提供标准化、配置驱动的工具集成方式
- ✅ 支持延迟初始化，提高启动性能
- ✅ 自动工具发现，减少手动管理
- ✅ 支持多种部署方式（stdio/HTTP）
- ✅ 完善的错误处理和重试机制

### 下一步

- 查看 [MCP 官方文档](https://modelcontextprotocol.io)
- 探索 [MCP 服务器列表](https://github.com/modelcontextprotocol)
- 阅读 [MCP 规范](https://spec.modelcontextprotocol.io/)

