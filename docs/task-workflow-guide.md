# Task 与 Workflow 使用教程

本文档详细说明如何在 agent-sdk-go 中使用 Task（任务）和 Workflow（工作流）来构建复杂的任务执行系统。

## 目录

1. [概述](#概述)
2. [Task（任务）使用指南](#task任务使用指南)
3. [Workflow（工作流）使用指南](#workflow工作流使用指南)
4. [Task 与 Workflow 的关系](#task-与-workflow-的关系)
5. [实际应用场景](#实际应用场景)
6. [最佳实践](#最佳实践)

## 概述

### Task vs Workflow

| 特性 | Task（任务） | Workflow（工作流） |
|------|-------------|-------------------|
| **定义** | 单个可执行的操作单元 | 多个任务的编排和执行流程 |
| **执行方式** | 同步或异步 | 按依赖关系顺序或并行执行 |
| **依赖管理** | 无 | 支持任务依赖关系 |
| **适用场景** | 简单操作、API调用、函数执行 | 复杂业务流程、多步骤任务 |
| **状态跟踪** | 基本状态 | 完整的状态跟踪和结果管理 |

### 核心概念

- **Task（任务）**：最小的执行单元，可以是函数、API调用或工作流
- **Workflow（工作流）**：由多个任务组成，支持依赖关系和并行执行
- **TaskExecutor（任务执行器）**：管理和执行任务的组件
- **Orchestrator（编排器）**：管理和执行工作流的组件

## Task（任务）使用指南

### 1. 基本任务执行

#### 创建任务执行器

```go
import "github.com/Ingenimax/agent-sdk-go/pkg/agentsdk"

// 创建任务执行器
executor := agentsdk.NewTaskExecutor()
```

#### 注册简单任务

```go
// 注册一个简单的任务函数
executor.RegisterTask("hello", func(ctx context.Context, params interface{}) (interface{}, error) {
    name, ok := params.(string)
    if !ok {
        name = "World"
    }
    return fmt.Sprintf("Hello, %s!", name), nil
})
```

#### 同步执行任务

```go
// 同步执行任务（阻塞直到完成）
result, err := executor.ExecuteSync(context.Background(), "hello", "John", nil)
if err != nil {
    fmt.Printf("Error: %v\n", err)
} else {
    fmt.Printf("Result: %v\n", result.Data)
}
```

#### 异步执行任务

```go
// 异步执行任务（立即返回，通过 channel 获取结果）
resultChan, err := executor.ExecuteAsync(context.Background(), "hello", "Jane", nil)
if err != nil {
    fmt.Printf("Error: %v\n", err)
} else {
    // 等待结果
    result := <-resultChan
    fmt.Printf("Result: %v\n", result.Data)
}
```

### 2. API 任务执行

#### 创建 API 客户端

```go
import "github.com/Ingenimax/agent-sdk-go/pkg/task"

// 创建 API 客户端
apiClient := agentsdk.NewAPIClient("https://api.example.com", 10*time.Second)
```

#### 注册 API 任务

```go
// 注册一个 API 任务
executor.RegisterTask("get_data", agentsdk.APITask(apiClient, task.APIRequest{
    Method: "GET",
    Path:   "/data",
    Query:  map[string]string{"limit": "10"},
    Headers: map[string]string{
        "Authorization": "Bearer token",
    },
}))
```

#### 执行 API 任务（带重试策略）

```go
// 配置超时和重试策略
timeout := 5 * time.Second
retryPolicy := &interfaces.RetryPolicy{
    MaxRetries:        3,                    // 最大重试次数
    InitialBackoff:    100 * time.Millisecond, // 初始退避时间
    MaxBackoff:        1 * time.Second,      // 最大退避时间
    BackoffMultiplier: 2.0,                  // 退避倍数（指数退避）
}

// 执行任务
result, err := executor.ExecuteSync(context.Background(), "get_data", nil, &interfaces.TaskOptions{
    Timeout:     &timeout,
    RetryPolicy: retryPolicy,
})
```

### 3. Temporal 工作流任务

#### 创建 Temporal 客户端

```go
// 创建 Temporal 客户端配置
temporalClient := agentsdk.NewTemporalClient(task.TemporalConfig{
    HostPort:                 "localhost:7233",  // Temporal 服务器地址
    Namespace:                "default",         // 命名空间
    TaskQueue:                "example",        // 任务队列
    WorkflowIDPrefix:         "example-",       // 工作流 ID 前缀
    WorkflowExecutionTimeout: 10 * time.Minute, // 执行超时
    WorkflowRunTimeout:       5 * time.Minute,   // 运行超时
    WorkflowTaskTimeout:      10 * time.Second, // 任务超时
})
```

#### 注册 Temporal 工作流任务

```go
// 注册 Temporal 工作流任务
executor.RegisterTask("example_workflow", agentsdk.TemporalWorkflowTask(
    temporalClient, 
    "ExampleWorkflow", // 工作流名称
))
```

#### 执行 Temporal 工作流任务

```go
// 执行 Temporal 工作流任务
result, err := executor.ExecuteSync(context.Background(), "example_workflow", map[string]interface{}{
    "input": "example input",
    "param1": "value1",
}, nil)
```

### 4. 任务选项配置

```go
// 完整的任务选项配置
timeout := 30 * time.Second
options := &interfaces.TaskOptions{
    // 超时设置
    Timeout: &timeout,
    
    // 重试策略
    RetryPolicy: &interfaces.RetryPolicy{
        MaxRetries:        5,
        InitialBackoff:    200 * time.Millisecond,
        MaxBackoff:        2 * time.Second,
        BackoffMultiplier: 2.0,
    },
    
    // 元数据（用于传递额外信息）
    Metadata: map[string]interface{}{
        "purpose":     "data_processing",
        "priority":    "high",
        "user_id":     "user123",
        "request_id":  "req-456",
    },
}
```

### 5. 任务结果处理

```go
// TaskResult 结构
type TaskResult struct {
    // Data 包含任务执行的结果数据
    Data interface{}
    
    // Error 包含执行过程中的错误
    Error error
    
    // Metadata 包含任务执行的元数据信息
    Metadata map[string]interface{}
}

// 处理任务结果
result, err := executor.ExecuteSync(ctx, "task_name", params, options)
if err != nil {
    // 处理执行错误
    log.Printf("Task execution failed: %v", err)
} else if result.Error != nil {
    // 处理任务返回的错误
    log.Printf("Task returned error: %v", result.Error)
} else {
    // 处理成功的结果
    fmt.Printf("Task result: %v\n", result.Data)
    
    // 访问元数据
    if metadata, ok := result.Metadata["execution_time"].(time.Duration); ok {
        fmt.Printf("Execution time: %v\n", metadata)
    }
}
```

## Workflow（工作流）使用指南

### 1. 基本工作流创建

#### 使用 workflow 包

```go
import "github.com/Ingenimax/agent-sdk-go/pkg/workflow"

// 创建新工作流
wf := workflow.New("my_workflow")

// 创建 Agent
agent1 := workflow.NewAgent("agent1", "You are a data analyst")
agent2 := workflow.NewAgent("agent2", "You are a report writer")

// 添加步骤
wf.AddStep(workflow.Step{
    Agent:               agent1,
    Name:                "analyze_data",
    Description:         "Analyze the input data",
    Input:               "Analyze this data: {{input}}",
    NextStep:            "generate_report",
    HandoffInstructions: "Pass the analysis results to the report generator",
})

wf.AddStep(workflow.Step{
    Agent:               agent2,
    Name:                "generate_report",
    Description:         "Generate a report based on analysis",
    Input:               "Generate a report based on: {{analyze_data.output}}",
    // NextStep 为空表示这是最后一步
})
```

### 2. 使用 Orchestration 包（推荐）

#### 创建编排器

```go
import "github.com/Ingenimax/agent-sdk-go/pkg/orchestration"

// 创建 Agent 注册表
registry := orchestration.NewAgentRegistry()

// 注册 Agent
registry.Register("data_analyst", dataAnalystAgent)
registry.Register("report_writer", reportWriterAgent)

// 创建代码编排器
orchestrator := orchestration.NewCodeOrchestrator(registry)
```

#### 创建工作流

```go
// 创建新工作流
workflow := orchestration.NewWorkflow()

// 添加任务（无依赖）
workflow.AddTask(
    "analyze_data",           // 任务 ID
    "data_analyst",           // Agent ID
    "Analyze the sales data", // 输入
    []string{},               // 依赖（空表示无依赖）
)

// 添加依赖任务
workflow.AddTask(
    "generate_report",        // 任务 ID
    "report_writer",          // Agent ID
    "Generate report",        // 输入（会被依赖结果增强）
    []string{"analyze_data"}, // 依赖：需要 analyze_data 完成
)

// 设置最终任务
workflow.SetFinalTask("generate_report")
```

#### 执行工作流

```go
// 执行工作流
result, err := orchestrator.ExecuteWorkflow(context.Background(), workflow)
if err != nil {
    log.Printf("Workflow execution failed: %v", err)
} else {
    fmt.Printf("Final result: %s\n", result)
    
    // 访问所有任务结果
    for taskID, taskResult := range workflow.Results {
        fmt.Printf("Task %s: %s\n", taskID, taskResult)
    }
}
```

### 3. 复杂工作流示例

#### 并行任务执行

```go
workflow := orchestration.NewWorkflow()

// 任务 1 和 2 可以并行执行（无依赖）
workflow.AddTask("fetch_user_data", "data_fetcher", "Fetch user data", []string{})
workflow.AddTask("fetch_product_data", "data_fetcher", "Fetch product data", []string{})

// 任务 3 需要等待任务 1 和 2 完成
workflow.AddTask(
    "combine_data",
    "data_processor",
    "Combine user and product data",
    []string{"fetch_user_data", "fetch_product_data"},
)

// 任务 4 需要等待任务 3 完成
workflow.AddTask(
    "generate_insights",
    "analyst",
    "Generate insights from combined data",
    []string{"combine_data"},
)

workflow.SetFinalTask("generate_insights")
```

#### 条件执行（通过 Agent 决策）

```go
workflow := orchestration.NewWorkflow()

// 第一步：分析需求
workflow.AddTask("analyze_requirement", "analyzer", "Analyze the requirement", []string{})

// 第二步：根据分析结果决定执行路径（由 Agent 决定）
workflow.AddTask(
    "decide_path",
    "decision_maker",
    "Based on the analysis, decide the execution path",
    []string{"analyze_requirement"},
)

// 第三步：执行选定的路径（Agent 会根据决策选择）
workflow.AddTask(
    "execute_path_a",
    "executor_a",
    "Execute path A",
    []string{"decide_path"},
)

workflow.AddTask(
    "execute_path_b",
    "executor_b",
    "Execute path B",
    []string{"decide_path"},
)
```

### 4. 工作流状态跟踪

```go
// 工作流执行过程中的状态
type TaskStatus string

const (
    TaskPending   TaskStatus = "pending"   // 等待执行
    TaskRunning   TaskStatus = "running"   // 正在执行
    TaskCompleted TaskStatus = "completed" // 已完成
    TaskFailed    TaskStatus = "failed"    // 执行失败
)

// 检查任务状态
for _, task := range workflow.Tasks {
    fmt.Printf("Task %s: %s\n", task.ID, task.Status)
    
    if task.Status == TaskFailed {
        fmt.Printf("  Error: %v\n", task.Error)
    } else if task.Status == TaskCompleted {
        fmt.Printf("  Result: %s\n", task.Result)
    }
}
```

## Task 与 Workflow 的关系

### 关系说明

1. **Workflow 由多个 Task 组成**
   - 每个 Workflow 中的步骤对应一个 Task
   - Task 是 Workflow 的基本执行单元

2. **Task 可以独立使用**
   - Task 可以单独执行，不依赖 Workflow
   - 适合简单的、一次性的操作

3. **Workflow 管理 Task 的执行顺序**
   - 通过依赖关系控制 Task 的执行顺序
   - 支持并行执行无依赖的 Task

### 使用场景对比

**使用 Task：**
- ✅ 简单的 API 调用
- ✅ 单一函数执行
- ✅ 不需要复杂编排的场景
- ✅ 异步任务处理

**使用 Workflow：**
- ✅ 多步骤业务流程
- ✅ 需要任务依赖管理
- ✅ 需要结果传递和状态跟踪
- ✅ 复杂的 Agent 协作场景

## 实际应用场景

### 场景 1: 数据处理流水线

```go
// 创建数据处理工作流
workflow := orchestration.NewWorkflow()

// 步骤 1: 数据提取
workflow.AddTask("extract", "extractor", "Extract data from source", []string{})

// 步骤 2: 数据清洗（依赖提取）
workflow.AddTask("clean", "cleaner", "Clean the extracted data", []string{"extract"})

// 步骤 3: 数据转换（依赖清洗）
workflow.AddTask("transform", "transformer", "Transform cleaned data", []string{"clean"})

// 步骤 4: 数据加载（依赖转换）
workflow.AddTask("load", "loader", "Load transformed data", []string{"transform"})

workflow.SetFinalTask("load")

// 执行工作流
result, err := orchestrator.ExecuteWorkflow(ctx, workflow)
```

### 场景 2: 多 Agent 协作分析

```go
workflow := orchestration.NewWorkflow()

// 研究 Agent 收集信息
workflow.AddTask("research", "research_agent", "Research the topic", []string{})

// 分析 Agent 分析信息（依赖研究）
workflow.AddTask("analyze", "analysis_agent", "Analyze research findings", []string{"research"})

// 写作 Agent 生成报告（依赖分析）
workflow.AddTask("write", "writing_agent", "Write analysis report", []string{"analyze"})

// 审核 Agent 审核报告（依赖写作）
workflow.AddTask("review", "review_agent", "Review the report", []string{"write"})

workflow.SetFinalTask("review")
```

### 场景 3: 混合使用 Task 和 Workflow

```go
// 使用 Task 执行简单的 API 调用
executor := agentsdk.NewTaskExecutor()
executor.RegisterTask("fetch_config", agentsdk.APITask(apiClient, task.APIRequest{
    Method: "GET",
    Path:   "/config",
}))

// 获取配置
configResult, _ := executor.ExecuteSync(ctx, "fetch_config", nil, nil)
config := configResult.Data.(map[string]interface{})

// 使用 Workflow 处理复杂流程
workflow := orchestration.NewWorkflow()
workflow.AddTask("process", "processor", fmt.Sprintf("Process with config: %v", config), []string{})

// 执行工作流
result, _ := orchestrator.ExecuteWorkflow(ctx, workflow)
```

## 最佳实践

### 1. 任务设计原则

```go
// ✅ 好的做法：任务职责单一
executor.RegisterTask("calculate_total", func(ctx context.Context, params interface{}) (interface{}, error) {
    // 只做一件事：计算总和
    numbers := params.([]float64)
    total := 0.0
    for _, n := range numbers {
        total += n
    }
    return total, nil
})

// ❌ 不好的做法：任务职责过多
executor.RegisterTask("do_everything", func(ctx context.Context, params interface{}) (interface{}, error) {
    // 做了太多事情，难以测试和维护
    // ...
})
```

### 2. 错误处理

```go
// ✅ 好的做法：详细的错误处理
executor.RegisterTask("safe_operation", func(ctx context.Context, params interface{}) (interface{}, error) {
    // 验证输入
    if params == nil {
        return nil, fmt.Errorf("params cannot be nil")
    }
    
    // 执行操作
    result, err := performOperation(params)
    if err != nil {
        // 返回详细的错误信息
        return nil, fmt.Errorf("operation failed: %w", err)
    }
    
    return result, nil
})
```

### 3. 超时和重试配置

```go
// ✅ 好的做法：根据任务类型配置超时
var timeout time.Duration
switch taskType {
case "api_call":
    timeout = 10 * time.Second
case "heavy_computation":
    timeout = 5 * time.Minute
default:
    timeout = 30 * time.Second
}

// ✅ 好的做法：配置合理的重试策略
retryPolicy := &interfaces.RetryPolicy{
    MaxRetries:        3,                    // 网络请求可以重试
    InitialBackoff:    100 * time.Millisecond,
    MaxBackoff:        1 * time.Second,
    BackoffMultiplier: 2.0,
}
```

### 4. 工作流设计

```go
// ✅ 好的做法：清晰的任务依赖
workflow := orchestration.NewWorkflow()

// 明确的任务依赖关系
workflow.AddTask("step1", "agent1", "Input", []string{})
workflow.AddTask("step2", "agent2", "Input", []string{"step1"})
workflow.AddTask("step3", "agent3", "Input", []string{"step2"})

// ✅ 好的做法：设置最终任务
workflow.SetFinalTask("step3")

// ❌ 不好的做法：循环依赖
// workflow.AddTask("a", "agent", "Input", []string{"b"})
// workflow.AddTask("b", "agent", "Input", []string{"a"}) // 错误！
```

### 5. 结果传递

```go
// ✅ 好的做法：在工作流中传递结果
workflow.AddTask("analyze", "analyzer", "Analyze data", []string{})

// 下一个任务会自动接收前一个任务的结果
workflow.AddTask("report", "reporter", "Generate report", []string{"analyze"})
// report 任务的输入会自动包含 analyze 任务的结果
```

### 6. 并发控制

```go
// ✅ 好的做法：利用并行执行提高效率
workflow := orchestration.NewWorkflow()

// 这些任务可以并行执行（无依赖）
workflow.AddTask("fetch_a", "fetcher", "Fetch A", []string{})
workflow.AddTask("fetch_b", "fetcher", "Fetch B", []string{})
workflow.AddTask("fetch_c", "fetcher", "Fetch C", []string{})

// 这个任务等待所有并行任务完成
workflow.AddTask("combine", "combiner", "Combine results", 
    []string{"fetch_a", "fetch_b", "fetch_c"})
```

### 7. 资源清理

```go
// ✅ 好的做法：确保资源清理
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

result, err := orchestrator.ExecuteWorkflow(ctx, workflow)
if err != nil {
    // 处理错误
    return err
}

// 清理资源
defer func() {
    // 清理工作流相关资源
    workflow.Results = nil
    workflow.Errors = nil
}()
```

## 总结

### 选择指南

1. **使用 Task 当：**
   - 需要执行简单的、独立的任务
   - 需要异步执行
   - 需要 API 调用或函数执行
   - 不需要复杂的依赖管理

2. **使用 Workflow 当：**
   - 需要多步骤业务流程
   - 需要任务依赖管理
   - 需要结果传递和状态跟踪
   - 需要多个 Agent 协作

### 关键要点

- ✅ Task 是基本的执行单元，可以独立使用
- ✅ Workflow 是任务的编排系统，管理任务执行顺序
- ✅ 支持同步和异步执行
- ✅ 内置重试机制和超时控制
- ✅ 支持任务依赖和并行执行
- ✅ 完整的状态跟踪和错误处理

### 下一步

- 查看 [Task 示例](../examples/task/)
- 查看 [Workflow 示例](../examples/orchestration/)
- 阅读 [Agent 文档](./agent.md) 了解 Agent 的使用
- 探索 [Execution Plan](./execution_plan.md) 了解执行计划

