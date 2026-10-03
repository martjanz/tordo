package catalog

import (
	"reflect"
	"testing"

	"tordo/internal/store"
)

func TestToConvert(t *testing.T) {
	snap := store.Snapshot{Changes: 7, Artists: map[string]store.Artist{}}
	snap.Folders = []store.SnapFolder{
		{FolderSummary: store.FolderSummary{Path: "../musica", Name: "musica"}, Parent: ""},
		{FolderSummary: store.FolderSummary{Path: "../musica/Album1", Name: "Album1"}, Parent: "../musica"},
		{FolderSummary: store.FolderSummary{Path: "../musica/Album2", Name: "Album2"}, Parent: "../musica"},
		{FolderSummary: store.FolderSummary{Path: "../musica/sueltos", Name: "sueltos"}, Parent: "../musica"},
		{FolderSummary: store.FolderSummary{Path: "../musica/Mixed", Name: "Mixed"}, Parent: "../musica"},
	}

	snap.Albums = append(snap.Albums, store.AlbumView{
		ID: 1, Title: "WMA Album", Artist: "Artist1", Dir: "../musica/Album1",
		Tracks: 2, DurationMs: 180000,
	})
	snap.Tracks = append(snap.Tracks, store.SnapTrack{
		TrackView: store.TrackView{
			ID: 101, Title: "Track1", Artist: "Artist1", Format: "asf", Codec: "wmav2",
			SizeBytes: 5, DurationMs: 90000, Fingerprint: "fp101",
		},
		AlbumID: 1, Dir: "../musica/Album1", Added: 1000,
	})
	snap.Tracks = append(snap.Tracks, store.SnapTrack{
		TrackView: store.TrackView{
			ID: 102, Title: "Track2", Artist: "Artist1", Format: "asf", Codec: "wmav2",
			SizeBytes: 7, DurationMs: 90000, Fingerprint: "fp102",
		},
		AlbumID: 1, Dir: "../musica/Album1", Added: 1001,
	})

	snap.Albums = append(snap.Albums, store.AlbumView{
		ID: 2, Title: "ALAC Album", Artist: "Artist2", Dir: "../musica/Album2",
		Tracks: 1, DurationMs: 200000,
	})
	snap.Tracks = append(snap.Tracks, store.SnapTrack{
		TrackView: store.TrackView{
			ID: 201, Title: "Track3", Artist: "Artist2", Format: "mp4", Codec: "alac",
			SizeBytes: 10, DurationMs: 200000, Fingerprint: "fp201",
		},
		AlbumID: 2, Dir: "../musica/Album2", Added: 1002,
	})

	snap.Tracks = append(snap.Tracks, store.SnapTrack{
		TrackView: store.TrackView{
			ID: 301, Title: "Loose Track", Artist: "Artist3", Format: "wav", Codec: "pcm_s16le",
			SizeBytes: 8, DurationMs: 180000, Fingerprint: "fp301",
		},
		AlbumID: 0, Dir: "../musica/sueltos", Added: 1003,
	})

	snap.Albums = append(snap.Albums, store.AlbumView{
		ID: 3, Title: "Mixed Album", Artist: "Artist4", Dir: "../musica/Mixed",
		Tracks: 2, DurationMs: 300000,
	})
	snap.Tracks = append(snap.Tracks, store.SnapTrack{
		TrackView: store.TrackView{
			ID: 401, Title: "MP3 Track", Artist: "Artist4", Format: "mp3", Codec: "mp3",
			SizeBytes: 1000, DurationMs: 150000, Fingerprint: "fp401",
		},
		AlbumID: 3, Dir: "../musica/Mixed", Added: 1004,
	})
	snap.Tracks = append(snap.Tracks, store.SnapTrack{
		TrackView: store.TrackView{
			ID: 402, Title: "FLAC Track", Artist: "Artist4", Format: "flac", Codec: "flac",
			SizeBytes: 2000, DurationMs: 150000, Fingerprint: "fp402",
		},
		AlbumID: 3, Dir: "../musica/Mixed", Added: 1005,
	})

	c := Build(snap, Options{})
	got := c.ToConvert()

	if got.Files != 4 || got.Bytes != 30 {
		t.Errorf("ToConvert totals: got Files=%d Bytes=%d, want Files=4 Bytes=30", got.Files, got.Bytes)
	}

	wantFormats := []ConvertFormat{
		{Key: "wma", Label: "WMA", Files: 2, Bytes: 12},
		{Key: "wav", Label: "WAV", Files: 1, Bytes: 8},
		{Key: "alac", Label: "ALAC", Files: 1, Bytes: 10},
	}
	if !reflect.DeepEqual(got.Formats, wantFormats) {
		t.Errorf("ToConvert formats: got %+v, want %+v", got.Formats, wantFormats)
	}

	if len(got.Items) != 3 {
		t.Errorf("ToConvert items count: got %d, want 3", len(got.Items))
	}
	if len(got.Items) >= 3 {
		if got.Items[0].AlbumID != 1 || got.Items[0].Title != "WMA Album" || got.Items[0].Format != "wma" ||
			got.Items[0].Files != 2 || got.Items[0].Bytes != 12 {
			t.Errorf("Items[0] (WMA): got %+v", got.Items[0])
		}
		if got.Items[1].AlbumID != 2 || got.Items[1].Title != "ALAC Album" || got.Items[1].Format != "alac" ||
			got.Items[1].Files != 1 || got.Items[1].Bytes != 10 {
			t.Errorf("Items[1] (ALAC): got %+v", got.Items[1])
		}
		if got.Items[2].AlbumID != 0 || got.Items[2].Title != "sueltos" || got.Items[2].Format != "wav" ||
			got.Items[2].Files != 1 || got.Items[2].Bytes != 8 {
			t.Errorf("Items[2] (WAV loose): got %+v", got.Items[2])
		}
	}

	emptySnap := store.Snapshot{Changes: 0, Artists: map[string]store.Artist{}}
	emptySnap.Folders = []store.SnapFolder{
		{FolderSummary: store.FolderSummary{Path: "../musica", Name: "musica"}, Parent: ""},
	}
	empty := Build(emptySnap, Options{})
	emptyResult := empty.ToConvert()
	if emptyResult.Files != 0 || emptyResult.Bytes != 0 || emptyResult.Formats == nil || emptyResult.Items == nil ||
		len(emptyResult.Formats) != 0 || len(emptyResult.Items) != 0 {
		t.Errorf("Empty catalog: got %+v, want all zeros and non-nil empty slices", emptyResult)
	}

	testToConvertTies(t, snap)
}

func testToConvertTies(t *testing.T, baseSnap store.Snapshot) {
	snap := baseSnap
	snap.Albums = snap.Albums[:0]
	snap.Tracks = snap.Tracks[:0]
	snap.Folders = []store.SnapFolder{
		{FolderSummary: store.FolderSummary{Path: "../musica", Name: "musica"}, Parent: ""},
		{FolderSummary: store.FolderSummary{Path: "../musica/Ties", Name: "Ties"}, Parent: "../musica"},
	}

	snap.Albums = append(snap.Albums, store.AlbumView{
		ID: 100, Title: "Ties Album", Artist: "TieArtist", Dir: "../musica/Ties",
		Tracks: 2, DurationMs: 300000,
	})
	snap.Tracks = append(snap.Tracks, store.SnapTrack{
		TrackView: store.TrackView{
			ID: 1001, Title: "WMA Tie", Artist: "TieArtist", Format: "asf", Codec: "wmav2",
			SizeBytes: 100, DurationMs: 150000, Fingerprint: "fpwma",
		},
		AlbumID: 100, Dir: "../musica/Ties", Added: 2000,
	})
	snap.Tracks = append(snap.Tracks, store.SnapTrack{
		TrackView: store.TrackView{
			ID: 1002, Title: "WAV Tie", Artist: "TieArtist", Format: "wav", Codec: "pcm_s16le",
			SizeBytes: 100, DurationMs: 150000, Fingerprint: "fpwav",
		},
		AlbumID: 100, Dir: "../musica/Ties", Added: 2001,
	})

	c := Build(snap, Options{})
	got := c.ToConvert()

	if len(got.Items) != 2 {
		t.Fatalf("Ties test: got %d items, want 2", len(got.Items))
	}

	if got.Items[0].AlbumID != 100 || got.Items[0].Format != "wma" {
		t.Errorf("Ties[0]: got format %q, want wma (wma before wav in ConvertKeys)", got.Items[0].Format)
	}
	if got.Items[1].AlbumID != 100 || got.Items[1].Format != "wav" {
		t.Errorf("Ties[1]: got format %q, want wav", got.Items[1].Format)
	}
}
