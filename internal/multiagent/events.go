package multiagent

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/event"
)

// eventCollector subscribes to the per-run EventBus and keeps only execution
// summaries that are safe to expose on StepResult: tool names, durations and
// success flags. Thought content, reasoning, tool arguments and tool outputs
// arrive in the same events but are dropped at the source — they are never
// stored, so they cannot leak into a result or any persistence built on it.
type eventCollector struct {
	mu    sync.Mutex
	calls []recordedCall
}

// recordedCall is one tool invocation seen on the bus. A tool_call event
// starts the record; the matching tool_result event completes it. Records
// without a result keep Success=false, which matches how a step that died
// mid-tool must be reported.
type recordedCall struct {
	toolCallID string
	iteration  int
	startedAt  time.Time
	summary    ToolCallSummary
}

func newEventCollector() *eventCollector {
	return &eventCollector{}
}

// attach subscribes the collector to the run's bus. Subscribe before engine
// Execute so no emitted event is missed; the sync bus dispatches inline.
func (c *eventCollector) attach(bus *event.EventBus) {
	bus.On(event.EventAgentToolCall, c.onToolCall)
	bus.On(event.EventAgentToolResult, c.onToolResult)
}

func (c *eventCollector) onToolCall(_ context.Context, e event.Event) error {
	data, ok := e.Data.(event.AgentToolCallData)
	if !ok || data.ToolName == "" {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.calls {
		if c.calls[i].toolCallID == data.ToolCallID && data.ToolCallID != "" {
			return nil // duplicate notification for the same call
		}
	}
	c.calls = append(c.calls, recordedCall{
		toolCallID: data.ToolCallID,
		iteration:  data.Iteration,
		startedAt:  time.Now(),
		// Arguments and Hint are intentionally discarded.
		summary: ToolCallSummary{ToolName: data.ToolName},
	})
	return nil
}

func (c *eventCollector) onToolResult(_ context.Context, e event.Event) error {
	data, ok := e.Data.(event.AgentToolResultData)
	if !ok {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.calls {
		if data.ToolCallID != "" && c.calls[i].toolCallID == data.ToolCallID {
			c.calls[i].summary.Success = data.Success
			c.calls[i].summary.DurationMS = data.Duration
			return nil
		}
	}
	// A result without a recorded call is still a real execution the trace
	// must show; Output/Error contents stay behind.
	c.calls = append(c.calls, recordedCall{
		toolCallID: data.ToolCallID,
		iteration:  data.Iteration,
		startedAt:  time.Now(),
		summary: ToolCallSummary{
			ToolName:   data.ToolName,
			DurationMS: data.Duration,
			Success:    data.Success,
		},
	})
	return nil
}

// snapshot groups the recorded calls into per-iteration traces. Iterations
// are ordered ascending; calls keep emission order.
func (c *eventCollector) snapshot() []StepTrace {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.calls) == 0 {
		return nil
	}
	byIteration := make(map[int]*StepTrace)
	order := make([]int, 0, len(c.calls))
	for _, call := range c.calls {
		trace, ok := byIteration[call.iteration]
		if !ok {
			trace = &StepTrace{Iteration: call.iteration, Timestamp: call.startedAt}
			byIteration[call.iteration] = trace
			order = append(order, call.iteration)
		}
		trace.ToolCalls = append(trace.ToolCalls, call.summary)
	}
	sort.Ints(order)
	traces := make([]StepTrace, 0, len(order))
	for _, iter := range order {
		traces = append(traces, *byIteration[iter])
	}
	return traces
}
