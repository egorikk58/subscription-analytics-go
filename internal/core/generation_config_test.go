package core

import "testing"

func TestAutoUsersCount(t *testing.T) {
	if got := AutoUsersCount(42); got != 31419 {
		t.Fatalf("seed42 count = %d", got)
	}
	if AutoUsersCount(731) == AutoUsersCount(42) {
		t.Fatal("different seeds should exercise different dataset sizes")
	}
	for _, seed := range []int64{0, 1, -1, 42, 731, 2147483647} {
		if n := AutoUsersCount(seed); n < 25000 || n > 100000 {
			t.Fatalf("size out of bounds: %d", n)
		}
	}
}
