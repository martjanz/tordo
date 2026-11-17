package identify

import (
	"context"
	"reflect"
	"testing"

	"tordo/internal/audiofile"
	"tordo/internal/musicbrainz"
	"tordo/internal/store"
)

func albumNamed(t *testing.T, st *store.Store, title string) store.AlbumDetail {
	t.Helper()
	as, err := st.Albums("")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range as {
		if a.TagTitle == title {
			d, _, err := st.Album(a.ID)
			if err != nil {
				t.Fatal(err)
			}
			return d
		}
	}
	t.Fatalf("no album %q in %+v", title, as)
	return store.AlbumDetail{}
}

// A correction resolves only the albums with the corrected tracks, and
// writes what a full apply would.
func TestCorrectionAppliesOnlyItsAlbums(t *testing.T) {
	files := signosFiles()
	for i := range files {
		files[i].tags.Genres = []string{"Rock nacional", "Chamamé"}
	}
	files = append(files, file{"../musica/Otro/01.mp3", "fp-o", 1000,
		audiofile.Tags{Title: "Uno", Artist: "Otro", Album: "Otro", Genres: []string{"Chamamé"}}})
	st := newCatalog(t, files...)
	mb := &fakeMB{releases: map[string]musicbrainz.Release{signosID: signosRelease()}}
	r, _ := newRunner(t, st, mb, nil)
	ctx := context.Background()
	if err := r.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if un, _ := st.UnmappedGenres(); !reflect.DeepEqual(un, []store.UnmappedGenre{{Raw: "Chamamé", Albums: 2}}) {
		t.Fatalf("unmapped = %+v", un)
	}
	// Otro gets a correction nothing applied yet: correcting Signos must not apply it.
	if err := st.SetCorrection([]string{"fp-o"}, store.Correction{Status: store.StatusIgnored}); err != nil {
		t.Fatal(err)
	}

	if err := r.SetRelease(ctx, albumNamed(t, st, "Signos").ID, signosID); err != nil {
		t.Fatal(err)
	}
	signos := albumNamed(t, st, "Signos")
	if signos.Status != store.StatusManual || signos.ReleaseID != signosID || signos.Confidence != 1 ||
		signos.Tracks[1].Title != "Sin sobresaltos" || len(signos.Genres) == 0 || signos.Genres[0].Source != "mb" {
		t.Fatalf("signos = %+v", signos)
	}
	if o := albumNamed(t, st, "Otro"); o.Status != store.StatusUnmatched {
		t.Fatalf("otro was applied: %q", o.Status)
	}
	if un, _ := st.UnmappedGenres(); !reflect.DeepEqual(un, []store.UnmappedGenre{{Raw: "Chamamé", Albums: 1}}) {
		t.Fatalf("unmapped after the correction = %+v", un)
	}

	if err := r.Apply(); err != nil {
		t.Fatal(err)
	}
	if full := albumNamed(t, st, "Signos"); !reflect.DeepEqual(full, signos) {
		t.Fatalf("a full apply differs:\n got %+v\nwant %+v", full, signos)
	}
	if o := albumNamed(t, st, "Otro"); o.Status != store.StatusIgnored {
		t.Fatalf("otro after a full apply: %q", o.Status)
	}
	if un, _ := st.UnmappedGenres(); !reflect.DeepEqual(un, []store.UnmappedGenre{{Raw: "Chamamé", Albums: 1}}) {
		t.Fatalf("unmapped after a full apply = %+v", un)
	}
}

// The same fingerprints in two folders: both albums follow the correction.
func TestCorrectionReachesCopies(t *testing.T) {
	files := signosFiles()
	for _, f := range signosFiles() {
		f.path = "../musica/copia/" + f.path[len(signosDir)+1:]
		files = append(files, f)
	}
	st := newCatalog(t, files...)
	r, _ := newRunner(t, st, &fakeMB{}, nil)
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	as, _ := st.Albums("")
	if len(as) != 2 {
		t.Fatalf("albums = %+v", as)
	}
	if err := r.Ignore(as[0].ID); err != nil {
		t.Fatal(err)
	}
	for _, a := range as {
		if d, _, _ := st.Album(a.ID); d.Status != store.StatusIgnored {
			t.Fatalf("%s: status %q", d.Dir, d.Status)
		}
	}
}
