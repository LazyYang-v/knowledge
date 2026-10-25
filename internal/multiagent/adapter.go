package multiagent

import (
	"context"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/types"
)

// AgentRunner executes one single-agent step synchronously and returns a
// sanitized StepResult. Execution outcomes (including BLOCKED and FAILED
// classifications) are carried on the result; the error return is non-nil
// only for caller bugs (nil runner dependencies), never for step outcomes.
type AgentRunner interface {
	Run(ctx context.Context, spec StepSpec) (*StepResult, error)
}

const (
	// defaultStepTimeout bounds one synchronous step end to end, including
	// model resolution and every ReAct round.
	defaultStepTimeout = 5 * time.Minute
	// defaultStepIterations is the ReAct round budget when the spec omits one.
	defaultStepIterations = 5
	// maxStepIterations is the hard cap for a multiagent step, tighter than
	// the engine's own cap so a runaway spec cannot pin a worker.
	maxStepIterations = 25
	// maxErrorDetailRunes bounds the sanitized error detail carried on
	// StepResult so provider payloads cannot bloat or poison persistence.
	maxErrorDetailRunes = 500
)

// minimalToolset is the fallback whitelist when a spec declares none. It is
// explicit on purpose: an empty AllowedTools would otherwise inherit the
// native DefaultAllowedTools() list and break config convergence.
const minimalToolset = tools.ToolThinking

// buildStepConfig projects a StepSpec onto a native AgentConfig with every
// optional capability P1 does not exercise switched off. The returned config
// is tenant-independent; identity stays on the caller's context.
func buildStepConfig(spec *StepSpec) *types.AgentConfig {
	allowed := spec.AllowedTools
	if len(allowed) == 0 {
		allowed = []string{minimalToolset}
	} else {
		// Copy: the spec must not share state with the config handed downstream.
		allowed = append([]string(nil), allowed...)
	}

	maxIter := spec.MaxIterations
	if maxIter <= 0 {
		maxIter = defaultStepIterations
	}
	if maxIter > maxStepIterations {
		maxIter = maxStepIterations
	}

	return &types.AgentConfig{
		MaxIterations:         maxIter,
		AllowedTools:          allowed,
		SystemPrompt:          spec.SystemPrompt,
		UseCustomSystemPrompt: spec.SystemPrompt != "",
		// MCPSelectionMode "none" is load-bearing: the native default for an
		// empty mode is "all", which would fan in tenant MCP services.
		MCPSelectionMode:    "none",
		MultiTurnEnabled:    false,
		WebSearchEnabled:    false,
		LocalBrowserEnabled: false,
		SkillsEnabled:       false,
	}
}

// sanitizeDetail flattens an error message into a single bounded line. It
// keeps provider diagnostics readable while stripping control characters and
// capping length; it never embeds prompts, tool arguments, or outputs.
func sanitizeDetail(err error) string {
	if err == nil {
		return ""
	}
	detail := strings.Join(strings.Fields(err.Error()), " ")
	runes := []rune(detail)
	if len(runes) > maxErrorDetailRunes {
		runes = runes[:maxErrorDetailRunes]
	}
	return string(runes)
}
