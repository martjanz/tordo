package search

import (
	"reflect"
	"testing"
)

func index(t *testing.T, docs ...Doc) *Index {
	t.Helper()
	x, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { x.Close() })
	if err := x.Refresh(1, func() []Doc { return docs }); err != nil {
		t.Fatal(err)
	}
	return x
}

func TestQuery(t *testing.T) {
	x := index(t,
		Doc{Key: "track:1", Name: "Fito", Context: "Fito Páez · Circo Beat"},
		Doc{Key: "artist:fito", Name: "Fito Páez"},
		Doc{Key: "album:2", Name: "Circo Beat", Context: "Fito Páez"},
		Doc{Key: "folder:x", Name: "Latinoamérica 600"}, // decomposed, as macOS writes names
	)
	for _, c := range []struct {
		q    string
		want []string
	}{
		{"fito paez", []string{"artist:fito", "track:1", "album:2"}}, // the name weighs more than the context
		{"PÁEZ circ", []string{"album:2", "track:1"}},                // the last word is a prefix
		{"latinoamérica", []string{"folder:x"}},
		{"latinoamér", []string{"folder:x"}},
		{"f", nil}, // too short
		{"!!", nil},
		{"zzz", nil},
	} {
		got, err := x.Query(c.q)
		if err != nil {
			t.Fatalf("%q: %v", c.q, err)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q = %q, want %q", c.q, got, c.want)
		}
	}
}

func TestRefreshOnlyWhenChanged(t *testing.T) {
	x := index(t, Doc{Key: "a", Name: "Signos"})
	calls := 0
	x.Refresh(1, func() []Doc { calls++; return nil })
	if calls != 0 {
		t.Fatal("rebuilt without changes")
	}
	x.Refresh(2, func() []Doc { calls++; return []Doc{{Key: "b", Name: "Doble vida"}} })
	if got, _ := x.Query("signos"); calls != 1 || got != nil {
		t.Fatalf("after a change: calls %d, signos %q", calls, got)
	}
	x.Refresh(1, func() []Doc { calls++; return nil }) // a slow request with an older catalog
	if got, _ := x.Query("doble"); calls != 1 || len(got) != 1 {
		t.Fatalf("an older catalog replaced the index: calls %d, doble %q", calls, got)
	}
}
