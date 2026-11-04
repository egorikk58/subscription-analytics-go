package app

import (
	"fmt"
	"strconv"
	"strings"
	"subscription-analytics/internal/core"
)

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
