package grouping

import (
	"testing"

	"tordo/internal/audiofile"
)

func album(id int64, dir, artist, title string) Track {
	return tr(id, dir+"/01.mp3", audiofile.Tags{Title: "t", Artist: artist, AlbumArtist: artist, Album: title})
}

func folderMap(r Result) map[string]Folder {
	m := map[string]Folder{}
	for _, f := range r.Folders {
		m[f.Path] = f
	}
	return m
}

func TestFolderTotals(t *testing.T) {
	r := Build([]Track{
		album(1, "../musica/Uruguay/Alfredo Zitarrosa/Guitarra negra", "Alfredo Zitarrosa", "Guitarra negra"),
		album(2, "../musica/Uruguay/Alfredo Zitarrosa/Adagio en mi país", "Alfredo Zitarrosa", "Adagio en mi país"),
		tr(3, "../musica/suelto.mp3", audiofile.Tags{Title: "x"}),
	}, nil, roots)
	f := folderMap(r)
	root := f["../musica"]
	if root.Parent != "" || root.Tracks != 3 || root.Albums != 2 || root.DurationMs != 3000 || root.Name != "musica" {
		t.Errorf("root = %+v", root)
	}
	z := f["../musica/Uruguay/Alfredo Zitarrosa"]
	if z.Parent != "../musica/Uruguay" || z.Tracks != 2 || z.Albums != 2 || z.Collection {
		t.Errorf("artist folder = %+v", z)
	}
	if len(r.Folders) != 5 {
		t.Errorf("folders = %d, want 5 (root, Uruguay, artist, 2 albums)", len(r.Folders))
	}
	if r.Folders[0].Path != "../musica" {
		t.Errorf("first folder = %q, want the root", r.Folders[0].Path)
	}
}

func TestCollections(t *testing.T) {
	la := "../musica/Latinoamérica 600"
	r := Build([]Track{
		album(1, la+"/[LA-598] Cypress Hill - (1999) Los Grandes Éxitos En Español", "Cypress Hill", "Los Grandes Éxitos En Español"),
		album(2, la+"/[LA-599] Rata Blanca - (1990) Magos, Espadas Y Rosas", "Rata Blanca", "Magos, Espadas Y Rosas"),
		album(3, la+"/[LA-600] Ricardo Villalobos - (2003) - Alcachofa", "Ricardo Villalobos", "Alcachofa"),
		// An artist's folder: same artist everywhere.
		album(4, "../musica/Noel Rosa/A", "Noel Rosa", "A"),
		album(5, "../musica/Noel Rosa/B", "Noel Rosa", "B"),
		album(6, "../musica/Noel Rosa/C", "Noel Rosa", "C"),
		// Named after an artist, even with a tribute album inside.
		album(7, "../musica/Fito Páez/Tercer Mundo", "Fito Páez", "Tercer Mundo"),
		album(8, "../musica/Fito Páez/Giros", "Fito Páez", "Giros"),
		album(9, "../musica/Fito Páez/Tributo", "Varios", "Tributo a Fito"),
		// Only two albums.
		album(10, "../musica/Dos/A", "X", "A"),
		album(11, "../musica/Dos/B", "Y", "B"),
	}, nil, roots)
	f := folderMap(r)
	if !f[la].Collection {
		t.Errorf("%s should be a collection: %+v", la, f[la])
	}
	for _, p := range []string{"../musica", "../musica/Noel Rosa", "../musica/Fito Páez", "../musica/Dos"} {
		if f[p].Collection {
			t.Errorf("%s should not be a collection", p)
		}
	}
}
