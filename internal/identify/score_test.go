package identify

import (
	"math"
	"testing"

	"tordo/internal/musicbrainz"
)

func TestSimilarity(t *testing.T) {
	if similarity("Fito Páez", "fito paez!") != 1 || similarity("", "x") != 0 {
		t.Fatal("similarity of equal or empty names")
	}
	if s := similarity("Sin sobresalt", "Sin sobresaltos"); s < 0.8 || s >= 1 {
		t.Fatalf("truncated title similarity = %v", s)
	}
}

func TestPartialScores(t *testing.T) {
	cases := []struct {
		name      string
		got, want float64
	}{
		{"year unknown", yearScore(0, 1986), 0.5},
		{"year exact", yearScore(1986, 1986), 1},
		{"year off by one", yearScore(1986, 1987), 0.5},
		{"year from original", yearScore(1986, 2001, 1986), 1},
		{"year wrong", yearScore(1986, 2001), 0},
		{"tracks equal", tracksScore(8, 8), 1},
		{"tracks partial", tracksScore(6, 12), 0.25},
		{"tracks more", tracksScore(12, 8), 0},
		{"durations positional", durationsScore([]int64{231500, 209000}, []int64{231000, 210000}), 1},
		{"durations half", durationsScore([]int64{231500, 100000}, []int64{231000, 210000}), 0.5},
		{"durations subset", durationsScore([]int64{210000}, []int64{231000, 210000}), 1},
		{"durations unknown", durationsScore([]int64{0, 0}, []int64{231000, 210000}), 0.5},
		{"artist various", artistScore(Variant{Various: true}, true, "Various Artists", nil), 1},
		{"artist various mismatch", artistScore(Variant{Artist: "Billy Blanco"}, true, "Various Artists", nil), 0.3},
		{"artist unknown", artistScore(Variant{}, false, "Soda Stereo", nil), 0.5},
		{"artist in credit", artistScore(Variant{Artist: "Noel Rosa"}, false, "Noel Rosa & Aracy de Almeida",
			[]string{"Noel Rosa", "Aracy de Almeida"}), 1},
		{"title without notes", titleScore(Variant{Title: "Signos (Remastered)"}, "Signos"), 1},
	}
	for _, c := range cases {
		if math.Abs(c.got-c.want) > 1e-9 {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestReleaseScore(t *testing.T) {
	q := Query{Year: 1986, Durations: []int64{231500, 209000}}
	v := Variant{Title: "Signos", Artist: "Soda Stereo"}
	rel := musicbrainz.Release{Title: "Signos", Artist: "Soda Stereo", ArtistNames: []string{"Soda Stereo"}, Year: 1986,
		Tracks: []musicbrainz.Track{{LengthMs: 231000}, {LengthMs: 210000}}}
	if s := releaseScore(v, q, rel); math.Abs(s-1) > 1e-9 {
		t.Fatalf("perfect release scores %v", s)
	}
	hit := musicbrainz.ReleaseHit{Title: "Signos", Artist: "Soda Stereo", Year: 1986, Tracks: 2}
	if s := hitScore(v, q, hit); math.Abs(s+durationsWeight-1) > 1e-9 {
		t.Fatalf("perfect hit scores %v", s)
	}
}
