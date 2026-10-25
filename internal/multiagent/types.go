// Package multiagent adapts the native WeKnora AgentEngine for supervised
// multi-step execution. P1 scope is a single synchronous agent step: resolve
// a tenant-scoped chat model through the existing ModelService, drive one
// stateless AgentEngine turn through the existing AgentService, and return a
// sanitized StepResult. No task tables, queues, supervisors, or HTTP surface.
package multiagent

import (
	"time"

	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
)

// StepStatus is the terminal outcome of one step.
type StepStatus string

const (
	// StepSucceeded means the engine completed and produced a final answer.
	StepSucceeded StepStatus = "SUCCEEDED"
	// StepFailed means execution started but did not produce a usable answer
	// (model runtime error, timeout, cancellation, engine error, or no answer).
	StepFailed StepStatus = "FAILED"
	// StepBlocked is reserved for preconditions checkable before execution
	// (identity, spec, model availability). It is never used for runtime
	// failures of an executing step.
	StepBlocked StepStatus = "BLOCKED"
)

// ErrorCode is the structured classification carried on every non-successful
// StepResult. Precondition codes pair with BLOCKED; execution codes with FAILED.
type ErrorCode string

const (
	// Precondition failures -> BLOCKED.
	ErrTenantContextMissing ErrorCode = "TENANT_CONTEXT_MISSING"
	ErrTenantMismatch       ErrorCode = "TENANT_MISMATCH"
	ErrInvalidSpec          ErrorCode = "INVALID_SPEC"
	ErrModelNotConfigured   ErrorCode = "MODEL_NOT_CONFIGURED"
	ErrModelUnavailable     ErrorCode = "MODEL_UNAVAILABLE"

	// Execution failures -> FAILED.
	ErrEngineCreate  ErrorCode = "ENGINE_CREATE_FAILED"
	ErrEngineExecute ErrorCode = "ENGINE_EXECUTE_FAILED"
	ErrStepTimeout   ErrorCode = "STEP_TIMEOUT"
	ErrStepCanceled  ErrorCode = "STEP_CANCELED"
	ErrNoFinalAnswer ErrorCode = "NO_FINAL_ANSWER"
)

// IsPrecondition reports whether the code describes a precondition (BLOCKED
// class) rather than an execution failure (FAILED class).
func (c ErrorCode) IsPrecondition() bool {
	switch c {
	case ErrTenantContextMissing, ErrTenantMismatch, ErrInvalidSpec,
		ErrModelNotConfigured, ErrModelUnavailable:
		return true
	}
	return false
}

// StepSpec is the input contract for one agent step.
type StepSpec struct {
	// StepID identifies the step in traces and logs; it is prefixed into the
	// engine session/message IDs. Empty defaults to a stable placeholder.
	StepID string
	// TenantID is an optional cross-check only. When non-zero it must equal
	// the tenant of the (server-authenticated) context; the runner never
	// derives or overrides identity from it.
	TenantID uint64
	// ModelID selects the tenant's chat model via ModelService.
	ModelID string
	// Query is the task for this step.
	Query string
	// SystemPrompt optionally overrides the engine's system prompt.
	SystemPrompt string
	// AllowedTools is the explicit tool whitelist. Empty falls back to the
	// minimal toolset; the native default allowlist is never inherited.
	AllowedTools []string
	// MaxIterations caps ReAct rounds; default 5, hard cap 25.
	MaxIterations int
	// Timeout bounds the whole step end to end. Zero uses defaultStepTimeout.
	Timeout time.Duration
	// History is optional prior conversation handed to the stateless engine.
	History []chat.Message
}

// ToolCallSummary is the sanitized record of one tool invocation: name,
// duration and outcome only. Arguments, outputs, thought and reasoning are
// deliberately absent so they cannot be persisted downstream.
type ToolCallSummary struct {
	ToolName   string `json:"tool_name"`
	DurationMS int64  `json:"duration_ms,omitempty"`
	Success    bool   `json:"success"`
}

// StepTrace summarizes one ReAct iteration of the step.
type StepTrace struct {
	Iteration int               `json:"iteration"`
	Timestamp time.Time         `json:"timestamp"`
	ToolCalls []ToolCallSummary `json:"tool_calls,omitempty"`
}

// StepResult is the outcome of one step. It is safe to persist: it carries no
// model reasoning, tool arguments, or raw tool output.
type StepResult struct {
	StepID      string           `json:"step_id"`
	Status      StepStatus       `json:"status"`
	FinalAnswer string           `json:"final_answer,omitempty"`
	IsComplete  bool             `json:"is_complete"`
	ErrorCode   ErrorCode        `json:"error_code,omitempty"`
	ErrorDetail string           `json:"error_detail,omitempty"`
	Trace       []StepTrace      `json:"trace,omitempty"`
	TokenUsage  types.TokenUsage `json:"token_usage"`
	DurationMS  int64            `json:"duration_ms"`
	ModelID     string           `json:"model_id"`
}
