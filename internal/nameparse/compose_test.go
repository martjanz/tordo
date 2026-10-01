package nameparse

import "testing"

func TestCompose(t *testing.T) {
	cases := map[string]string{
		"Império":     "Império",
		"Fito Páez":   "Fito Páez",
		"Coração":     "Coração",
		"Müller":      "Müller",
		"Niño":        "Niño",
		"Páez":        "Páez",
		"plain ascii": "plain ascii",
		"x́":          "x́", // no precomposed letter: left alone
	}
	for in, want := range cases {
		if got := Compose(in); got != want {
			t.Errorf("Compose(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParsersCompose(t *testing.T) {
	if f := ParseFolder("Fito Páez - (1990) Tercer Mundo"); f.Artist != "Fito Páez" {
		t.Errorf("ParseFolder artist = %q", f.Artist)
	}
	if f := ParseFile("05 - Fue amor de verano de Páez.mp3", ""); f.Title != "Fue amor de verano de Páez" {
		t.Errorf("ParseFile title = %q", f.Title)
	}
}
