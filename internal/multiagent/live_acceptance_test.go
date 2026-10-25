package multiagent

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Live acceptance: run the full stack — real model row from the live
// WeKnora database, real chat model client, real AgentService/AgentEngine —
// against a configured provider. A scripted model is never substituted: when
// the gates below are not satisfied the suite reports BLOCKED — LIVE
// ACCEPTANCE instead of running.
//
// Gates:
//
//	WEKNORA_LIVE_ACCEPTANCE=1        (master switch; otherwise skip)
//	WEKNORA_LIVE_DSN                 (host-reachable postgres DSN)
//	WEKNORA_LIVE_TENANT_ID           (default 10000)
//	WEKNORA_LIVE_MODEL_ID            (default: the deepseek-chat row)
func liveGates(t *testing.T) (dsn string, tenantID uint64, modelID string) {
	t.Helper()
	if os.Getenv("WEKNORA_LIVE_ACCEPTANCE") != "1" {
		t.Skip("BLOCKED — LIVE ACCEPTANCE: set WEKNORA_LIVE_ACCEPTANCE=1, WEKNORA_LIVE_DSN, WEKNORA_LIVE_TENANT_ID, WEKNORA_LIVE_MODEL_ID")
	}
	dsn = os.Getenv("WEKNORA_LIVE_DSN")
	if dsn == "" {
		t.Skip("BLOCKED — LIVE ACCEPTANCE: WEKNORA_LIVE_DSN is not set; refusing to fall back to a scripted model")
	}
	tenantID = 10000
	if v := os.Getenv("WEKNORA_LIVE_TENANT_ID"); v != "" {
		parsed, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			t.Fatalf("WEKNORA_LIVE_TENANT_ID: %v", err)
		}
		tenantID = parsed
	}
	modelID = "88174841-2a00-4bd1-a677-b726d0851d33" // deepseek-chat
	if v := os.Getenv("WEKNORA_LIVE_MODEL_ID"); v != "" {
		modelID = v
	}
	return dsn, tenantID, modelID
}

// newLiveRunner wires the production ModelService over the live database and
// the P1-converged AgentService. The chat client it resolves is the same one
// the production QA path builds for this model row.
func newLiveRunner(t *testing.T, dsn string) AgentRunner {
	t.Helper()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open live database: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	modelService := service.NewModelService(
		repository.NewModelRepository(db), nil, nil, nil, nil, nil,
	)
	return NewRunner(realAgentService(), modelService)
}

func TestLiveAcceptanceRealModel(t *testing.T) {
	dsn, tenantID, modelID := liveGates(t)
	runner := newLiveRunner(t, dsn)
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, tenantID)

	t.Run("real model answers a real question", func(t *testing.T) {
		spec := StepSpec{
			StepID:  "live-acceptance",
			ModelID: modelID,
			Query:   "用一句话回答：1+1等于几？",
			Timeout: 120 * time.Second,
		}
		res, err := runner.Run(ctx, spec)
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
		if res.Status != StepSucceeded {
			t.Fatalf("status = %s (%s: %s)", res.Status, res.ErrorCode, res.ErrorDetail)
		}
		if strings.TrimSpace(res.FinalAnswer) == "" {
			t.Fatalf("final answer is empty: %+v", res)
		}
		if !res.IsComplete {
			t.Fatalf("engine did not complete: %+v", res)
		}
		if res.TokenUsage.TotalTokens == 0 {
			t.Fatalf("provider reported no token usage: %+v", res.TokenUsage)
		}
		if res.DurationMS <= 0 {
			t.Fatalf("duration not measured: %d", res.DurationMS)
		}
		t.Logf("LIVE OK: answer=%q tokens=%d duration=%dms",
			res.FinalAnswer, res.TokenUsage.TotalTokens, res.DurationMS)
	})

	t.Run("unknown model blocks as a precondition", func(t *testing.T) {
		spec := StepSpec{
			StepID:  "live-blocked",
			ModelID: "00000000-0000-0000-0000-000000000000",
			Query:   "hi",
		}
		res, _ := runner.Run(ctx, spec)
		if res.Status != StepBlocked || res.ErrorCode != ErrModelUnavailable {
			t.Fatalf("got %s/%s, want BLOCKED/%s", res.Status, res.ErrorCode, ErrModelUnavailable)
		}
	})

	t.Run("tenant mismatch blocks", func(t *testing.T) {
		spec := StepSpec{
			StepID:   "live-mismatch",
			TenantID: tenantID + 1,
			ModelID:  modelID,
			Query:    "hi",
		}
		res, _ := runner.Run(ctx, spec)
		if res.Status != StepBlocked || res.ErrorCode != ErrTenantMismatch {
			t.Fatalf("got %s/%s, want BLOCKED/%s", res.Status, res.ErrorCode, ErrTenantMismatch)
		}
	})

	t.Run("wall-clock timeout fails with structured code", func(t *testing.T) {
		spec := StepSpec{
			StepID:  "live-timeout",
			ModelID: modelID,
			Query:   "写一篇三百字的短文",
			Timeout: 50 * time.Millisecond,
		}
		res, _ := runner.Run(ctx, spec)
		if res.Status != StepFailed || res.ErrorCode != ErrStepTimeout {
			t.Fatalf("got %s/%s, want FAILED/%s", res.Status, res.ErrorCode, ErrStepTimeout)
		}
	})
}
