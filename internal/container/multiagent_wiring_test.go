package container

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/multiagent"
	"go.uber.org/dig"
)

// Resolve the multiagent AgentRunner from the production dependency graph in
// dry-run mode, mirroring TestRouterWiring: this catches missing DI types and
// parameter-shape mistakes that go build alone cannot.
func TestMultiagentRunnerWiring(t *testing.T) {
	for _, mode := range []struct {
		name      string
		redisAddr string
	}{
		{name: "lite"},
		{name: "redis", redisAddr: "localhost:6379"},
	} {
		t.Run(mode.name, func(t *testing.T) {
			t.Setenv("REDIS_ADDR", mode.redisAddr)
			c := BuildContainer(dig.New(dig.DryRun(true)))
			if err := c.Invoke(func(r multiagent.AgentRunner) {}); err != nil {
				t.Fatalf("resolve multiagent runner dependencies: %v", err)
			}
		})
	}
}
