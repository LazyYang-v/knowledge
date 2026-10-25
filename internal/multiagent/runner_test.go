package multiagent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/models/rerank"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// ---- test doubles -------------------------------------------------------

// stubModelService resolves every GetChatModel from configurable outcomes.
type stubModelService struct {
	interfaces.ModelService
	model chat.Chat
	err   error
	calls int
}

func (s *stubModelService) GetChatModel(context.Context, string) (chat.Chat, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.model, nil
}

// stubAgentService records engine creation and returns a canned engine.
type stubAgentService struct {
	interfaces.AgentService
	createErr error
	engine    interfaces.AgentEngine
	configs   []*types.AgentConfig
}

func (s *stubAgentService) CreateAgentEngine(
	_ context.Context,
	config *types.AgentConfig,
	_ chat.Chat,
	_ rerank.Reranker,
	_ *event.EventBus,
	_, _ string,
) (interfaces.AgentEngine, error) {
	s.configs = append(s.configs, config)
	if s.createErr != nil {
		return nil, s.createErr
	}
	return s.engine, nil
}

// stubEngine overrides Execute only; the other AgentEngine methods are never
// called by the runner.
type stubEngine struct {
	interfaces.AgentEngine
	execute func(ctx context.Context) (*types.AgentState, error)
}

func (e *stubEngine) Execute(ctx context.Context, _, _, _ string, _ []chat.Message, _ ...[]string) (*types.AgentState, error) {
	return e.execute(ctx)
}

// scriptedChatModel speaks the real chat.Chat protocol: optionally one
// tool-call round, then a final answer with token usage.
type scriptedChatModel struct {
	mu            sync.Mutex
	rounds        int
	emitToolCall  bool
	toolName      string
	toolArguments string
	answer        string
	execErr       error
	blocking      bool
}

func (m *scriptedChatModel) Chat(context.Context, []chat.Message, *chat.ChatOptions) (*types.ChatResponse, error) {
	return &types.ChatResponse{}, nil
}

func (m *scriptedChatModel) ChatStream(
	_ context.Context, _ []chat.Message, _ *chat.ChatOptions,
) (<-chan types.StreamResponse, error) {
	m.mu.Lock()
	m.rounds++
	round := m.rounds
	m.mu.Unlock()

	ch := make(chan types.StreamResponse, 2)
	if m.blocking {
		close(ch)
		return ch, nil
	}
	if m.execErr != nil {
		ch <- types.StreamResponse{ResponseType: types.ResponseTypeError, Content: m.execErr.Error(), Done: true}
		close(ch)
		return ch, nil
	}
	if round == 1 && m.emitToolCall {
		ch <- types.StreamResponse{
			ResponseType: types.ResponseTypeToolCall,
			ToolCalls: []types.LLMToolCall{{
				ID:   "call-1",
				Type: "function",
				Function: types.FunctionCall{
					Name:      m.toolName,
					Arguments: m.toolArguments,
				},
			}},
			FinishReason: "tool_calls",
			Done:         true,
		}
		close(ch)
		return ch, nil
	}
	ch <- types.StreamResponse{
		ResponseType: types.ResponseTypeAnswer,
		Content:      m.answer,
		Done:         true,
		FinishReason: "stop",
		Usage:        &types.TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}
	close(ch)
	return ch, nil
}

func (*scriptedChatModel) GetModelName() string { return "scripted-chat" }
func (*scriptedChatModel) GetModelID() string   { return "scripted-chat-id" }

// tenantCtx builds a context carrying a server-side tenant identity, the same
// way the auth middleware does.
func tenantCtx(t *testing.T, tenantID uint64) context.Context {
	t.Helper()
	return context.WithValue(context.Background(), types.TenantIDContextKey, tenantID)
}

// realAgentService builds the production AgentService with every dependency
// the P1 converged config does not touch left nil, and a disabled sandbox.
func realAgentService() interfaces.AgentService {
	return service.NewAgentService(
		nil, nil, nil, nil, nil, nil, // cfg, model, kb, knowledge, file, chunk
		nil, nil, // mcp service, mcp manager
		nil, nil, // event bus (per-run), db
		nil, nil, nil, // web search, duckdb, wiki
		nil, nil, nil, // tenant, message, memory
		nil, nil, // storage resolver, approval gate
		sandbox.NewDisabledManager(), // sandbox mgr
		nil, nil, nil,                // sandbox resolver, pinner, policy
		service.HostSandboxManager{}, // host sandbox (zero value)
		nil, nil, nil,                // browser skill, user repo, graph repo
	)
}

func baseSpec() StepSpec {
	return StepSpec{
		StepID:  "unit-step",
		ModelID: "model-1",
		Query:   "hello",
	}
}

// ---- identity ------------------------------------------------------------

func TestRunTenantIdentityComesFromContextOnly(t *testing.T) {
	models := &stubModelService{model: &scriptedChatModel{}}
	runner := NewRunner(&stubAgentService{}, models)

	t.Run("missing tenant context blocks before any model call", func(t *testing.T) {
		res, err := runner.Run(context.Background(), baseSpec())
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
		if res.Status != StepBlocked || res.ErrorCode != ErrTenantContextMissing {
			t.Fatalf("got %s/%s, want BLOCKED/%s", res.Status, res.ErrorCode, ErrTenantContextMissing)
		}
		if models.calls != 0 {
			t.Fatalf("model service was called %d time(s) on a blocked step", models.calls)
		}
	})

	t.Run("spec tenant mismatching context identity is blocked", func(t *testing.T) {
		spec := baseSpec()
		spec.TenantID = 42
		res, _ := runner.Run(tenantCtx(t, 7), spec)
		if res.Status != StepBlocked || res.ErrorCode != ErrTenantMismatch {
			t.Fatalf("got %s/%s, want BLOCKED/%s", res.Status, res.ErrorCode, ErrTenantMismatch)
		}
		if models.calls != 0 {
			t.Fatalf("model service was called %d time(s) on a blocked step", models.calls)
		}
		if strings.Contains(res.ErrorDetail, " 7 ") == false && !strings.Contains(res.ErrorDetail, "42") {
			t.Fatalf("mismatch detail should name both tenants: %q", res.ErrorDetail)
		}
	})

	t.Run("spec tenant matching context identity is accepted", func(t *testing.T) {
		engine := &stubEngine{execute: func(context.Context) (*types.AgentState, error) {
			return &types.AgentState{IsComplete: true, FinalAnswer: "ok"}, nil
		}}
		runner := NewRunner(&stubAgentService{engine: engine}, &stubModelService{model: &scriptedChatModel{}})
		spec := baseSpec()
		spec.TenantID = 7
		res, _ := runner.Run(tenantCtx(t, 7), spec)
		if res.Status != StepSucceeded {
			t.Fatalf("got %s/%s, want SUCCEEDED", res.Status, res.ErrorCode)
		}
	})
}

// ---- preconditions -------------------------------------------------------

func TestRunPreconditionsBlockWithoutExecuting(t *testing.T) {
	engine := &stubEngine{execute: func(context.Context) (*types.AgentState, error) {
		t.Fatal("engine must not execute on a blocked step")
		return nil, nil
	}}
	runner := NewRunner(&stubAgentService{engine: engine}, &stubModelService{model: &scriptedChatModel{}})
	ctx := tenantCtx(t, 7)

	cases := []struct {
		name string
		spec StepSpec
		code ErrorCode
	}{
		{"empty model id", StepSpec{Query: "q"}, ErrModelNotConfigured},
		{"blank query", StepSpec{ModelID: "m"}, ErrInvalidSpec},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, _ := runner.Run(ctx, tc.spec)
			if res.Status != StepBlocked || res.ErrorCode != tc.code {
				t.Fatalf("got %s/%s, want BLOCKED/%s", res.Status, res.ErrorCode, tc.code)
			}
		})
	}
}

func TestRunModelResolutionFailureBlocks(t *testing.T) {
	engine := &stubEngine{execute: func(context.Context) (*types.AgentState, error) {
		t.Fatal("engine must not execute when the model cannot be resolved")
		return nil, nil
	}}
	models := &stubModelService{err: errors.New("model not found: model-1")}
	runner := NewRunner(&stubAgentService{engine: engine}, models)

	res, _ := runner.Run(tenantCtx(t, 7), baseSpec())
	if res.Status != StepBlocked || res.ErrorCode != ErrModelUnavailable {
		t.Fatalf("got %s/%s, want BLOCKED/%s", res.Status, res.ErrorCode, ErrModelUnavailable)
	}
	if res.ErrorDetail == "" {
		t.Fatal("blocked result should carry a sanitized detail")
	}
}

// ---- execution error classification --------------------------------------

func TestRunClassifiesExecutionFailures(t *testing.T) {
	ctx := tenantCtx(t, 7)

	t.Run("engine creation failure", func(t *testing.T) {
		runner := NewRunner(
			&stubAgentService{createErr: errors.New("registry exploded")},
			&stubModelService{model: &scriptedChatModel{}},
		)
		res, _ := runner.Run(ctx, baseSpec())
		if res.Status != StepFailed || res.ErrorCode != ErrEngineCreate {
			t.Fatalf("got %s/%s, want FAILED/%s", res.Status, res.ErrorCode, ErrEngineCreate)
		}
	})

	t.Run("engine execution failure", func(t *testing.T) {
		engine := &stubEngine{execute: func(context.Context) (*types.AgentState, error) {
			return nil, errors.New("upstream 500")
		}}
		runner := NewRunner(&stubAgentService{engine: engine}, &stubModelService{model: &scriptedChatModel{}})
		res, _ := runner.Run(ctx, baseSpec())
		if res.Status != StepFailed || res.ErrorCode != ErrEngineExecute {
			t.Fatalf("got %s/%s, want FAILED/%s", res.Status, res.ErrorCode, ErrEngineExecute)
		}
	})

	t.Run("nil state is an execution failure", func(t *testing.T) {
		engine := &stubEngine{execute: func(context.Context) (*types.AgentState, error) {
			return nil, nil
		}}
		runner := NewRunner(&stubAgentService{engine: engine}, &stubModelService{model: &scriptedChatModel{}})
		res, _ := runner.Run(ctx, baseSpec())
		if res.Status != StepFailed || res.ErrorCode != ErrEngineExecute {
			t.Fatalf("got %s/%s, want FAILED/%s", res.Status, res.ErrorCode, ErrEngineExecute)
		}
	})

	t.Run("incomplete engine state has no final answer", func(t *testing.T) {
		engine := &stubEngine{execute: func(context.Context) (*types.AgentState, error) {
			return &types.AgentState{IsComplete: false, CurrentRound: 5}, nil
		}}
		runner := NewRunner(&stubAgentService{engine: engine}, &stubModelService{model: &scriptedChatModel{}})
		res, _ := runner.Run(ctx, baseSpec())
		if res.Status != StepFailed || res.ErrorCode != ErrNoFinalAnswer {
			t.Fatalf("got %s/%s, want FAILED/%s", res.Status, res.ErrorCode, ErrNoFinalAnswer)
		}
		if res.FinalAnswer != "" {
			t.Fatal("a failed step must not expose a partial answer")
		}
	})
}

func TestRunClassifiesTimeAndCancellation(t *testing.T) {
	t.Run("wall-clock timeout", func(t *testing.T) {
		engine := &stubEngine{execute: func(ctx context.Context) (*types.AgentState, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}}
		spec := baseSpec()
		spec.Timeout = 50 * time.Millisecond
		runner := NewRunner(&stubAgentService{engine: engine}, &stubModelService{model: &scriptedChatModel{}})
		res, _ := runner.Run(tenantCtx(t, 7), spec)
		if res.Status != StepFailed || res.ErrorCode != ErrStepTimeout {
			t.Fatalf("got %s/%s, want FAILED/%s", res.Status, res.ErrorCode, ErrStepTimeout)
		}
	})

	t.Run("caller cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(tenantCtx(t, 7))
		cancel()
		engine := &stubEngine{execute: func(ctx context.Context) (*types.AgentState, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}}
		runner := NewRunner(&stubAgentService{engine: engine}, &stubModelService{model: &scriptedChatModel{}})
		res, _ := runner.Run(ctx, baseSpec())
		if res.Status != StepFailed || res.ErrorCode != ErrStepCanceled {
			t.Fatalf("got %s/%s, want FAILED/%s", res.Status, res.ErrorCode, ErrStepCanceled)
		}
	})
}

// ---- status/code class consistency ---------------------------------------

func TestErrorCodeClassesMatchStatuses(t *testing.T) {
	preconditions := []ErrorCode{
		ErrTenantContextMissing, ErrTenantMismatch, ErrInvalidSpec,
		ErrModelNotConfigured, ErrModelUnavailable,
	}
	for _, code := range preconditions {
		if !code.IsPrecondition() {
			t.Fatalf("%s must be classified as a precondition (BLOCKED)", code)
		}
	}
	executions := []ErrorCode{
		ErrEngineCreate, ErrEngineExecute, ErrStepTimeout, ErrStepCanceled, ErrNoFinalAnswer,
	}
	for _, code := range executions {
		if code.IsPrecondition() {
			t.Fatalf("%s must be classified as an execution failure (FAILED)", code)
		}
	}
}

// ---- config convergence --------------------------------------------------

func TestBuildStepConfigConvergence(t *testing.T) {
	t.Run("defaults are explicit and closed", func(t *testing.T) {
		cfg := buildStepConfig(&StepSpec{})
		if len(cfg.AllowedTools) != 1 || cfg.AllowedTools[0] != minimalToolset {
			t.Fatalf("allowed tools = %v, want [%s]", cfg.AllowedTools, minimalToolset)
		}
		if cfg.MaxIterations != defaultStepIterations {
			t.Fatalf("iterations = %d, want %d", cfg.MaxIterations, defaultStepIterations)
		}
		if cfg.MCPSelectionMode != "none" {
			t.Fatalf("mcp mode = %q, want none (native default for empty is all)", cfg.MCPSelectionMode)
		}
		if cfg.WebSearchEnabled || cfg.LocalBrowserEnabled || cfg.SkillsEnabled || cfg.MultiTurnEnabled {
			t.Fatal("optional capabilities must default to off")
		}
		if len(cfg.KnowledgeBases) != 0 || len(cfg.KnowledgeIDs) != 0 || len(cfg.SearchTargets) != 0 {
			t.Fatal("knowledge scope must default to empty")
		}
		if cfg.UseCustomSystemPrompt {
			t.Fatal("custom prompt flag must default to off")
		}
	})

	t.Run("iterations are clamped", func(t *testing.T) {
		low := buildStepConfig(&StepSpec{MaxIterations: -3})
		if low.MaxIterations != defaultStepIterations {
			t.Fatalf("negative iterations = %d, want default %d", low.MaxIterations, defaultStepIterations)
		}
		high := buildStepConfig(&StepSpec{MaxIterations: 1000})
		if high.MaxIterations != maxStepIterations {
			t.Fatalf("iterations = %d, want cap %d", high.MaxIterations, maxStepIterations)
		}
	})

	t.Run("allowed tools are copied not shared", func(t *testing.T) {
		tools := []string{"a", "b"}
		cfg := buildStepConfig(&StepSpec{AllowedTools: tools})
		tools[0] = "mutated"
		if cfg.AllowedTools[0] != "a" {
			t.Fatal("config tool list must not alias the spec slice")
		}
	})

	t.Run("system prompt opts in", func(t *testing.T) {
		cfg := buildStepConfig(&StepSpec{SystemPrompt: "be brief"})
		if !cfg.UseCustomSystemPrompt || cfg.SystemPrompt != "be brief" {
			t.Fatal("system prompt must be carried and flagged")
		}
	})
}

// ---- sanitization --------------------------------------------------------

func TestSanitizeDetailBoundsAndFlattens(t *testing.T) {
	long := strings.Repeat("x", 2000)
	got := sanitizeDetail(errors.New("line1\nline2\t" + long))
	if strings.ContainsAny(got, "\n\t") {
		t.Fatal("detail must be a single line")
	}
	if gotRuneLen(got) != maxErrorDetailRunes {
		t.Fatalf("detail length = %d, want cap %d", gotRuneLen(got), maxErrorDetailRunes)
	}
	if sanitizeDetail(nil) != "" {
		t.Fatal("nil error must sanitize to empty")
	}
}

func gotRuneLen(s string) int { return len([]rune(s)) }

func TestEventCollectorKeepsOnlySafeSummaries(t *testing.T) {
	c := newEventCollector()
	bus := event.NewEventBus()
	c.attach(bus)

	mustEmit(t, bus, event.Event{Type: event.EventAgentToolCall, Data: event.AgentToolCallData{
		ToolCallID: "call-1", ToolName: "thinking", Iteration: 0,
		Arguments: map[string]any{"thought": "SECRET-COVERT-REASONING"},
	}})
	mustEmit(t, bus, event.Event{Type: event.EventAgentToolResult, Data: event.AgentToolResultData{
		ToolCallID: "call-1", ToolName: "thinking", Iteration: 0,
		Success: true, Duration: 12, Output: "SECRET-TOOL-OUTPUT",
	}})
	// A result whose call notification was never seen still lands in the trace.
	mustEmit(t, bus, event.Event{Type: event.EventAgentToolResult, Data: event.AgentToolResultData{
		ToolCallID: "call-2", ToolName: "web_search", Iteration: 1,
		Success: false, Duration: 3, Error: "SECRET-FAILURE-REASON",
	}})

	traces := c.snapshot()
	if len(traces) != 2 {
		t.Fatalf("trace iterations = %d, want 2", len(traces))
	}
	first := traces[0].ToolCalls[0]
	if first.ToolName != "thinking" || !first.Success || first.DurationMS != 12 {
		t.Fatalf("unexpected summary %+v", first)
	}
	second := traces[1].ToolCalls[0]
	if second.ToolName != "web_search" || second.Success {
		t.Fatalf("unexpected summary %+v", second)
	}

	blob, err := json.Marshal(traces)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"SECRET-COVERT-REASONING", "SECRET-TOOL-OUTPUT", "SECRET-FAILURE-REASON", "arguments"} {
		if strings.Contains(string(blob), secret) {
			t.Fatalf("trace leaked %q: %s", secret, blob)
		}
	}
}

func mustEmit(t *testing.T, bus *event.EventBus, e event.Event) {
	t.Helper()
	if err := bus.Emit(context.Background(), e); err != nil {
		t.Fatalf("emit %s: %v", e.Type, err)
	}
}

// ---- end-to-end over the real engine -------------------------------------

// The success and sanitization paths run against the production AgentService
// and a real AgentEngine; only the chat model is scripted.
func newRealStack(t *testing.T, model chat.Chat) AgentRunner {
	t.Helper()
	return NewRunner(realAgentService(), &stubModelService{model: model})
}

func TestRunSucceedsOverRealEngine(t *testing.T) {
	model := &scriptedChatModel{answer: "the answer is 42"}
	res, err := newRealStack(t, model).Run(tenantCtx(t, 7), baseSpec())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if res.Status != StepSucceeded {
		t.Fatalf("status = %s (%s: %s)", res.Status, res.ErrorCode, res.ErrorDetail)
	}
	if res.FinalAnswer != "the answer is 42" || !res.IsComplete {
		t.Fatalf("answer = %q complete = %v", res.FinalAnswer, res.IsComplete)
	}
	if res.TokenUsage.TotalTokens == 0 {
		t.Fatal("token usage must be reported")
	}
	if res.DurationMS < 0 {
		t.Fatal("duration must be measured")
	}
}

func TestRunResultIsSanitizedOverRealEngine(t *testing.T) {
	model := &scriptedChatModel{
		emitToolCall:  true,
		toolName:      "thinking",
		toolArguments: `{"thought":"SECRET-COVERT-REASONING"}`,
		answer:        "done",
	}
	res, _ := newRealStack(t, model).Run(tenantCtx(t, 7), baseSpec())
	if res.Status != StepSucceeded {
		t.Fatalf("status = %s (%s: %s)", res.Status, res.ErrorCode, res.ErrorDetail)
	}
	if len(res.Trace) == 0 || len(res.Trace[0].ToolCalls) == 0 {
		t.Fatal("the thinking tool call must be traced")
	}
	if res.Trace[0].ToolCalls[0].ToolName != "thinking" {
		t.Fatalf("traced tool = %q, want thinking", res.Trace[0].ToolCalls[0].ToolName)
	}

	blob, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"SECRET-COVERT-REASONING", "\"thought\"", "reasoning", "arguments"} {
		if strings.Contains(string(blob), secret) {
			t.Fatalf("StepResult leaked %q: %s", secret, blob)
		}
	}
}
