package process

import (
	"errors"
	"testing"
	"time"
)

// countingProvider wires a GopsutilProvider to a fake enumeration pass and a
// manual clock so the cache window can be exercised deterministically.
func countingProvider(t *testing.T, lites []liteProc) (*GopsutilProvider, *int, *time.Time) {
	t.Helper()
	calls := 0
	clock := time.Date(2026, time.September, 10, 12, 0, 0, 0, time.UTC)
	p := &GopsutilProvider{
		enumerate: func() ([]liteProc, error) {
			calls++
			return lites, nil
		},
		now: func() time.Time { return clock },
	}
	return p, &calls, &clock
}

func TestGopsutilProvider_ReusesEnumerationWithinTTL(t *testing.T) {
	p, calls, clock := countingProvider(t, nil)

	if _, err := p.lites(); err != nil {
		t.Fatalf("first lites: %v", err)
	}
	*clock = clock.Add(liteCacheTTL / 2)
	if _, err := p.lites(); err != nil {
		t.Fatalf("second lites: %v", err)
	}
	if *calls != 1 {
		t.Errorf("enumeration passes within TTL = %d, want 1", *calls)
	}

	*clock = clock.Add(liteCacheTTL)
	if _, err := p.lites(); err != nil {
		t.Fatalf("third lites: %v", err)
	}
	if *calls != 2 {
		t.Errorf("enumeration passes after TTL = %d, want 2", *calls)
	}
}

// An empty host result must still count as a valid pass and be reused —
// otherwise a quiet host would re-enumerate on every call.
func TestGopsutilProvider_CachesEmptyEnumeration(t *testing.T) {
	p, calls, _ := countingProvider(t, []liteProc{})
	if _, err := p.lites(); err != nil {
		t.Fatalf("first lites: %v", err)
	}
	if _, err := p.lites(); err != nil {
		t.Fatalf("second lites: %v", err)
	}
	if *calls != 1 {
		t.Errorf("enumeration passes = %d, want 1", *calls)
	}
}

// A failed pass must not be cached: the next caller retries the OS.
func TestGopsutilProvider_DoesNotCacheErrors(t *testing.T) {
	calls := 0
	p := &GopsutilProvider{
		enumerate: func() ([]liteProc, error) {
			calls++
			return nil, errors.New("boom")
		},
		now: time.Now,
	}
	if _, err := p.lites(); err == nil {
		t.Fatal("expected error from first lites")
	}
	if _, err := p.lites(); err == nil {
		t.Fatal("expected error from second lites")
	}
	if calls != 2 {
		t.Errorf("enumeration passes = %d, want 2 (errors are not cached)", calls)
	}
}
