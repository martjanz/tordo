package identify

import (
	"reflect"
	"testing"

	"tordo/internal/genres"
	"tordo/internal/grouping"
	"tordo/internal/musicbrainz"
	"tordo/internal/store"
)

const signosDir = "../musica/Soda Stereo/Soda Stereo - (1986) Signos"

func target() store.IdentifyTarget {
	return store.IdentifyTarget{AlbumID: 7, Dir: signosDir, Title: "Signos", Artist: "Soda Stereo", Year: 1986,
		RawGenres: []string{"Rock nacional", "Chamamé"},
		Tracks: []store.TargetTrack{
			{FileID: 1, Fingerprint: "fp-1", Title: "Tactica y estrategia", Disc: 1, Track: 1, DurationMs: 231500},
			{FileID: 2, Fingerprint: "fp-2", Title: "Sin sobresalt", Disc: 1, Track: 2, DurationMs: 209000},
		}}
}

func newWorld() world {
	return world{matches: map[string]store.Match{}, corrections: map[string]store.Correction{},
		releases: map[string]store.Release{}, artists: map[string]store.Artist{}, genres: genres.New(nil)}
}

func autoMatch(w world, tg store.IdentifyTarget) {
	k := BuildQuery(tg).Key()
	w.matches[k] = store.Match{Query: k, Status: store.StatusAuto, ReleaseID: signosID, Confidence: 0.97}
}

func TestResolveAutoMatch(t *testing.T) {
	tg := target()
	q := BuildQuery(tg)
	rel := releaseFrom(signosRelease())
	rel.Discogs = store.DiscogsInfo{Done: true, Genres: []string{"Rock"}, Styles: []string{"Pop Rock"}}
	rel.Cover = "cache/covers/" + signosID + ".jpg"
	w := newWorld()
	autoMatch(w, tg)
	w.releases[signosID] = rel
	w.artists[sodaID] = store.Artist{ID: sodaID, Country: "AR"}
	unmapped := map[string]int{}
	got := resolve(tg, q, w, unmapped)
	want := store.AlbumIdentity{AlbumID: 7, Query: q.Key(), Status: store.StatusAuto, ReleaseID: signosID, Confidence: 0.97,
		Title: "Signos", Artist: "Soda Stereo", OriginalYear: 1986, Country: "AR", ArtistID: sodaID, CoverCache: rel.Cover,
		Genres: []store.GenreLink{{Genre: "Rock", Style: "Pop Rock", Source: "discogs"}},
		Tracks: []store.TrackIdentity{{FileID: 1, Title: "Táctica y estrategia", RecordingID: "rec-1"},
			{FileID: 2, Title: "Sin sobresaltos", RecordingID: "rec-2"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
	if len(unmapped) != 0 {
		t.Fatalf("unmapped = %v", unmapped)
	}
}

func TestResolveCorrectionWins(t *testing.T) {
	tg := target()
	w := newWorld()
	autoMatch(w, tg)
	manual := store.Correction{Status: store.StatusManual, ReleaseID: otherID}
	w.corrections["fp-1"], w.corrections["fp-2"] = manual, manual
	unmapped := map[string]int{}
	got := resolve(tg, BuildQuery(tg), w, unmapped)
	if got.Status != store.StatusManual || got.ReleaseID != otherID || got.Confidence != 1 || got.Title != "" {
		t.Fatalf("got %+v", got)
	}
	// The release is not stored yet: genres still come from the tags.
	if want := []store.GenreLink{{Genre: "Rock", Source: "tag"}}; !reflect.DeepEqual(got.Genres, want) {
		t.Fatalf("genres = %+v", got.Genres)
	}
	if !reflect.DeepEqual(unmapped, map[string]int{"Chamamé": 1}) {
		t.Fatalf("unmapped = %v", unmapped)
	}
}

func TestResolveMinorityCorrectionIgnored(t *testing.T) {
	tg := target()
	tg.Tracks = append(tg.Tracks, store.TargetTrack{FileID: 3, Fingerprint: "fp-3", Title: "Prófugos", Disc: 1, Track: 3})
	w := newWorld()
	k := BuildQuery(tg).Key()
	w.matches[k] = store.Match{Query: k, Status: store.StatusUnmatched, Confidence: 0.4}
	w.corrections["fp-1"] = store.Correction{Status: store.StatusIgnored}
	if got := resolve(tg, BuildQuery(tg), w, map[string]int{}); got.Status != store.StatusUnmatched {
		t.Fatalf("status = %q", got.Status)
	}
}

func TestResolvePendingAndSelection(t *testing.T) {
	tg := target()
	w := newWorld()
	if got := resolve(tg, BuildQuery(tg), w, map[string]int{}); got.Status != store.StatusPending {
		t.Fatalf("status = %q", got.Status)
	}
	k := BuildQuery(tg).Key()
	w.matches[k] = store.Match{Query: k, Status: store.StatusUnmatched}
	tg.Selection = true
	if got := resolve(tg, BuildQuery(tg), w, map[string]int{}); got.Status != store.StatusSelection {
		t.Fatalf("status = %q", got.Status)
	}
}

func TestResolveVariousArtists(t *testing.T) {
	tg := target()
	w := newWorld()
	autoMatch(w, tg)
	rel := releaseFrom(signosRelease())
	rel.Various, rel.ArtistIDs, rel.Artist, rel.Country = true, []string{musicbrainz.VariousArtistsID}, "Various Artists", "BR"
	w.releases[signosID] = rel
	got := resolve(tg, BuildQuery(tg), w, map[string]int{})
	if got.Artist != grouping.Various || !got.Compilation || got.Country != "BR" || got.ArtistID != "" {
		t.Fatalf("got %+v", got)
	}
	rel.Country = "XW"
	w.releases[signosID] = rel
	if got := resolve(tg, BuildQuery(tg), w, map[string]int{}); got.Country != "" {
		t.Fatalf("worldwide release gave country %q", got.Country)
	}
}

func TestResolveKeepsLocalCoverAndUsesMBGenres(t *testing.T) {
	tg := target()
	tg.HasCover = true
	w := newWorld()
	autoMatch(w, tg)
	rel := releaseFrom(signosRelease())
	rel.Cover = "cache/covers/x.jpg"
	rel.MBGenres = []string{"rock", "new wave", "singer-songwriter"}
	w.releases[signosID] = rel
	unmapped := map[string]int{}
	got := resolve(tg, BuildQuery(tg), w, unmapped)
	want := []store.GenreLink{{Genre: "Rock", Source: "mb"}, {Genre: "Rock", Style: "New Wave", Source: "mb"}}
	if got.CoverCache != "" || !reflect.DeepEqual(got.Genres, want) || len(unmapped) != 0 {
		t.Fatalf("cover %q, genres %+v, unmapped %v", got.CoverCache, got.Genres, unmapped)
	}
}

func TestAlignTracks(t *testing.T) {
	rts := releaseFrom(signosRelease()).Tracks
	// Fewer tracks than the release: paired by duration.
	got := alignTracks([]store.TargetTrack{{FileID: 9, Title: "Sin sobresalt", DurationMs: 209000}}, rts)
	if want := []store.TrackIdentity{{FileID: 9, Title: "Sin sobresaltos", RecordingID: "rec-2"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("by duration: %+v", got)
	}
	// Unknown duration: paired by title.
	got = alignTracks([]store.TargetTrack{{FileID: 9, Title: "Tactica y estrategia"}}, rts)
	if len(got) != 1 || got[0].RecordingID != "rec-1" {
		t.Fatalf("by title: %+v", got)
	}
	// Same count but a duration far off: that track is left alone.
	got = alignTracks([]store.TargetTrack{{FileID: 1, DurationMs: 231000}, {FileID: 2, DurationMs: 400000}}, rts)
	if len(got) != 1 || got[0].FileID != 1 {
		t.Fatalf("positional: %+v", got)
	}
}
