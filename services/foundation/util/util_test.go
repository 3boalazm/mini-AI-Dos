package util

import (
	"strings"
	"testing"
	"time"
)

func TestNewUUIDv7_CorrectFormat(t *testing.T) {
	id, err := NewUUIDv7()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(id) != 36 {
		t.Fatalf("expected 36-character UUID string, got %d chars: %q", len(id), id)
	}
	parts := strings.Split(id, "-")
	if len(parts) != 5 {
		t.Fatalf("expected 5 hyphen-separated groups, got %d: %q", len(parts), id)
	}
	wantLens := []int{8, 4, 4, 4, 12}
	for i, p := range parts {
		if len(p) != wantLens[i] {
			t.Errorf("group %d: expected length %d, got %d (%q)", i, wantLens[i], len(p), p)
		}
	}
}

func TestNewUUIDv7_VersionNibbleIsSeven(t *testing.T) {
	id, err := NewUUIDv7()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Third group's first character must be '7' per RFC 9562 §5.7.
	thirdGroup := strings.Split(id, "-")[2]
	if thirdGroup[0] != '7' {
		t.Errorf("expected version nibble '7', got %q in group %q", string(thirdGroup[0]), thirdGroup)
	}
}

func TestNewUUIDv7_VariantBitsAreRFC4122(t *testing.T) {
	id, err := NewUUIDv7()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Fourth group's first character's top two bits must be '10' —
	// meaning the hex digit is 8, 9, a, or b.
	fourthGroup := strings.Split(id, "-")[3]
	c := fourthGroup[0]
	if !(c == '8' || c == '9' || c == 'a' || c == 'b') {
		t.Errorf("expected variant nibble in {8,9,a,b}, got %q in group %q", string(c), fourthGroup)
	}
}

func TestNewUUIDv7_TimeOrdered(t *testing.T) {
	first, err := NewUUIDv7()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	time.Sleep(2 * time.Millisecond) // ensure the millisecond timestamp actually advances
	second, err := NewUUIDv7()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !(first < second) {
		t.Errorf("expected UUIDs generated in sequence to sort in generation order (the entire point of v7 over v4): first=%s second=%s", first, second)
	}
}

func TestNewUUIDv7_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 10000; i++ {
		id, err := NewUUIDv7()
		if err != nil {
			t.Fatalf("unexpected error on iteration %d: %v", i, err)
		}
		if seen[id] {
			t.Fatalf("duplicate UUID generated: %s", id)
		}
		seen[id] = true
	}
}

func TestFakeClock_ReturnsFixedTime(t *testing.T) {
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := FakeClock{FixedTime: fixed}

	if got := clock.Now(); !got.Equal(fixed) {
		t.Errorf("expected FakeClock to return the fixed time %v, got %v", fixed, got)
	}
	// Calling it twice must return the same instant — that's the point.
	if got := clock.Now(); !got.Equal(fixed) {
		t.Errorf("expected a second call to still return the fixed time, got %v", got)
	}
}

func TestRealClock_ReturnsCurrentTime(t *testing.T) {
	before := time.Now()
	got := RealClock{}.Now()
	after := time.Now()

	if got.Before(before) || got.After(after) {
		t.Errorf("expected RealClock.Now() to fall between %v and %v, got %v", before, after, got)
	}
}
