package store

import (
	"strings"
	"testing"
	"time"
)

func TestHashKey_DeterministicAndDistinct(t *testing.T) {
	a1 := HashKey("mad_aaaa")
	a2 := HashKey("mad_aaaa")
	b := HashKey("mad_bbbb")

	if a1 != a2 {
		t.Error("same input must hash identically")
	}
	if a1 == b {
		t.Error("different inputs must not collide trivially")
	}
	if len(a1) != 64 {
		t.Errorf("SHA-256 hex must be 64 chars, got %d", len(a1))
	}
	if a1 == "mad_aaaa" {
		t.Error("hash must not be the raw key")
	}
}

func TestGenerateKey(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	raw, key, err := GenerateKey("test label", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(raw, "mad_") {
		t.Errorf("raw key should carry the mad_ prefix, got %q", raw[:8])
	}
	if len(raw) != len("mad_")+48 {
		t.Errorf("raw key length: got %d, want %d (24 random bytes hex)", len(raw), len("mad_")+48)
	}
	if key.KeyHash != HashKey(raw) {
		t.Error("stored hash must be the hash of the raw key")
	}
	if key.KeyHash == raw || strings.Contains(key.KeyHash, raw) {
		t.Error("the raw key must never appear in the stored record")
	}
	if key.KeyPrefix != raw[:12] {
		t.Errorf("prefix should be the first 12 chars of the raw key, got %q", key.KeyPrefix)
	}
	if key.Name != "test label" || key.ID == "" || !key.CreatedAt.Equal(now) {
		t.Errorf("record fields wrong: %+v", key)
	}
	if key.Revoked() {
		t.Error("a new key must not be revoked")
	}
}

func TestGenerateKey_Unique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		raw, _, err := GenerateKey("", time.Now())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if seen[raw] {
			t.Fatal("generated a duplicate raw key")
		}
		seen[raw] = true
	}
}
