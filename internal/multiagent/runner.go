package multiagent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// Runner is the P1 AgentRunner: one synchronous agent step over the native
// AgentService/AgentEngine stack. It owns no state between calls; every Run
// builds its own EventBus, tool registry and engine.
type Runner struct {
	agents interfaces.AgentService
	models interfaces.ModelService
}

// NewRunner wires the runner from the container graph. Both dependencies are
// the production services — the runner never constructs models or engines
// itself, so tenant authorization, tool registration and MCP approval all
// keep their native semantics.
func NewRunner(agents interfaces.AgentService, models interfaces.ModelService) AgentRunner {
	return &Runner{agents: agents, models: models}
}

// Run executes one step. See AgentRunner for the outcome contract.
func (r *Runner) Run(ctx context.Context, spec StepSpec) (*StepResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	start := time.Now()
	res := &StepResult{StepID: spec.StepID, ModelID: spec.ModelID}
	defer func() { res.DurationMS = time.Since(start).Milliseconds() }()

	if r == nil || r.agents == nil || r.models == nil {
		return nil, errors.New("multiagent: runner dependencies are not wired")
	}

	// Identity comes from the server-side context only. The spec's TenantID
	// is a cross-check, never a source: a mismatch is a blocked step, and no
	// code path below injects or overrides the context tenant.
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok {
		return blocked(res, ErrTenantContextMissing,
			"execution tenant is missing from the server context"), nil
	}
	if spec.TenantID != 0 && spec.TenantID != tenantID {
		return blocked(res, ErrTenantMismatch,
			fmt.Sprintf("spec tenant %d does not match the authenticated tenant %d",
				spec.TenantID, tenantID)), nil
	}
	if spec.ModelID == "" {
		return blocked(res, ErrModelNotConfigured, "model id is empty"), nil
	}
	if strings.TrimSpace(spec.Query) == "" {
		return blocked(res, ErrInvalidSpec, "query is empty"), nil
	}

	timeout := spec.Timeout
	if timeout <= 0 {
		timeout = defaultStepTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	logger.Infof(runCtx, "[multiagent] step %s starting: model=%s tenant=%d timeout=%s",
		spec.StepID, spec.ModelID, tenantID, timeout)

	// Model resolution is a precondition. A model that cannot be resolved
	// (missing row, missing credentials, failed client construction) blocks
	// the step rather than failing it: nothing has executed yet.
	chatModel, err := r.models.GetChatModel(runCtx, spec.ModelID)
	if err != nil {
		if code, detail := classifyContext(runCtx, err); code != "" {
			return failed(res, code, detail), nil
		}
		return blocked(res, ErrModelUnavailable, sanitizeDetail(err)), nil
	}

	config := buildStepConfig(&spec)
	bus := event.NewEventBus()
	collector := newEventCollector()
	collector.attach(bus)

	sessionID := stepSessionID(spec.StepID)
	engine, err := r.agents.CreateAgentEngine(
		runCtx, config, chatModel, nil, bus, sessionID, stepMessageID(spec.StepID),
	)
	if err != nil {
		if code, detail := classifyContext(runCtx, err); code != "" {
			return failed(res, code, detail), nil
		}
		return failed(res, ErrEngineCreate, sanitizeDetail(err)), nil
	}

	state, execErr := engine.Execute(
		runCtx, sessionID, stepMessageID(spec.StepID), spec.Query, spec.History,
	)

	res.Trace = collector.snapshot()
	if execErr != nil {
		code := ErrEngineExecute
		if runCtx.Err() == context.DeadlineExceeded || errors.Is(execErr, context.DeadlineExceeded) {
			code = ErrStepTimeout
		} else if runCtx.Err() == context.Canceled || errors.Is(execErr, context.Canceled) {
			code = ErrStepCanceled
		}
		return failed(res, code, sanitizeDetail(execErr)), nil
	}
	if state == nil {
		return failed(res, ErrEngineExecute, "engine returned no state"), nil
	}

	res.TokenUsage = state.TurnUsage
	res.IsComplete = state.IsComplete
	res.FinalAnswer = state.FinalAnswer
	switch {
	case !state.IsComplete && state.FinalAnswer == "":
		return failed(res, ErrNoFinalAnswer,
			fmt.Sprintf("engine stopped after %d round(s) without completing", state.CurrentRound)), nil
	case !state.IsComplete:
		return failed(res, ErrNoFinalAnswer,
			"engine stopped before completing; partial answer withheld"), nil
	case state.FinalAnswer == "":
		return failed(res, ErrNoFinalAnswer,
			"engine completed without a final answer"), nil
	}

	res.Status = StepSucceeded
	logger.Infof(runCtx, "[multiagent] step %s succeeded: rounds=%d tokens=%d duration=%dms",
		spec.StepID, state.CurrentRound, state.TurnUsage.TotalTokens, res.DurationMS)
	return res, nil
}

// classifyContext maps an aborted runCtx onto the execution error class while
// the underlying error may still be wrapping it. Empty code means the error
// is unrelated to context cancellation.
func classifyContext(ctx context.Context, err error) (ErrorCode, string) {
	switch {
	case ctx.Err() == context.DeadlineExceeded || errors.Is(err, context.DeadlineExceeded):
		return ErrStepTimeout, "step exceeded its time budget"
	case ctx.Err() == context.Canceled || errors.Is(err, context.Canceled):
		return ErrStepCanceled, "step canceled by caller"
	}
	return "", ""
}

// stepSessionID derives the engine session ID from the step. The "ma-" prefix
// keeps multiagent runs distinguishable from user sessions in logs and any
// session-keyed store the engine touches.
func stepSessionID(stepID string) string {
	id := strings.TrimSpace(stepID)
	if id == "" {
		return "ma-step"
	}
	return "ma-" + id
}

func stepMessageID(stepID string) string {
	return stepSessionID(stepID) + "-msg"
}

func blocked(res *StepResult, code ErrorCode, detail string) *StepResult {
	res.Status = StepBlocked
	res.ErrorCode = code
	res.ErrorDetail = detail
	return res
}

func failed(res *StepResult, code ErrorCode, detail string) *StepResult {
	res.Status = StepFailed
	res.ErrorCode = code
	res.ErrorDetail = detail
	return res
}
