package nameparse

import "testing"

func TestParseFile(t *testing.T) {
	cases := []struct {
		name, folderArtist string
		want               File
	}{
		{"05-Fue Amor.mp3", "", File{Track: 5, Title: "Fue Amor"}},
		{"08- Inglaterra.mp3", "Fito Páez", File{Track: 8, Title: "Inglaterra"}},
		{"03. Detour Ahead (take 2).mp3", "", File{Track: 3, Title: "Detour Ahead (take 2)"}},
		{"06 Camelô.opus", "", File{Track: 6, Title: "Camelô"}},
		{"08 antonico(alcides gerardi).opus", "", File{Track: 8, Title: "antonico(alcides gerardi)"}},
		{"1-05 Garota de Ipanema.flac", "", File{Disc: 1, Track: 5, Title: "Garota de Ipanema"}},
		{"07 - Curare - Nilze Carvalho.mp3", "Nilze Carvalho", File{Track: 7, Title: "Curare", Artist: "Nilze Carvalho"}},
		{"16 - Só pra contrariar - c- Almirante.mp3", "Noel Rosa", File{Track: 16, Title: "Só pra contrariar", Artist: "Almirante"}},
		{"20 - Lito Barrientos y Su Orquesta  - Cumbia en Do Menor.mp3", "", File{Track: 20, Artist: "Lito Barrientos y Su Orquesta", Title: "Cumbia en Do Menor"}},
		{"3 - La añera - Silvia Barrios ⧸ Daniel Barrera.opus", "Silvia Barrios", File{Track: 3, Title: "La añera", Artist: "Silvia Barrios / Daniel Barrera"}},
		{"1985 - La Nueva Milonga - Retirada  A los Colores-doblada.mp3", "", File{Year: 1985, Artist: "La Nueva Milonga", Title: "Retirada A los Colores-doblada"}},
		{"retirada 1961 nueva milonga-T1b-D4-doblada.mp3", "", File{Title: "retirada 1961 nueva milonga-T1b-D4-doblada"}},
		{"Criosho [K-NtwAQ8Oqk].opus", "", File{Title: "Criosho"}},
		{"Gilberto Gil - Aquele Abraço.mp3", "", File{Artist: "Gilberto Gil", Title: "Aquele Abraço"}},
		{"05.mp3", "", File{Title: "05"}},
	}
	for _, c := range cases {
		if got := ParseFile(c.name, c.folderArtist); got != c.want {
			t.Errorf("ParseFile(%q, %q)\n got  %+v\n want %+v", c.name, c.folderArtist, got, c.want)
		}
	}
}
