package nameparse

import "testing"

func TestCompose(t *testing.T) {
	cases := map[string]string{
		"Impe\u0301rio":       "Império",
		"Fito Pa\u0301ez":     "Fito Páez",
		"Corac\u0327a\u0303o": "Coração",
		"Mu\u0308ller":        "Müller",
		"Nin\u0303o":          "Niño",
		"Páez":                "Páez",
		"plain ascii":         "plain ascii",
		"x\u0301":             "x\u0301", // no precomposed letter: left alone
	}
	for in, want := range cases {
		if got := Compose(in); got != want {
			t.Errorf("Compose(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParsersCompose(t *testing.T) {
	if f := ParseFolder("Fito Pa\u0301ez - (1990) Tercer Mundo"); f.Artist != "Fito Páez" {
		t.Errorf("ParseFolder artist = %q", f.Artist)
	}
	if f := ParseFile("05 - Fue amor de verano de Pa\u0301ez.mp3", ""); f.Title != "Fue amor de verano de Páez" {
		t.Errorf("ParseFile title = %q", f.Title)
	}
}
