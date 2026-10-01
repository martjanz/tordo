package audiofile

import (
	"strconv"
	"strings"
)

// genreNames is the ID3v1 genre table (0–79 standard, 80–191 Winamp
// extensions). ID3v2 TCON, MP4 gnre and ID3v1 refer to it by number.
var genreNames = []string{
	"Blues", "Classic Rock", "Country", "Dance", "Disco", "Funk", "Grunge", "Hip-Hop", "Jazz", "Metal",
	"New Age", "Oldies", "Other", "Pop", "R&B", "Rap", "Reggae", "Rock", "Techno", "Industrial",
	"Alternative", "Ska", "Death Metal", "Pranks", "Soundtrack", "Euro-Techno", "Ambient", "Trip-Hop", "Vocal", "Jazz+Funk",
	"Fusion", "Trance", "Classical", "Instrumental", "Acid", "House", "Game", "Sound Clip", "Gospel", "Noise",
	"Alternative Rock", "Bass", "Soul", "Punk", "Space", "Meditative", "Instrumental Pop", "Instrumental Rock", "Ethnic", "Gothic",
	"Darkwave", "Techno-Industrial", "Electronic", "Pop-Folk", "Eurodance", "Dream", "Southern Rock", "Comedy", "Cult", "Gangsta",
	"Top 40", "Christian Rap", "Pop/Funk", "Jungle", "Native American", "Cabaret", "New Wave", "Psychedelic", "Rave", "Showtunes",
	"Trailer", "Lo-Fi", "Tribal", "Acid Punk", "Acid Jazz", "Polka", "Retro", "Musical", "Rock & Roll", "Hard Rock",
	"Folk", "Folk-Rock", "National Folk", "Swing", "Fast Fusion", "Bebop", "Latin", "Revival", "Celtic", "Bluegrass",
	"Avantgarde", "Gothic Rock", "Progressive Rock", "Psychedelic Rock", "Symphonic Rock", "Slow Rock", "Big Band", "Chorus", "Easy Listening", "Acoustic",
	"Humour", "Speech", "Chanson", "Opera", "Chamber Music", "Sonata", "Symphony", "Booty Bass", "Primus", "Porn Groove",
	"Satire", "Slow Jam", "Club", "Tango", "Samba", "Folklore", "Ballad", "Power Ballad", "Rhythmic Soul", "Freestyle",
	"Duet", "Punk Rock", "Drum Solo", "A Cappella", "Euro-House", "Dance Hall", "Goa", "Drum & Bass", "Club-House", "Hardcore",
	"Terror", "Indie", "Britpop", "Afro-Punk", "Polsk Punk", "Beat", "Christian Gangsta Rap", "Heavy Metal", "Black Metal", "Crossover",
	"Contemporary Christian", "Christian Rock", "Merengue", "Salsa", "Thrash Metal", "Anime", "JPop", "Synthpop", "Abstract", "Art Rock",
	"Baroque", "Bhangra", "Big Beat", "Breakbeat", "Chillout", "Downtempo", "Dub", "EBM", "Eclectic", "Electro",
	"Electroclash", "Emo", "Experimental", "Garage", "Global", "IDM", "Illbient", "Industro-Goth", "Jam Band", "Krautrock",
	"Leftfield", "Lounge", "Math Rock", "New Romantic", "Nu-Breakz", "Post-Punk", "Post-Rock", "Psytrance", "Shoegaze", "Space Rock",
	"Trop Rock", "World Music", "Neoclassical", "Audiobook", "Audio Theatre", "Neue Deutsche Welle", "Podcast", "Indie Rock", "G-Funk", "Dubstep",
	"Garage Rock", "Psybient",
}

// id3Genres expands ID3v1 genre references in a TCON value — "(17)",
// "(17)Rock", "17", "(RX)", "(CR)" — and returns any other text as is.
func id3Genres(s string) []string {
	var out []string
	for strings.HasPrefix(s, "(") {
		end := strings.IndexByte(s, ')')
		if end < 0 {
			break
		}
		ref := s[1:end]
		s = s[end+1:]
		switch ref {
		case "RX":
			out = append(out, "Remix")
		case "CR":
			out = append(out, "Cover")
		default:
			if n, err := strconv.Atoi(ref); err == nil && n >= 0 && n < len(genreNames) {
				out = append(out, genreNames[n])
			}
		}
	}
	s = strings.TrimSpace(s)
	if n, err := strconv.Atoi(s); err == nil {
		if n >= 0 && n < len(genreNames) {
			out = append(out, genreNames[n])
		}
		return out
	}
	if s != "" {
		out = append(out, s)
	}
	return out
}
