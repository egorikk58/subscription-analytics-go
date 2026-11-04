package app

import (
	"fmt"
	"strconv"
	"strings"
	"subscription-analytics/internal/core"
)

// Old datasets stay intact and keep their actual generator version. New code
// can read them, but never silently relabel or regenerate them as the new model.
func sameDatasetParameters(stored, requested string) bool {
	a, b := strings.SplitN(stored, "/", 2), strings.SplitN(requested, "/", 2)
	if len(a) != 2 || len(b) != 2 {
		return false
	}
	switch a[0] {
	case "v1", "v2", "v2-migrated", core.GeneratorVersion:
		return a[1] == b[1]
	default:
		return false
	}
}

// Auto preserves an existing dataset. Fresh databases derive the size from seed.
func seedUsersCount(raw string, seed int64, previous string) (int, error) {
	if raw == "auto" {
		if previous != "" {
			fields := strings.SplitN(previous, "/", 5)
			if len(fields) != 5 {
				return 0, fmt.Errorf("invalid stored seed configuration")
			}
			count, err := strconv.Atoi(fields[2])
			if err != nil || count < 10000 {
				return 0, fmt.Errorf("invalid stored users_count")
			}
			return count, nil
		}
		return core.AutoUsersCount(seed), nil
	}
	count, err := strconv.Atoi(raw)
	if err != nil || count < 10000 || count > 100000 {
		return 0, fmt.Errorf("USERS_COUNT must be auto or an integer between 10000 and 100000")
	}
	return count, nil
}
