package genres

import (
	"reflect"
	"testing"
)

const fwc = "Folk, World, & Country"

func TestMap(t *testing.T) {
	n := New(nil)
	cases := []struct {
		raw  string
		want []Mapping
		ok   bool
	}{
		{"Rock", []Mapping{{Genre: "Rock"}}, true},
		{"rock nacional", []Mapping{{Genre: "Rock"}}, true},
		{"Samba", []Mapping{{Genre: "Latin", Style: "Samba"}}, true},
		{"Bossa Nova", []Mapping{{Genre: "Latin", Style: "Bossanova"}}, true},
		{"Folclore", []Mapping{{Genre: fwc, Style: "Folk"}}, true},
		{"Latino", []Mapping{{Genre: "Latin"}}, true},
		{"Latin, Folk, World, & Country", []Mapping{{Genre: "Latin"}, {Genre: fwc}}, true},
		{"Pop Pop Internacional", []Mapping{{Genre: "Pop"}}, true},
		{"Rock/Pop", []Mapping{{Genre: "Rock"}, {Genre: "Pop"}}, true},
		{"Funk / Soul", []Mapping{{Genre: "Funk / Soul"}}, true},
		{"murga", []Mapping{{Genre: "Latin", Style: "Murga"}}, true},
		{"Pop Rock", []Mapping{{Genre: "Rock", Style: "Pop Rock"}}, true},
		{"Canciones de Cuna", []Mapping{{Genre: "Children's", Style: "Nursery Rhymes"}}, true},
		{"Progressive Rock", []Mapping{{Genre: "Rock", Style: "Prog Rock"}}, true},
		{"Psych", []Mapping{{Genre: "Rock", Style: "Psychedelic Rock"}}, true},
		{"Scores de Películas", []Mapping{{Genre: "Stage & Screen", Style: "Score"}}, true},
		{"Poesía", []Mapping{{Genre: "Non-Music", Style: "Poetry"}}, true},
		{"Tango Cancion", []Mapping{{Genre: "Latin", Style: "Tango"}}, true},
		{"Tango Latino", []Mapping{{Genre: "Latin", Style: "Tango"}}, true},
		{"Música popular latinoamericana", []Mapping{{Genre: "Latin"}}, true},
		{"Género desconocido", nil, true},
		{"Uruguay", nil, true},
		{"Other", nil, true},
		{"(12)", nil, true},
		{"", nil, true},
		{"Chamamé", nil, false},
		{"Rock, Chamamé", []Mapping{{Genre: "Rock"}}, false},
	}
	for _, c := range cases {
		got, ok := n.Map(c.raw)
		if ok != c.ok || !reflect.DeepEqual(got, c.want) {
			t.Errorf("Map(%q) = %+v, %v; want %+v, %v", c.raw, got, ok, c.want, c.ok)
		}
	}
}

func TestUserAliasesWin(t *testing.T) {
	n := New(map[string]Mapping{
		Key("Chamamé"): {Genre: fwc, Style: "Folk"},
		Key("Rock"):    {Genre: "Latin"},
		Key("Basura"):  {},
	})
	if got, ok := n.Map("chamame"); !ok || !reflect.DeepEqual(got, []Mapping{{Genre: fwc, Style: "Folk"}}) {
		t.Errorf("chamame = %+v, %v", got, ok)
	}
	if got, _ := n.Map("Rock"); !reflect.DeepEqual(got, []Mapping{{Genre: "Latin"}}) {
		t.Errorf("rock = %+v", got)
	}
	if got, ok := n.Map("basura"); !ok || got != nil {
		t.Errorf("basura = %+v, %v", got, ok)
	}
}

func TestKey(t *testing.T) {
	if Key("Pop  Pop Internacional!") != "pop internacional" || Key("Folclóre") != "folclore" {
		t.Fatalf("Key: %q %q", Key("Pop  Pop Internacional!"), Key("Folclóre"))
	}
}

func TestFromDiscogs(t *testing.T) {
	got := FromDiscogs([]string{"Rock", "Latin"}, []string{"Pop Rock", "Samba", "Rarísimo"})
	want := []Mapping{{Genre: "Rock", Style: "Pop Rock"}, {Genre: "Latin", Style: "Samba"}, {Genre: "Rock", Style: "Rarísimo"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	got = FromDiscogs([]string{"Jazz", "Latin"}, []string{"Bossanova"})
	want = []Mapping{{Genre: "Latin", Style: "Bossanova"}, {Genre: "Jazz"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	if FromDiscogs(nil, []string{"Samba"}) != nil {
		t.Fatal("styles without genres")
	}
}

func TestValid(t *testing.T) {
	if !Valid("Latin", "") || !Valid("Latin", "Samba") || !Valid(fwc, "Folk") {
		t.Fatal("valid values rejected")
	}
	if Valid("Latin", "Pop Rock") || Valid("Cumbia", "") || Valid("", "") {
		t.Fatal("invalid values accepted")
	}
}

func TestTaxonomyKeysAreUnique(t *testing.T) {
	seen := map[string]string{}
	for _, g := range Taxonomy {
		for _, name := range append([]string{g.Name}, g.Styles...) {
			if prev, ok := seen[Key(name)]; ok {
				t.Errorf("%q and %q share the key %q", prev, name, Key(name))
			}
			seen[Key(name)] = name
		}
	}
}
