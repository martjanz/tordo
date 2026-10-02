package store

import (
	"testing"

	"tordo/internal/audiofile"
)

const sk = "../musica/Soulseek/complete"

// explorer is a library shaped like the real one's Soulseek downloads:
// pass-through chains, an album folder with a loose file next to albums,
// and an accented (NFD) folder name.
func explorer(t *testing.T) *Store {
	return catalog(t, explorerFiles(), explorerImages...)
}

var explorerImages = []string{sk + "/Low_ID/Lhasa de Sela/folder.jpg", sk + "/Low_ID/scan.jpg"}

func explorerFiles() map[string]audiofile.Tags {
	return map[string]audiofile.Tags{
		sk + "/Low_ID/Lhasa de Sela/01.mp3":         {Title: "El desierto", Artist: "Lhasa de Sela", Album: "La Llorona", Track: 1},
		sk + "/Low_ID/Lhasa de Sela/02.mp3":         {Title: "De cara a la pared", Artist: "Lhasa de Sela", Album: "La Llorona", Track: 2},
		sk + "/Low_ID/music of the crusades/01.mp3": {Title: "Chanterai por mon corage", Artist: "Early Music Consort", Album: "Music of the Crusades", Track: 1},
		sk + "/Low_ID/suelto.mp3":                   {Title: "Suelto"},
		sk + "/benteveo/Zitarrosa/Canta/01.mp3":     {Title: "Guitarra negra", Artist: "Alfredo Zitarrosa", Album: "Canta", Track: 1},
		sk + "/benteveo/Zitarrosa/Canta/02.mp3":     {Title: "Doña Soledad", Artist: "Alfredo Zitarrosa", Album: "Canta", Track: 2},
		"../musica/Uruguay/Jaime Roos/01.mp3":       {Title: "Brindis por Pierrot", Artist: "Jaime Roos", Album: "Brindis por Pierrot", Track: 1},
		"../musica/Brasil/Música Popular/01.mp3":   {Title: "Aquarela", Artist: "Toquinho", Album: "Aquarela", Track: 1},
	}
}

func childByName(t *testing.T, v FolderView, name string) FolderChild {
	t.Helper()
	for _, c := range v.Children {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no child %q in %+v", name, v.Children)
	return FolderChild{}
}

func TestFolderSizes(t *testing.T) {
	s := explorer(t)
	low, _, err := s.Folder(sk + "/Low_ID")
	if err != nil {
		t.Fatal(err)
	}
	if low.SizeBytes != 42 {
		t.Errorf("Low_ID size = %d, want 42", low.SizeBytes)
	}
	lhasa := childByName(t, low, "Lhasa de Sela")
	if lhasa.SizeBytes != 21 || lhasa.Codec != "mp3" || lhasa.Album == 0 {
		t.Errorf("Lhasa child = %+v", lhasa)
	}
	complete, _, err := s.Folder(sk)
	if err != nil {
		t.Fatal(err)
	}
	if complete.SizeBytes != 62 {
		t.Errorf("complete size = %d, want 62", complete.SizeBytes)
	}
	bent := childByName(t, complete, "benteveo")
	if bent.Album != 0 || bent.SizeBytes != 20 {
		t.Errorf("benteveo child = %+v", bent)
	}
	top, _, err := s.Folder("")
	if err != nil {
		t.Fatal(err)
	}
	if len(top.Children) != 1 || top.Children[0].SizeBytes != 82 {
		t.Errorf("top children = %+v", top.Children)
	}
}

func TestFolderSizesSkipMissing(t *testing.T) {
	s := explorer(t)
	var rows []FileRow
	for p := range explorerFiles() {
		if p != sk+"/Low_ID/suelto.mp3" {
			rows = append(rows, audioRow(p, 10, 1))
		}
	}
	for _, p := range explorerImages {
		rows = append(rows, FileRow{Path: p, Size: 1, MTime: 1, Kind: "image"})
	}
	if err := s.SyncFiles(rows, root); err != nil {
		t.Fatal(err)
	}
	low, _, err := s.Folder(sk + "/Low_ID")
	if err != nil {
		t.Fatal(err)
	}
	if low.SizeBytes != 32 {
		t.Errorf("size = %d, want 32", low.SizeBytes)
	}
	for _, f := range low.Files {
		if f.Name == "suelto.mp3" {
			t.Errorf("missing file listed: %+v", f)
		}
	}
}

func TestFolderFiles(t *testing.T) {
	s := explorer(t)
	low, _, err := s.Folder(sk + "/Low_ID")
	if err != nil {
		t.Fatal(err)
	}
	if len(low.Files) != 2 || low.Files[0].Name != "scan.jpg" || low.Files[1].Name != "suelto.mp3" {
		t.Fatalf("files = %+v", low.Files)
	}
	a, im := low.Files[1], low.Files[0]
	if a.Kind != "audio" || !a.Track || a.DurationMs != 1000 || a.Codec != "mp3" || a.Bitrate != 320 || a.SizeBytes != 10 {
		t.Errorf("audio file = %+v", a)
	}
	if im.Kind != "image" || im.Track || im.SizeBytes != 1 {
		t.Errorf("image file = %+v", im)
	}
	lh, _, _ := s.Folder(sk + "/Low_ID/Lhasa de Sela")
	if len(lh.Files) != 3 || lh.Files[0].Name != "01.mp3" || lh.Files[1].Name != "02.mp3" || lh.Files[2].Name != "folder.jpg" {
		t.Errorf("Lhasa files = %+v", lh.Files)
	}
	top, _, _ := s.Folder("")
	if top.Files == nil || len(top.Files) != 0 {
		t.Errorf("top files = %#v", top.Files)
	}
}

func TestFolderFilesPrefixSibling(t *testing.T) {
	s := catalog(t, map[string]audiofile.Tags{
		"../musica/A/01.mp3":   {Title: "a", Artist: "x", Album: "One"},
		"../musica/A B/01.mp3": {Title: "b", Artist: "x", Album: "Two"},
		"../musica/A0/01.mp3":  {Title: "c", Artist: "x", Album: "Three"},
	})
	v, ok, err := s.Folder("../musica/A")
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	if v.SizeBytes != 10 || len(v.Files) != 1 {
		t.Errorf("size %d files %+v", v.SizeBytes, v.Files)
	}
}
