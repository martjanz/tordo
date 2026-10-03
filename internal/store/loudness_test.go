package store

import (
	"slices"
	"testing"

	"tordo/internal/audiofile"
	"tordo/internal/loudness"
)

const signos = la + "/[LA-10] Soda Stereo - Signos/"

func loudnessLibrary(t *testing.T) *Store {
	t.Helper()
	g := -6.5
	return catalog(t, map[string]audiofile.Tags{
		signos + "01.mp3":                     {Title: "Signos", Artist: "Soda Stereo", Album: "Signos", Track: 1},
		signos + "02.mp3":                     {Title: "No existes", Artist: "Soda Stereo", Album: "Signos", Track: 2},
		la + "/[LA-11] Virus - Locura/01.mp3": {Title: "Una luna", Artist: "Virus", Album: "Locura", Track: 1, TrackGain: &g},
		"../musica/suelto.mp3":                {Title: "Suelto"},
	})
}

func TestLoudnessPending(t *testing.T) {
	s := loudnessLibrary(t)
	jobs, err := s.LoudnessPending(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 3 {
		t.Fatalf("jobs = %+v, want 3 (the tagged track is left out)", jobs)
	}
	for _, j := range jobs {
		if j.Ref != "fp"+j.Path || j.ID == 0 {
			t.Fatalf("job = %+v", j)
		}
		if j.Path == la+"/[LA-11] Virus - Locura/01.mp3" {
			t.Fatalf("tagged track listed: %+v", j)
		}
	}
	if n, _ := s.LoudnessPendingCount(); n != 3 {
		t.Fatalf("count = %d", n)
	}
	if one, _ := s.LoudnessPending(1); len(one) != 1 {
		t.Fatalf("limit: %+v", one)
	}

	c0, _ := s.Changes()
	p0 := s.PersonalVersion()
	v := -10.0
	if err := s.SaveLoudness(jobs[0].Ref, &v, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveLoudness(jobs[1].Ref, nil, "roto"); err != nil {
		t.Fatal(err)
	}
	if c, _ := s.Changes(); c != c0 {
		t.Fatal("SaveLoudness changed the catalog counter")
	}
	if s.PersonalVersion() != p0 {
		t.Fatal("SaveLoudness changed the personal version")
	}
	left, _ := s.LoudnessPending(10)
	if len(left) != 1 || left[0].Ref != jobs[2].Ref {
		t.Fatalf("left = %+v", left)
	}
	if n, _ := s.LoudnessPendingCount(); n != 1 {
		t.Fatalf("count = %d", n)
	}
}

func TestLoudnessJobsAndFacts(t *testing.T) {
	s := loudnessLibrary(t)
	items, _ := s.PlayAlbum(albumNamed(t, s, "Signos"))
	signos1, signos2 := items[0], items[1]
	virus := playIDOf(t, s, "Una luna")
	other := playIDOf(t, s, "Suelto")
	v := -10.0
	if err := s.SaveLoudness("fp"+signos+"01.mp3", &v, ""); err != nil {
		t.Fatal(err)
	}
	jobs, err := s.LoudnessJobs([]int64{virus, signos1.ID, other})
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].ID != other {
		t.Fatalf("jobs = %+v, want only the loose one", jobs)
	}

	facts, err := s.LoudnessFacts([]int64{signos1.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 2 {
		t.Fatalf("facts = %+v", facts)
	}
	f1, f2 := facts[signos1.ID], facts[signos2.ID]
	if f1.AlbumID != signos1.AlbumID || f1.AlbumID == 0 || f1.DurationMs != 1000 {
		t.Fatalf("f1 = %+v", f1)
	}
	if f1.LUFS == nil || *f1.LUFS != -10 || f2.LUFS != nil {
		t.Fatalf("lufs: %v %v", f1.LUFS, f2.LUFS)
	}
	vf, _ := s.LoudnessFacts([]int64{virus})
	if g := vf[virus].TagTrack; g == nil || *g != -6.5 {
		t.Fatalf("tag = %v", g)
	}
}

func TestLoudnessPendingNewestFirst(t *testing.T) {
	s := loudnessLibrary(t)
	if _, err := s.db.Exec(`UPDATE files SET first_seen = 100`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE files SET first_seen = 500 WHERE path = '../musica/suelto.mp3'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE files SET first_seen = 300 WHERE path = ?`, signos+"02.mp3"); err != nil {
		t.Fatal(err)
	}
	jobs, _ := s.LoudnessPending(10)
	var got []string
	for _, j := range jobs {
		got = append(got, j.Path)
	}
	want := []string{"../musica/suelto.mp3", signos + "02.mp3", signos + "01.mp3"}
	if !slices.Equal(got, want) {
		t.Fatalf("order = %q", got)
	}
}

func TestLoudnessJobOfDuplicatesIsLowestID(t *testing.T) {
	s := loudnessLibrary(t)
	// Two present files with the same audio (same fingerprint); the higher id
	// is the newer one.
	if _, err := s.db.Exec(`UPDATE files SET first_seen = 1`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE files SET fingerprint = 'dup' WHERE path IN (?, ?)`, signos+"01.mp3", signos+"02.mp3"); err != nil {
		t.Fatal(err)
	}
	var lowID, highID int64
	var lowPath string
	if err := s.db.QueryRow(`SELECT id, path FROM files WHERE fingerprint = 'dup' ORDER BY id LIMIT 1`).Scan(&lowID, &lowPath); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`SELECT id FROM files WHERE fingerprint = 'dup' ORDER BY id DESC LIMIT 1`).Scan(&highID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE files SET first_seen = 99 WHERE id = ?`, highID); err != nil {
		t.Fatal(err)
	}
	check := func(jobs []loudness.Job) {
		t.Helper()
		n := 0
		for _, j := range jobs {
			if j.Ref == "dup" {
				n++
				if j.ID != lowID || j.Path != lowPath {
					t.Fatalf("job = %+v, want id %d and its own path", j, lowID)
				}
			}
		}
		if n != 1 {
			t.Fatalf("dup listed %d times: %+v", n, jobs)
		}
	}
	pending, err := s.LoudnessPending(10)
	if err != nil {
		t.Fatal(err)
	}
	check(pending)
	if pending[0].Ref != "dup" {
		t.Fatalf("newest ref (first_seen 99) should lead: %+v", pending)
	}
	jobs, err := s.LoudnessJobs([]int64{highID}) // asked through the higher id
	if err != nil {
		t.Fatal(err)
	}
	check(jobs)
}

func TestLoudnessChunks(t *testing.T) {
	ids := make([]int64, 1201)
	got := chunks(ids)
	if len(got) != 3 || len(got[0]) != 500 || len(got[2]) != 201 {
		t.Fatalf("chunks = %d", len(got))
	}
}

func playIDOf(t *testing.T, s *Store, title string) int64 {
	t.Helper()
	items, err := s.PlayFolder("../musica")
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.Title == title {
			return it.ID
		}
	}
	t.Fatalf("no track %q", title)
	return 0
}

func TestLoudnessFactsFailed(t *testing.T) {
	s := loudnessLibrary(t)
	items, _ := s.PlayAlbum(albumNamed(t, s, "Signos"))
	signos1, signos2 := items[0], items[1]
	v := -10.0
	if err := s.SaveLoudness("fp"+signos+"01.mp3", &v, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveLoudness("fp"+signos+"02.mp3", nil, "roto"); err != nil {
		t.Fatal(err)
	}
	facts, err := s.LoudnessFacts([]int64{signos1.ID})
	if err != nil {
		t.Fatal(err)
	}
	if facts[signos1.ID].Failed || !facts[signos2.ID].Failed {
		t.Fatalf("failed: %v %v", facts[signos1.ID].Failed, facts[signos2.ID].Failed)
	}
}
