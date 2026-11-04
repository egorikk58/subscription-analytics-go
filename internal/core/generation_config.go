package core

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

// AutoUsersCount keeps a fresh dataset sizeable while retaining seed reproducibility.
func AutoUsersCount(seed int64) int {
	digest := sha256.Sum256([]byte(fmt.Sprintf("users-count/v1/%d", seed)))
	return 25000 + int(binary.BigEndian.Uint32(digest[:4])%75001)
}
