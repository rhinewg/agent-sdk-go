package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Ingenimax/agent-sdk-go/pkg/executionplan"
	"github.com/Ingenimax/agent-sdk-go/pkg/interfaces"
)

// generateExecutionPlanTool is a built-in tool that lets the LLM generate an execution plan for a task.
// Plans are stored in the session-scoped store when available so they are isolated per user/conversation.
type generateExecutionPlanTool struct{ agent *Agent }

// NewGenerateExecutionPlanTool returns a tool that generates an execution plan when executed.
func NewGenerateExecutionPlanTool(a *Agent) interfaces.Tool {
	return &generateExecutionPlanTool{agent: a}
}

func (t *generateExecutionPlanTool) Name() string { return "generate_execution_plan" }

func (t *generateExecutionPlanTool) Description() string {
	return "Generate a step-by-step execution plan for a task using available tools. Use when the user asks for a multi-step plan before execution. Returns the plan (with task_id) for approval; the user or you can then call execute_execution_plan with that task_id to run it."
}

func (t *generateExecutionPlanTool) Parameters() map[string]interfaces.ParameterSpec {
	return map[string]interfaces.ParameterSpec{
		"task": {
			Type:        "string",
			Description: "Description of the task or user request to plan for",
			Required:    true,
		},
	}
}

func (t *generateExecutionPlanTool) Run(ctx context.Context, input string) (string, error) {
	return t.Execute(ctx, input)
}

func (t *generateExecutionPlanTool) Execute(ctx context.Context, args string) (string, error) {
	task := parsePlanTaskArg(args)
	if task == "" {
		return "", fmt.Errorf("task is required")
	}
	if t.agent.planGenerator == nil {
		return "", fmt.Errorf("execution plan generator not available")
	}

	// Use session-scoped tools and prompt when available (same as runWithExecutionPlan path).
	effectiveTools := t.agent.getEffectiveTools(ctx)
	effectivePrompt := t.agent.getEffectiveSystemPrompt(ctx)
	gen := executionplan.NewGenerator(t.agent.llm, effectiveTools, effectivePrompt, t.agent.requirePlanApproval)

	plan, err := gen.GenerateExecutionPlan(ctx, task)
	if err != nil {
		return "", fmt.Errorf("failed to generate execution plan: %w", err)
	}

	// Store in session-scoped store first (session isolation), then global store for compatibility.
	if t.agent.planSessionStore != nil {
		t.agent.planSessionStore.StorePlan(ctx, plan)
	}
	if t.agent.planStore != nil {
		t.agent.planStore.StorePlan(plan)
	}

	formatted := executionplan.FormatExecutionPlan(plan)
	return fmt.Sprintf("Execution plan created (task_id: %s). You can present this to the user for approval, then call execute_execution_plan with task_id=%s to run it.\n\n%s",
		plan.TaskID, plan.TaskID, formatted), nil
}

// executeExecutionPlanTool is a built-in tool that lets the LLM approve and execute a stored plan.
// Plan lookup uses the session-scoped store when available so only plans from the current conversation are visible.
type executeExecutionPlanTool struct{ agent *Agent }

// NewExecuteExecutionPlanTool returns a tool that executes an approved plan when executed.
func NewExecuteExecutionPlanTool(a *Agent) interfaces.Tool {
	return &executeExecutionPlanTool{agent: a}
}

func (t *executeExecutionPlanTool) Name() string { return "execute_execution_plan" }

func (t *executeExecutionPlanTool) Description() string {
	return "Execute an existing execution plan by task_id. No user approval required—calling this runs the plan immediately. Pass task_id from generate_execution_plan, or use task_id 'latest' to run the most recent pending plan in this conversation."
}

func (t *executeExecutionPlanTool) Parameters() map[string]interfaces.ParameterSpec {
	return map[string]interfaces.ParameterSpec{
		"task_id": {
			Type:        "string",
			Description: "Task ID of the plan to execute (from generate_execution_plan), or 'latest' to run the most recent pending plan",
			Required:    true,
		},
	}
}

func (t *executeExecutionPlanTool) Run(ctx context.Context, input string) (string, error) {
	return t.Execute(ctx, input)
}

func (t *executeExecutionPlanTool) Execute(ctx context.Context, args string) (string, error) {
	taskID := parsePlanTaskIDArg(args)
	if taskID == "" {
		return "", fmt.Errorf("task_id is required")
	}
	if t.agent.planExecutor == nil {
		return "", fmt.Errorf("execution plan executor not available")
	}

	plan, err := t.agent.getPlanForExecution(ctx, taskID)
	if err != nil {
		return "", err
	}
	if plan == nil {
		return "", fmt.Errorf("plan with task_id %q not found in this session", taskID)
	}

	// Tool path: execute directly without requiring user approval
	return t.agent.executePlanDirect(ctx, plan)
}

// getExecutionPlanStatusTool returns the status and summary of a plan by task_id.
type getExecutionPlanStatusTool struct{ agent *Agent }

// NewGetExecutionPlanStatusTool returns a tool that queries plan status by task_id.
func NewGetExecutionPlanStatusTool(a *Agent) interfaces.Tool {
	return &getExecutionPlanStatusTool{agent: a}
}

func (t *getExecutionPlanStatusTool) Name() string { return "get_execution_plan_status" }

func (t *getExecutionPlanStatusTool) Description() string {
	return "Get the status and summary of an execution plan by task_id. Use when the user asks for the status of a plan or whether a plan is pending/completed. Returns status, description, and timestamps."
}

func (t *getExecutionPlanStatusTool) Parameters() map[string]interfaces.ParameterSpec {
	return map[string]interfaces.ParameterSpec{
		"task_id": {
			Type:        "string",
			Description: "Task ID of the plan (from generate_execution_plan)",
			Required:    true,
		},
	}
}

func (t *getExecutionPlanStatusTool) Run(ctx context.Context, input string) (string, error) {
	return t.Execute(ctx, input)
}

func (t *getExecutionPlanStatusTool) Execute(ctx context.Context, args string) (string, error) {
	taskID := parsePlanTaskIDArg(args)
	if taskID == "" {
		return "", fmt.Errorf("task_id is required")
	}
	plan := t.agent.getPlanByTaskID(ctx, taskID)
	if plan == nil {
		return "", fmt.Errorf("plan with task_id %q not found in this session", taskID)
	}
	return formatPlanStatus(plan), nil
}

// listExecutionPlansTool lists execution plans in the current session.
type listExecutionPlansTool struct{ agent *Agent }

// NewListExecutionPlansTool returns a tool that lists plans in the current session.
func NewListExecutionPlansTool(a *Agent) interfaces.Tool {
	return &listExecutionPlansTool{agent: a}
}

func (t *listExecutionPlansTool) Name() string { return "list_execution_plans" }

func (t *listExecutionPlansTool) Description() string {
	return "List all execution plans in the current conversation. Optionally filter by status (e.g. pending_approval, completed, all). Use when the user asks what plans exist, which are pending, or to show plan list."
}

func (t *listExecutionPlansTool) Parameters() map[string]interfaces.ParameterSpec {
	return map[string]interfaces.ParameterSpec{
		"status_filter": {
			Type:        "string",
			Description: "Optional: filter by status—pending_approval, approved, executing, completed, failed, cancelled, or 'all' (default)",
			Required:    false,
		},
	}
}

func (t *listExecutionPlansTool) Run(ctx context.Context, input string) (string, error) {
	return t.Execute(ctx, input)
}

func (t *listExecutionPlansTool) Execute(ctx context.Context, args string) (string, error) {
	filter := parsePlanStatusFilterArg(args)
	plans := t.agent.listPlansInSession(ctx)
	if len(plans) == 0 {
		return "No execution plans in this conversation.", nil
	}
	var out []string
	for _, p := range plans {
		if filter != "" && filter != "all" && string(p.Status) != filter {
			continue
		}
		out = append(out, fmt.Sprintf("- task_id: %s | status: %s | %s", p.TaskID, p.Status, truncateDesc(p.Description, 60)))
	}
	if len(out) == 0 {
		return fmt.Sprintf("No plans matching status %q. Total plans in session: %d.", filter, len(plans)), nil
	}
	return "Execution plans in this conversation:\n" + strings.Join(out, "\n"), nil
}

// deleteExecutionPlanTool deletes an execution plan by task_id.
type deleteExecutionPlanTool struct{ agent *Agent }

// NewDeleteExecutionPlanTool returns a tool that deletes a plan by task_id.
func NewDeleteExecutionPlanTool(a *Agent) interfaces.Tool {
	return &deleteExecutionPlanTool{agent: a}
}

func (t *deleteExecutionPlanTool) Name() string { return "delete_execution_plan" }

func (t *deleteExecutionPlanTool) Description() string {
	return "Delete an execution plan by task_id. Use when the user wants to remove or discard a plan. The plan is removed from the current conversation."
}

func (t *deleteExecutionPlanTool) Parameters() map[string]interfaces.ParameterSpec {
	return map[string]interfaces.ParameterSpec{
		"task_id": {
			Type:        "string",
			Description: "Task ID of the plan to delete (from generate_execution_plan or list_execution_plans)",
			Required:    true,
		},
	}
}

func (t *deleteExecutionPlanTool) Run(ctx context.Context, input string) (string, error) {
	return t.Execute(ctx, input)
}

func (t *deleteExecutionPlanTool) Execute(ctx context.Context, args string) (string, error) {
	taskID := parsePlanTaskIDArg(args)
	if taskID == "" {
		return "", fmt.Errorf("task_id is required")
	}
	deleted := t.agent.deletePlan(ctx, taskID)
	if !deleted {
		return "", fmt.Errorf("plan with task_id %q not found in this session", taskID)
	}
	return fmt.Sprintf("Execution plan %q has been deleted.", taskID), nil
}

// deletePlan removes a plan by task_id from session store and global store. Returns true if found and removed.
func (a *Agent) deletePlan(ctx context.Context, taskID string) bool {
	var ok bool
	if a.planSessionStore != nil {
		ok = a.planSessionStore.DeletePlan(ctx, taskID)
	}
	if a.planStore != nil {
		if a.planStore.DeletePlan(taskID) {
			ok = true
		}
	}
	return ok
}

func formatPlanStatus(plan *executionplan.ExecutionPlan) string {
	return fmt.Sprintf("Task ID: %s\nStatus: %s\nDescription: %s\nCreated: %s\nUpdated: %s",
		plan.TaskID, plan.Status, plan.Description,
		plan.CreatedAt.Format("2006-01-02 15:04:05"), plan.UpdatedAt.Format("2006-01-02 15:04:05"))
}

func truncateDesc(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func parsePlanStatusFilterArg(args string) string {
	s := strings.TrimSpace(args)
	if s == "" {
		return "all"
	}
	var v struct {
		StatusFilter string `json:"status_filter"`
	}
	if err := json.Unmarshal([]byte(args), &v); err == nil && v.StatusFilter != "" {
		return strings.TrimSpace(strings.ToLower(v.StatusFilter))
	}
	return strings.ToLower(s)
}

// getPlanByTaskID returns a plan by task_id from session or global store (read-only, any status).
func (a *Agent) getPlanByTaskID(ctx context.Context, taskID string) *executionplan.ExecutionPlan {
	if taskID == "" {
		return nil
	}
	if a.planSessionStore != nil {
		if p, ok := a.planSessionStore.GetPlanByTaskID(ctx, taskID); ok {
			return p
		}
	}
	if a.planStore != nil {
		if p, ok := a.planStore.GetPlanByTaskID(taskID); ok {
			return p
		}
	}
	return nil
}

// listPlansInSession returns all plans in the current session (session store first, then global if no session).
func (a *Agent) listPlansInSession(ctx context.Context) []*executionplan.ExecutionPlan {
	if a.planSessionStore != nil {
		return a.planSessionStore.ListPlans(ctx)
	}
	if a.planStore != nil {
		return a.planStore.ListPlans()
	}
	return nil
}

// statusRunnableForLatest is the set of plan statuses that "latest" should consider (plans that can be executed).
// When require_plan_approval is false the generator sets StatusApproved, so we must include it.
var statusRunnableForLatest = map[executionplan.ExecutionPlanStatus]bool{
	executionplan.StatusPendingApproval: true,
	executionplan.StatusApproved:        true,
}

// getPlanForExecution resolves a plan by task_id with session isolation.
// If taskID is "latest", returns the most recent runnable plan (pending_approval or approved) in the current session.
func (a *Agent) getPlanForExecution(ctx context.Context, taskID string) (*executionplan.ExecutionPlan, error) {
	if a.planSessionStore != nil {
		if taskID == "latest" {
			plans := a.planSessionStore.ListPlans(ctx)
			var latest *executionplan.ExecutionPlan
			for _, p := range plans {
				if !statusRunnableForLatest[p.Status] {
					continue
				}
				if latest == nil || p.UpdatedAt.After(latest.UpdatedAt) {
					latest = p
				}
			}
			return latest, nil
		}
		if p, ok := a.planSessionStore.GetPlanByTaskID(ctx, taskID); ok {
			return p, nil
		}
	}
	if a.planStore != nil {
		if taskID == "latest" {
			all := a.planStore.ListPlans()
			var latest *executionplan.ExecutionPlan
			for _, p := range all {
				if !statusRunnableForLatest[p.Status] {
					continue
				}
				if latest == nil || p.UpdatedAt.After(latest.UpdatedAt) {
					latest = p
				}
			}
			return latest, nil
		}
		if p, ok := a.planStore.GetPlanByTaskID(taskID); ok {
			return p, nil
		}
	}
	return nil, nil
}

func parsePlanTaskArg(args string) string {
	s := strings.TrimSpace(args)
	if s == "" {
		return ""
	}
	var v struct {
		Task string `json:"task"`
	}
	if err := json.Unmarshal([]byte(args), &v); err == nil && v.Task != "" {
		return strings.TrimSpace(v.Task)
	}
	return s
}

func parsePlanTaskIDArg(args string) string {
	s := strings.TrimSpace(args)
	if s == "" {
		return ""
	}
	var v struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal([]byte(args), &v); err == nil && v.TaskID != "" {
		return strings.TrimSpace(v.TaskID)
	}
	return s
}

// DefaultPlanTools returns the built-in tools for execution plan generation, execution, status query, listing, and deletion.
// They are injected in getEffectiveTools() when planStore is in use (independent of skill configuration).
// Plans are session-isolated when WithExecutionPlanSessionStore is set.
func DefaultPlanTools(a *Agent) []interfaces.Tool {
	return []interfaces.Tool{
		NewGenerateExecutionPlanTool(a),
		NewExecuteExecutionPlanTool(a),
		NewGetExecutionPlanStatusTool(a),
		NewListExecutionPlansTool(a),
		NewDeleteExecutionPlanTool(a),
	}
}
