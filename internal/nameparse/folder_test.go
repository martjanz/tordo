package nameparse

import "testing"

func TestParseFolder(t *testing.T) {
	cases := []struct {
		in   string
		want Folder
	}{
		{"El Cuarteto Imperial - (2008) Para Bailar y Bailar con El Cuarteto Imperial",
			Folder{Artist: "El Cuarteto Imperial", Album: "Para Bailar y Bailar con El Cuarteto Imperial", Year: 2008}},
		{"[LA-599] Rata Blanca - (1990) Magos, Espadas Y Rosas",
			Folder{Position: 599, Artist: "Rata Blanca", Album: "Magos, Espadas Y Rosas", Year: 1990}},
		{"[LA-600] Ricardo Villalobos - (2003) - Alcachofa",
			Folder{Position: 600, Artist: "Ricardo Villalobos", Album: "Alcachofa", Year: 2003}},
		{"(1990) Tercer Mundo", Folder{Album: "Tercer Mundo", Year: 1990}},
		{"Bill Evans Trio - 1961 -Waltz For Debby", Folder{Artist: "Bill Evans Trio", Album: "Waltz For Debby", Year: 1961}},
		{"Elizeth Cardoso - Live in Japan - 1977", Folder{Artist: "Elizeth Cardoso", Album: "Live in Japan", Year: 1977}},
		{"Herb Alpert - [1980] Beyond (2015) [24-88.2]", Folder{Artist: "Herb Alpert", Album: "Beyond", Year: 1980}},
		{"orquestra afro-brasileira - obaluaye! (1957) (digital) (aac vbr)",
			Folder{Artist: "orquestra afro-brasileira", Album: "obaluaye!", Year: 1957}},
		{"orquestra afro-brasileira - orquestra afro-brasileira 1968 (lp) (256kb)",
			Folder{Artist: "orquestra afro-brasileira", Album: "orquestra afro-brasileira", Year: 1968}},
		{"1971 - História da Música Popular Brasileira 32 BILLY BLANCO",
			Folder{Album: "História da Música Popular Brasileira 32 BILLY BLANCO", Year: 1971}},
		{"Noel Rosa - Noel pela primeira vez Cd 02 -x", Folder{Artist: "Noel Rosa", Album: "Noel pela primeira vez", Disc: 2}},
		{"Varios - Candombe de llamadas CD 1 (selección, falta tema 20)",
			Folder{Various: true, Album: "Candombe de llamadas (selección, falta tema 20)", Disc: 1}},
		{"AAVV - Vinicius de Moraes Songbook (1993)", Folder{Various: true, Album: "Vinicius de Moraes Songbook", Year: 1993}},
		{"VA - História da MPB", Folder{Various: true, Album: "História da MPB"}},
		{"Volume 2, CD 1 (Social Music)", Folder{Album: "Volume 2 (Social Music)", Disc: 1}},
		{"História das escolas de samba Disc 4", Folder{Album: "História das escolas de samba", Disc: 4}},
		{"CD1", Folder{Disc: 1}},
		{"Nelson_Cavaquinho", Folder{Album: "Nelson Cavaquinho"}},
		{"Silvia Barrios - Folclore y tango", Folder{Artist: "Silvia Barrios", Album: "Folclore y tango"}},
		{"15 discos de los 60", Folder{Album: "15 discos de los 60"}},
		{"Latinoamérica 600", Folder{Album: "Latinoamérica 600"}},
		{"07 - Soda Stereo - Signos", Folder{Position: 7, Artist: "Soda Stereo", Album: "Signos"}},
		{"[1980-1981] Casi en privado (programa de Radio Educación - Mexico)",
			Folder{Album: "Casi en privado (programa de Radio Educación - Mexico)", Year: 1980}},
		{"[1962] en Radio Centenario", Folder{Album: "en Radio Centenario", Year: 1962}},
	}
	for _, c := range cases {
		if got := ParseFolder(c.in); got != c.want {
			t.Errorf("ParseFolder(%q)\n got  %+v\n want %+v", c.in, got, c.want)
		}
	}
}

func TestNorm(t *testing.T) {
	cases := map[string]string{
		"Fito Páez":           "fito paez",
		"  FITO   PÁEZ!! ":    "fito paez",
		"Nelson_Cavaquinho":   "nelson cavaquinho",
		"Ñandú & Çedilla":     "nandu cedilla",
		"Os Originais do Sam": "os originais do sam",
		"Império Serrano":    "imperio serrano", // macOS hands out decomposed (NFD) names
		"":                    "",
	}
	for in, want := range cases {
		if got := Norm(in); got != want {
			t.Errorf("Norm(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsVarious(t *testing.T) {
	for _, s := range []string{"VA", "V.A.", "AAVV", "Varios", "Various Artists", "Varios artistas", "VVAA"} {
		if !IsVarious(s) {
			t.Errorf("IsVarious(%q) = false", s)
		}
	}
	for _, s := range []string{"", "Vanessa", "Varios Rock"} {
		if IsVarious(s) {
			t.Errorf("IsVarious(%q) = true", s)
		}
	}
}

func TestStripDisc(t *testing.T) {
	cases := []struct {
		in   string
		rest string
		disc int
	}{
		{"Noel pela primeira vez Cd 02", "Noel pela primeira vez", 2},
		{"Greatest Hits (Disc 1)", "Greatest Hits", 1},
		{"Disco 3", "", 3},
		{"Discovery", "Discovery", 0},
		{"Disco 2000", "Disco 2000", 0},
	}
	for _, c := range cases {
		if rest, disc := StripDisc(c.in); rest != c.rest || disc != c.disc {
			t.Errorf("StripDisc(%q) = %q, %d; want %q, %d", c.in, rest, disc, c.rest, c.disc)
		}
	}
}
