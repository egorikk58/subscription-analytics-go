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

func TestGeneratorUpgradeDoesNotRelabelExistingData(t *testing.T) {
	for _, version := range []string{"v1", "v2", "v2-migrated", "v3"} {
		if !sameDatasetParameters(version+"/42/10000/24/2026-11-01T00:00:00Z", "v3/42/10000/24/2026-11-01T00:00:00Z") {
			t.Fatal("compatible persisted dataset rejected", version)
		}
	}
	if sameDatasetParameters("v2/42/10000/24/2026-11-01T00:00:00Z", "v3/42/31419/24/2026-11-01T00:00:00Z") {
		t.Fatal("silently changed existing dataset size")
	}
}
