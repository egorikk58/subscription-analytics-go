package app

import "testing"

func TestSeedCountPreservesExistingData(t *testing.T) {
	n, err := seedUsersCount("auto", 42, "v1/42/10000/24/2026-11-01T00:00:00Z")
	if err != nil || n != 10000 {
		t.Fatalf("existing seed changed: %d %v", n, err)
	}
	n, err = seedUsersCount("auto", 42, "")
	if err != nil || n < 25000 || n > 100000 {
		t.Fatalf("new auto count: %d %v", n, err)
	}
	for _, raw := range []string{"9999", "100001", "bad", ""} {
		if _, err = seedUsersCount(raw, 42, ""); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}
