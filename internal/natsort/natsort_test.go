package natsort

import (
	"sort"
	"testing"
)

func TestLess(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"[LA-9] Soda Stereo", "[LA-10] Charly García", true},
		{"[LA-10] Charly García", "[LA-9] Soda Stereo", false},
		{"track 2", "Track 10", true},
		{"CD 01", "cd 1", false},
		{"a", "b", true},
		{"abc", "abcd", true},
		{"same", "same", false},
		{"Disc 2", "Disc 2b", true},
	}
	for _, c := range cases {
		if got := Less(c.a, c.b); got != c.want {
			t.Errorf("Less(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestSortStable(t *testing.T) {
	s := []string{"10 - x", "2 - x", "1 - x", "01 - y"}
	sort.SliceStable(s, func(i, j int) bool { return Less(s[i], s[j]) })
	want := []string{"1 - x", "01 - y", "2 - x", "10 - x"}
	for i := range want {
		if s[i] != want[i] {
			t.Fatalf("got %q, want %q", s, want)
		}
	}
}
