package store

import (
	"strings"
	"testing"
	"time"
)

func TestNewCommitID_LengthAndCharset(t *testing.T) {
	now := time.Now()
	id := NewCommitID(now)

	if len(id) != 26 {
		t.Fatalf("expected ID length 26, got %d (%s)", len(id), id)
	}

	charset := "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	for _, char := range id {
		if !strings.ContainsRune(charset, char) {
			t.Errorf("invalid character %c in ID %s", char, id)
		}
	}
}

func TestNewCommitID_Ordering(t *testing.T) {
	t1 := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	t2 := t1.Add(1 * time.Second)

	id1 := NewCommitID(t1)
	id2 := NewCommitID(t2)

	if id1 >= id2 {
		t.Errorf("expected id1 (%s) to be less than id2 (%s) based on time", id1, id2)
	}
}

func TestNewCommitID_Randomness(t *testing.T) {
	now := time.Now()
	id1 := NewCommitID(now)
	id2 := NewCommitID(now)

	if id1 == id2 {
		t.Errorf("expected IDs generated at the same time to have different randomness, both got %s", id1)
	}
}
