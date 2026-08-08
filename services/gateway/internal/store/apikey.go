// Package store is the gateway's persistence layer: the APIKey model,
// the repository abstraction, and key generation/hashing. SQL exists
// only in this package's Postgres implementation — handlers and
// middleware never see it.
package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	stderrors "errors"
	"fmt"
	"time"

	"github.com/ai-dos/foundation/util"
)

// Sentinel errors the auth layer branches on. Deliberately coarse:
// callers outside this package never learn more than "not found" /
// "duplicate" — revocation is a field on the returned key, not an
// error, so the middleware can treat unknown and revoked identically
// at the HTTP boundary.
var (
	ErrNotFound  = stderrors.New("store: api key not found")
	ErrDuplicate = stderrors.New("store: api key hash already exists")
)

// APIKey mirrors the api_keys table (migrations/000001). KeyHash is
// the SHA-256 hex of the raw key — the raw key itself exists only in
// the moment of generation and in the caller's hands.
type APIKey struct {
	ID        string
	KeyHash   string
	KeyPrefix string
	Name      string
	CreatedAt time.Time
	RevokedAt *time.Time
}

// Revoked reports whether the key has been revoked.
func (k *APIKey) Revoked() bool { return k.RevokedAt != nil }

// Repository is the whole persistence abstraction the gateway needs.
type Repository interface {
	// FindByHash returns the key with the given hash, revoked or not
	// (the caller decides what revocation means), or ErrNotFound.
	FindByHash(ctx context.Context, hash string) (*APIKey, error)
	// Create inserts a new key row. ErrDuplicate on hash collision.
	Create(ctx context.Context, key *APIKey) error
	// Revoke marks the key revoked. ErrNotFound if no active key has
	// that id (already-revoked keys are not re-revoked).
	Revoke(ctx context.Context, id string) error
	// List returns all keys, newest first. Hashes are included in the
	// struct but must never be printed or serialized by callers.
	List(ctx context.Context) ([]APIKey, error)
}

// keyPrefixLen is how much of the raw key is stored/shown for human
// identification: "mad_" plus 8 hex chars — enough to tell keys apart,
// far too little to reconstruct one.
const keyPrefixLen = 12

// HashKey is the single definition of raw-key → stored-hash. SHA-256
// is appropriate here (not bcrypt/argon2) because raw keys are
// 24 random bytes — brute-forcing the preimage space is infeasible,
// and auth-path lookups must be a cheap indexed equality match.
func HashKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// GenerateKey mints a new raw key and its storable record. The raw key
// is returned exactly once and never retained by this package.
func GenerateKey(name string, now time.Time) (raw string, key *APIKey, err error) {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", nil, fmt.Errorf("store: failed to read random bytes for api key: %w", err)
	}
	raw = "mad_" + hex.EncodeToString(b[:])

	id, err := util.NewUUIDv7()
	if err != nil {
		return "", nil, fmt.Errorf("store: failed to generate api key id: %w", err)
	}

	return raw, &APIKey{
		ID:        id,
		KeyHash:   HashKey(raw),
		KeyPrefix: raw[:keyPrefixLen],
		Name:      name,
		CreatedAt: now.UTC(),
	}, nil
}
