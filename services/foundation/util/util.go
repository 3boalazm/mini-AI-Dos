// Package util provides infrastructure primitives every service needs
// and none should reimplement independently: ID generation and a
// testable clock. It contains no domain logic — nothing here knows
// what a Provider or a Model is.
package util

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// NewUUIDv7 generates a UUIDv7 (RFC 9562) — time-ordered, so primary
// keys sort naturally by creation order without a separate indexed
// timestamp column. This is the id generation strategy
// database/entities.md specifies for every table's primary key; it
// lives here so every service generates ids the same way, not as a
// per-service reimplementation.
//
// Layout: 48 bits Unix ms timestamp | 4 bits version (0111) | 12 bits
// random | 2 bits variant (10) | 62 bits random.
func NewUUIDv7() (string, error) {
	var b [16]byte

	ms := time.Now().UnixMilli()
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)

	if _, err := rand.Read(b[6:]); err != nil {
		return "", fmt.Errorf("util: failed to read random bytes for UUIDv7: %w", err)
	}

	// Version 7: top nibble of byte 6 becomes 0111.
	b[6] = (b[6] & 0x0F) | 0x70
	// Variant RFC 4122: top two bits of byte 8 become 10.
	b[8] = (b[8] & 0x3F) | 0x80

	return formatUUID(b), nil
}

func formatUUID(b [16]byte) string {
	buf := make([]byte, 36)
	hex.Encode(buf[0:8], b[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], b[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], b[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], b[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], b[10:16])
	return string(buf)
}

// Clock abstracts time.Now so code that needs the current time is
// testable without sleeping or depending on wall-clock time in a test.
// A real service uses RealClock; a test injects a fixed FakeClock.
type Clock interface {
	Now() time.Time
}

type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }

// FakeClock returns a fixed instant every time Now is called — a test
// controls exactly what "now" means rather than depending on however
// long the test happens to take to run.
type FakeClock struct {
	FixedTime time.Time
}

func (c FakeClock) Now() time.Time { return c.FixedTime }
