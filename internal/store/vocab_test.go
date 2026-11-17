package store

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestUpgradeStationDef(t *testing.T) {
	old := `{"name":"x","source":{"view":"pistas","facets":{"megusta":"si","calidad":"sin-perdida","decada":"1980"}},"mode":"albumes","artistGap":3}`
	got, changed := upgradeStationDef([]byte(old))
	if !changed {
		t.Fatal("changed = false")
	}
	var a, b any
	if err := json.Unmarshal(got, &a); err != nil {
		t.Fatal(err)
	}
	json.Unmarshal([]byte(`{"name":"x","source":{"view":"tracks","facets":{"liked":"yes","quality":"lossless","decade":"1980"}},"mode":"albums","artistGap":3}`), &b)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("got %s", got)
	}

	for _, in := range []string{
		`{"source":{"view":"tracks","facets":{"liked":"yes","listened":"year-ago"}},"mode":"tracks"}`,
		`not json`, `[]`, `{"source":{"folder":"Brasil"}}`,
		`{"source":{"facets":{"genre":"si","artist":"nunca"}}}`,
	} {
		out, changed := upgradeStationDef([]byte(in))
		if changed || string(out) != in {
			t.Errorf("%s -> %s, %v", in, out, changed)
		}
	}
}

func TestUpgradeStationDefValues(t *testing.T) {
	got, changed := upgradeStationDef([]byte(`{"source":{"view":"artistas","facets":{"estado":"con-duplicados","escucha":"hace-un-anio","puntaje":"4","pais":"AR"}}}`))
	if !changed {
		t.Fatal("not changed")
	}
	var d struct {
		Source struct {
			View   string
			Facets map[string]string
		}
	}
	json.Unmarshal(got, &d)
	want := map[string]string{"state": "duplicated", "listened": "year-ago", "rating": "4", "country": "AR"}
	if d.Source.View != "artists" || !reflect.DeepEqual(d.Source.Facets, want) {
		t.Fatalf("got %s", got)
	}
}
