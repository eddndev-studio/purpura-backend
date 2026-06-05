package sys

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestClock_NowIsUTC(t *testing.T) {
	before := time.Now().UTC()
	got := NewClock().Now()
	after := time.Now().UTC()

	if got.Location() != time.UTC {
		t.Errorf("Now() debe estar en UTC, obtuve %s", got.Location())
	}
	if got.Before(before) || got.After(after) {
		t.Errorf("Now() = %s fuera de [%s, %s]", got, before, after)
	}
}

func TestUUIDGenerator_NewIDIsUniqueV4(t *testing.T) {
	g := NewUUIDGenerator()
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id := g.NewID()
		parsed, err := uuid.Parse(id)
		if err != nil {
			t.Fatalf("NewID() = %q no es un uuid valido: %v", id, err)
		}
		if parsed.Version() != 4 {
			t.Errorf("NewID() version = %d, quiero 4", parsed.Version())
		}
		if seen[id] {
			t.Fatalf("NewID() repitio %q", id)
		}
		seen[id] = true
	}
}
