package service

import (
	"context"
	"strings"
	"testing"
	"time"

	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
)

type fakeTenantAPIKeyRepo struct {
	byHash              map[string]*types.TenantAPIKey
	nextID              uint64
	lastUsedUpdateCount int
}

func newFakeTenantAPIKeyRepo() *fakeTenantAPIKeyRepo {
	return &fakeTenantAPIKeyRepo{byHash: map[string]*types.TenantAPIKey{}, nextID: 1}
}

func (r *fakeTenantAPIKeyRepo) seed(key *types.TenantAPIKey) *types.TenantAPIKey {
	cp := *key
	cp.ID = r.nextID
	r.nextID++
	r.byHash[cp.KeyHash] = &cp
	return &cp
}

func (r *fakeTenantAPIKeyRepo) GetAPIKeyByHash(_ context.Context, hash string) (*types.TenantAPIKey, error) {
	key, ok := r.byHash[hash]
	if !ok {
		return nil, apprepo.ErrTenantAPIKeyNotFound
	}
	cp := *key
	return &cp, nil
}

func (r *fakeTenantAPIKeyRepo) UpdateAPIKeyHash(_ context.Context, id uint64, hash string) error {
	for oldHash, key := range r.byHash {
		if key.ID == id && key.RevokedAt == nil {
			delete(r.byHash, oldHash)
			key.KeyHash = hash
			r.byHash[hash] = key
			return nil
		}
	}
	return apprepo.ErrTenantAPIKeyNotFound
}

func (r *fakeTenantAPIKeyRepo) HasKeysWithPlaceholderHash(_ context.Context) (bool, error) {
	for _, key := range r.byHash {
		if key.RevokedAt == nil && strings.HasPrefix(key.KeyHash, "migrated-tenant-") {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeTenantAPIKeyRepo) ListKeysWithPlaceholderHash(_ context.Context) ([]*types.TenantAPIKey, error) {
	out := []*types.TenantAPIKey{}
	for _, key := range r.byHash {
		if key.RevokedAt == nil && strings.HasPrefix(key.KeyHash, "migrated-tenant-") {
			cp := *key
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *fakeTenantAPIKeyRepo) UpdateAPIKeyLastUsed(_ context.Context, id uint64, at time.Time) error {
	r.lastUsedUpdateCount++
	for _, key := range r.byHash {
		if key.ID == id && key.RevokedAt == nil {
			key.LastUsedAt = &at
		}
	}
	return nil
}

func uint64Pointer(value uint64) *uint64 { return &value }

func TestTenantAPIKeyServiceBackfillMissingKeyHashes(t *testing.T) {
	ctx := context.Background()
	repo := newFakeTenantAPIKeyRepo()
	svc := NewTenantAPIKeyService(repo)

	token := "sk-legacy-token-value"
	repo.seed(&types.TenantAPIKey{
		TenantID:   uint64Pointer(7),
		Name:       "legacy",
		KeyHash:    "migrated-tenant-7",
		APIKey:     token,
		FullAccess: true,
	})

	n, err := svc.BackfillMissingKeyHashes(ctx)
	if err != nil {
		t.Fatalf("BackfillMissingKeyHashes returned error: %v", err)
	}
	if n != 1 {
		t.Fatalf("backfilled = %d, want 1", n)
	}
	if _, err := svc.AuthenticateAPIKey(ctx, token); err != nil {
		t.Fatalf("AuthenticateAPIKey after backfill returned error: %v", err)
	}
	if n, err := svc.BackfillMissingKeyHashes(ctx); err != nil || n != 0 {
		t.Fatalf("second BackfillMissingKeyHashes = (%d, %v), want (0, nil)", n, err)
	}
}

func TestTenantAPIKeyServiceAuthenticateThrottlesLastUsedUpdates(t *testing.T) {
	ctx := context.Background()
	repo := newFakeTenantAPIKeyRepo()
	svc := NewTenantAPIKeyService(repo)

	token := "sk-test-token-value"
	repo.seed(&types.TenantAPIKey{
		TenantID:   uint64Pointer(42),
		Name:       "integration",
		KeyHash:    hashTenantAPIKey(token),
		APIKey:     token,
		FullAccess: true,
	})

	for i := 0; i < 5; i++ {
		if _, err := svc.AuthenticateAPIKey(ctx, token); err != nil {
			t.Fatalf("AuthenticateAPIKey #%d returned error: %v", i+1, err)
		}
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for repo.lastUsedUpdateCount == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if repo.lastUsedUpdateCount != 1 {
		t.Fatalf("last_used update count = %d, want 1 (throttled async write)", repo.lastUsedUpdateCount)
	}
}

func TestTenantAPIKeyServiceAuthenticateRejectsExpiredKey(t *testing.T) {
	ctx := context.Background()
	repo := newFakeTenantAPIKeyRepo()
	svc := NewTenantAPIKeyService(repo)

	token := "sk-expired-token-value"
	expired := time.Now().UTC().Add(-time.Minute)
	repo.seed(&types.TenantAPIKey{
		TenantID:   uint64Pointer(42),
		Name:       "short-lived",
		KeyHash:    hashTenantAPIKey(token),
		APIKey:     token,
		FullAccess: true,
		ExpiresAt:  &expired,
	})
	if _, err := svc.AuthenticateAPIKey(ctx, token); err == nil {
		t.Fatal("expired key should not authenticate")
	}
}
