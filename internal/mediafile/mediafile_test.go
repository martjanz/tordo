package mediafile

import "testing"

func TestClassify(t *testing.T) {
	cases := map[string]Kind{
		"05-Fue Amor.mp3":                   Audio,
		"01 - Camelô.OPUS":                  Audio,
		"track.flac":                        Audio,
		"x.m4a":                             Audio,
		"x.wma":                             Audio,
		"x.wav":                             Audio,
		"x.aiff":                            Audio,
		"x.ape":                             Audio,
		"x.wv":                              Audio,
		"x.ogg":                             Audio,
		"cover.JPG":                         Image,
		"folder.png":                        Image,
		"x.webp":                            Image,
		"letra.lrc":                         Lyrics,
		"lista.m3u8":                        Playlist,
		"libro.epub":                        None,
		"info.nfo":                          None,
		".DS_Store":                         None,
		"Thumbs.db":                         None,
		"._05-Fue Amor.mp3":                 None, // macOS resource fork on exFAT
		"Music Library.musiclibrary.tagset": None,
	}
	for name, want := range cases {
		if got := Classify(name); got != want {
			t.Errorf("Classify(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestIgnoredDir(t *testing.T) {
	cases := []struct {
		rel   string
		extra []string
		want  bool
	}{
		{"Ableton", nil, true},
		{"Audio Music Apps", nil, true},
		{"djay", nil, true},
		{"Soulseek/downloading", nil, true},
		{"Music/Music Library.musiclibrary", nil, true},
		{"Music/Previous Libraries.localized", nil, true},
		{"Music/Media.localized", nil, false}, // Apple Music keeps real files here
		{".hidden", nil, true},
		{"Brasil/.AppleDouble", nil, true},
		{"$RECYCLE.BIN", nil, true},
		{"Brasil/Elza Soares", nil, false},
		{"Soulseek/complete", nil, false},
		{"Podcasts", []string{"podcasts"}, true},
		{"Brasil/Podcasts", []string{"podcasts"}, true},
		{"Soulseek/incomplete", []string{"Soulseek/incomplete/"}, true},
		{"Otro/incomplete", []string{"Soulseek/incomplete"}, false},
		{"Brasil/Samba", []string{"brasil/s*"}, true},
	}
	for _, c := range cases {
		if got := IgnoredDir(c.rel, c.extra); got != c.want {
			t.Errorf("IgnoredDir(%q, %q) = %v, want %v", c.rel, c.extra, got, c.want)
		}
	}
}
