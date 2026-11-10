package store

import (
	"fmt"
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

// pendingOracle is LoudnessPending and LoudnessPendingCount computed from
// scratch, the way they were before the pending list was kept.
func pendingOracle(t *testing.T, s *Store, limit int) ([]loudness.Job, int) {
	t.Helper()
	jobs, err := s.loudnessJobs(oneJobPerRef("", `ORDER BY mx DESC, id DESC LIMIT ?`), limit)
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(DISTINCT ` + trackRef + `) ` + needsMeasure).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return jobs, n
}

func checkPending(t *testing.T, s *Store, limit int) []loudness.Job {
	t.Helper()
	wantJobs, wantN := pendingOracle(t, s, limit)
	jobs, err := s.LoudnessPending(limit)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(jobs, wantJobs) {
		t.Fatalf("pending = %+v\nwant %+v", jobs, wantJobs)
	}
	n, err := s.LoudnessPendingCount()
	if err != nil {
		t.Fatal(err)
	}
	if n != wantN {
		t.Fatalf("count = %d, want %d", n, wantN)
	}
	return jobs
}

// addPendingFiles adds n present tracks with no gain tag, fingerprints
// "bulkN" and first_seen n-i, so they come in insertion order.
func addPendingFiles(t *testing.T, s *Store, n int) []string {
	t.Helper()
	var refs []string
	for i := range n {
		fp := fmt.Sprintf("bulk%d", i)
		res, err := s.db.Exec(`INSERT INTO files (path, size, mtime, kind, fingerprint, missing, first_seen)
			VALUES (?, 1, 1, 'audio', ?, 0, ?)`, fmt.Sprintf("../bulk/%04d.mp3", i), fp, 10_000+n-i)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := res.LastInsertId()
		if _, err := s.db.Exec(`INSERT INTO audio (file_id, size, mtime, read_version, with_ffprobe, tags)
			VALUES (?, 1, 1, 1, 0, '{}')`, id); err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.Exec(`INSERT INTO tracks (file_id, dir, title, artist, disc, track, year, duration_ms)
			VALUES (?, '../bulk', 't', 'a', 1, 1, 0, 1000)`, id); err != nil {
			t.Fatal(err)
		}
		refs = append(refs, fp)
	}
	return refs
}

func TestLoudnessPendingFollowsSavesAndCatalogChanges(t *testing.T) {
	s := loudnessLibrary(t)
	for path, fs := range map[string]int{"../musica/suelto.mp3": 500, signos + "02.mp3": 300, signos + "01.mp3": 100} {
		if _, err := s.db.Exec(`UPDATE files SET first_seen = ? WHERE path = ?`, fs, path); err != nil {
			t.Fatal(err)
		}
	}
	checkPending(t, s, 1)
	if again := checkPending(t, s, 1); again[0].Path != "../musica/suelto.mp3" {
		t.Fatalf("an unsaved job is listed again: %+v", again)
	}
	v := -9.0
	save := func(ref string, lufs *float64) {
		t.Helper()
		if err := s.SaveLoudness(ref, lufs, ""); err != nil {
			t.Fatal(err)
		}
	}
	save("fp"+signos+"01.mp3", &v) // the last of the list, measured first (as Prioritize does)
	checkPending(t, s, 10)
	save("fp"+signos+"01.mp3", nil) // measured again: still one track less
	save("fp"+la+"/[LA-11] Virus - Locura/01.mp3", &v) // not pending: it has a tag
	save("nada", &v)
	checkPending(t, s, 10)

	if _, err := s.db.Exec(`UPDATE files SET missing = 1 WHERE path = '../musica/suelto.mp3'`); err != nil {
		t.Fatal(err)
	}
	checkPending(t, s, 10)
	if _, err := s.db.Exec(`DELETE FROM loudness WHERE ref = ?`, "fp"+signos+"01.mp3"); err != nil {
		t.Fatal(err)
	}
	checkPending(t, s, 10)
	if _, err := s.db.Exec(`UPDATE audio SET tags = '{"trackGain":-3}' WHERE file_id = (SELECT id FROM files WHERE path = ?)`, signos+"02.mp3"); err != nil {
		t.Fatal(err)
	}
	if left := checkPending(t, s, 10); len(left) != 1 || left[0].Path != signos+"01.mp3" {
		t.Fatalf("left = %+v", left)
	}
}

func TestLoudnessPendingManyMeasured(t *testing.T) {
	s := loudnessLibrary(t)
	refs := addPendingFiles(t, s, 1500)
	checkPending(t, s, 20)
	v := -9.0
	// Measured from the head of the list past one chunk, and a few further on.
	for _, i := range []int{0, 1, 2, 3, 700, 701, 1499} {
		if err := s.SaveLoudness(refs[i], &v, ""); err != nil {
			t.Fatal(err)
		}
	}
	for i := 4; i < 650; i++ {
		if err := s.SaveLoudness(refs[i], &v, ""); err != nil {
			t.Fatal(err)
		}
	}
	checkPending(t, s, 20)
	for i := 650; i < 1500; i++ {
		if err := s.SaveLoudness(refs[i], &v, ""); err != nil {
			t.Fatal(err)
		}
		if i%97 == 0 {
			checkPending(t, s, 20)
		}
	}
	if jobs := checkPending(t, s, 20); len(jobs) != 3 {
		t.Fatalf("left = %+v, want the 3 of the library", jobs)
	}
}

func TestLoudnessPendingRefByID(t *testing.T) {
	s := loudnessLibrary(t)
	var id int64
	if err := s.db.QueryRow(`SELECT id FROM files WHERE path = '../musica/suelto.mp3'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE files SET fingerprint = '' WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	checkPending(t, s, 10)
	v := -9.0
	if err := s.SaveLoudness(fmt.Sprintf("id:%d", id), &v, ""); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.LoudnessPendingCount(); n != 2 {
		t.Fatalf("count = %d, want 2", n)
	}
	checkPending(t, s, 10)
}
