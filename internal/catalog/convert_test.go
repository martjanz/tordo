package catalog

import (
	"reflect"
	"testing"

	"tordo/internal/store"
)

func TestToConvert(t *testing.T) {
	// Build a snapshot with formats to convert:
	// - Two WMA tracks of one album (sizes 5 and 7)
	// - One ALAC track (Format: "mp4", Codec: "alac") of another album
	// - One WAV track (Format: "wav", Codec: "pcm_s16le") loose in ../musica/sueltos
	// - One MP3 and one FLAC (NOT to convert)
	snap := store.Snapshot{Changes: 7, Artists: map[string]store.Artist{}}
	snap.Folders = []store.SnapFolder{
		{FolderSummary: store.FolderSummary{Path: "../musica", Name: "musica"}, Parent: ""},
		{FolderSummary: store.FolderSummary{Path: "../musica/Album1", Name: "Album1"}, Parent: "../musica"},
		{FolderSummary: store.FolderSummary{Path: "../musica/Album2", Name: "Album2"}, Parent: "../musica"},
		{FolderSummary: store.FolderSummary{Path: "../musica/sueltos", Name: "sueltos"}, Parent: "../musica"},
	}

	// Album 1: two WMA tracks (sizes 5 and 7)
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

	// Album 2: one ALAC track
	snap.Albums = append(snap.Albums, store.AlbumView{
		ID: 2, Title: "ALAC Album", Artist: "Artist2", Dir: "../musica/Album2",
		Tracks: 1, DurationMs: 200000,
	})

	snap.Tracks = append(snap.Tracks, store.SnapTrack{
		TrackView: store.TrackView{
			ID: 201, Title: "Track3", Artist: "Artist2", Format: "mp4", Codec: "alac",
			SizeBytes: 100, DurationMs: 200000, Lossless: true, Fingerprint: "fp201",
		},
		AlbumID: 2, Dir: "../musica/Album2", Added: 1002,
	})

	// Loose track: WAV in sueltos
	snap.Tracks = append(snap.Tracks, store.SnapTrack{
		TrackView: store.TrackView{
			ID: 301, Title: "Loose Track", Artist: "Artist3", Format: "wav", Codec: "pcm_s16le",
			SizeBytes: 50, DurationMs: 180000, Lossless: true, Fingerprint: "fp301",
		},
		AlbumID: 0, Dir: "../musica/sueltos", Added: 1003,
	})

	// Tracks NOT to convert: MP3 and FLAC
	snap.Albums = append(snap.Albums, store.AlbumView{
		ID: 3, Title: "Mixed Album", Artist: "Artist4", Dir: "../musica/Mixed",
		Tracks: 2, DurationMs: 300000,
	})
	snap.Folders = append(snap.Folders, store.SnapFolder{
		FolderSummary: store.FolderSummary{Path: "../musica/Mixed", Name: "Mixed"}, Parent: "../musica",
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
			SizeBytes: 2000, DurationMs: 150000, Lossless: true, Fingerprint: "fp402",
		},
		AlbumID: 3, Dir: "../musica/Mixed", Added: 1005,
	})

	c := Build(snap, Options{})
	got := c.ToConvert()

	// Expected: 4 files total (WMA + WMA + ALAC + WAV), bytes = 5 + 7 + 100 + 50 = 162
	if got.Files != 4 || got.Bytes != 162 {
		t.Errorf("ToConvert totals: got Files=%d Bytes=%d, want Files=4 Bytes=162", got.Files, got.Bytes)
	}

	// Check Formats: should have WMA, WAV, ALAC in ConvertKeys order
	wantFormats := []ConvertFormat{
		{Key: "wma", Label: "WMA", Files: 2, Bytes: 12},
		{Key: "wav", Label: "WAV", Files: 1, Bytes: 50},
		{Key: "alac", Label: "ALAC", Files: 1, Bytes: 100},
	}
	if !reflect.DeepEqual(got.Formats, wantFormats) {
		t.Errorf("ToConvert formats: got %+v, want %+v", got.Formats, wantFormats)
	}

	// Check Items: should have WMA album, WAV loose track, ALAC album (ordered by bytes desc, then title)
	// Expected order: ALAC (100), WMA (12), WAV (50) → sorted by bytes desc: ALAC (100), WAV (50), WMA (12)
	if len(got.Items) != 3 {
		t.Errorf("ToConvert items count: got %d, want 3", len(got.Items))
	}

	if len(got.Items) >= 3 {
		// First item: ALAC (100 bytes, largest)
		if got.Items[0].AlbumID != 2 || got.Items[0].Title != "ALAC Album" || got.Items[0].Format != "alac" ||
			got.Items[0].Files != 1 || got.Items[0].Bytes != 100 {
			t.Errorf("Items[0] (ALAC): got %+v", got.Items[0])
		}

		// Second item: WAV loose (50 bytes)
		if got.Items[1].AlbumID != 0 || got.Items[1].Title != "sueltos" || got.Items[1].Format != "wav" ||
			got.Items[1].Files != 1 || got.Items[1].Bytes != 50 {
			t.Errorf("Items[1] (WAV loose): got %+v", got.Items[1])
		}

		// Third item: WMA (12 bytes)
		if got.Items[2].AlbumID != 1 || got.Items[2].Title != "WMA Album" || got.Items[2].Format != "wma" ||
			got.Items[2].Files != 2 || got.Items[2].Bytes != 12 {
			t.Errorf("Items[2] (WMA): got %+v", got.Items[2])
		}
	}

	// Test empty catalog
	emptySnap := store.Snapshot{Changes: 0, Artists: map[string]store.Artist{}}
	emptySnap.Folders = []store.SnapFolder{
		{FolderSummary: store.FolderSummary{Path: "../musica", Name: "musica"}, Parent: ""},
	}
	empty := Build(emptySnap, Options{})
	emptyResult := empty.ToConvert()
	if emptyResult.Files != 0 || emptyResult.Bytes != 0 || len(emptyResult.Formats) != 0 || len(emptyResult.Items) != 0 {
		t.Errorf("Empty catalog: got %+v, want all zeros and empty non-nil slices", emptyResult)
	}
}
