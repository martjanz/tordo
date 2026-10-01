package nameparse

import "testing"

func TestCleanTag(t *testing.T) {
	cases := map[string]string{
		"WWW.TODOMUSICACD.8K.COM":                     "",
		"www.lomusical.com":                           "",
		"http://musica.example.net":                   "",
		"Tercer Mundo (LoMusical.Com)":                "Tercer Mundo",
		"Naturaleza Sangre [www.todomusicacd.8k.com]": "Naturaleza Sangre",
		"M031CAS_e_01_01":                             "",
		"Tercer Mundo":                                "Tercer Mundo",
		"P.1":                                         "P.1",
		"U2":                                          "U2",
		"Blink-182":                                   "Blink-182",
		"Mr. Bungle":                                  "Mr. Bungle",
		"La Ley es Tela de Araña y Melo":              "La Ley es Tela de Araña y Melo",
		"  Signos  ":                                  "Signos",
		"":                                            "",
	}
	for in, want := range cases {
		if got := CleanTag(in); got != want {
			t.Errorf("CleanTag(%q) = %q, want %q", in, got, want)
		}
	}
}
