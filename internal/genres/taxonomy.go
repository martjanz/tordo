package genres

// Genre is a top-level genre with its styles.
type Genre struct {
	Name   string   `json:"name"`
	Styles []string `json:"styles"`
}

// Taxonomy is Discogs' genres with the styles tordo knows under each
// (Discogs' names; "Murga" is tordo's own). A style belongs to one genre.
var Taxonomy = []Genre{
	{"Blues", []string{"Chicago Blues", "Country Blues", "Delta Blues", "Electric Blues", "Piano Blues"}},
	{"Brass & Military", []string{"Brass Band", "Marches", "Military"}},
	{"Children's", []string{"Educational", "Nursery Rhymes", "Story"}},
	{"Classical", []string{"Baroque", "Choral", "Contemporary", "Modern", "Opera", "Romantic"}},
	{"Electronic", []string{"Ambient", "Downtempo", "Drum n Bass", "Electro", "Experimental", "House", "IDM", "Minimal",
		"Synth-pop", "Techno", "Trance"}},
	{"Folk, World, & Country", []string{"African", "Andean", "Bluegrass", "Celtic", "Chacarera", "Country", "Fado",
		"Flamenco", "Folk", "Zamba"}},
	{"Funk / Soul", []string{"Disco", "Funk", "Gospel", "Neo Soul", "Rhythm & Blues", "Soul"}},
	{"Hip Hop", []string{"Boom Bap", "Conscious", "Gangsta", "Trip Hop"}},
	{"Jazz", []string{"Bebop", "Big Band", "Contemporary Jazz", "Cool Jazz", "Free Jazz", "Fusion", "Hard Bop",
		"Latin Jazz", "Modal", "Post Bop", "Soul-Jazz", "Swing"}},
	{"Latin", []string{"Bachata", "Baião", "Bolero", "Bossanova", "Candombe", "Cha-Cha", "Choro", "Cumbia", "Forró",
		"Mambo", "Merengue", "Milonga", "MPB", "Murga", "Música Criolla", "Nueva Cancion", "Nueva Trova", "Ranchera",
		"Reggaeton", "Rumba", "Salsa", "Samba", "Son", "Tango", "Vallenato"}},
	{"Non-Music", []string{"Comedy", "Field Recording", "Interview", "Poetry", "Spoken Word"}},
	{"Pop", []string{"Ballad", "Chanson", "Easy Listening", "Europop", "Indie Pop", "Light Music", "Vocal"}},
	{"Reggae", []string{"Dancehall", "Dub", "Rocksteady", "Roots Reggae", "Ska"}},
	{"Rock", []string{"Alternative Rock", "Blues Rock", "Classic Rock", "Folk Rock", "Garage Rock", "Hard Rock",
		"Heavy Metal", "Indie Rock", "New Wave", "Pop Rock", "Post-Punk", "Prog Rock", "Psychedelic Rock", "Punk",
		"Rock & Roll", "Soft Rock"}},
	{"Stage & Screen", []string{"Musical", "Score", "Soundtrack", "Theme"}},
}

// ignored are raw values that are not genres. Country names are common in
// this collection ("Uruguay"); the country comes from MusicBrainz instead.
var ignored = keys("other", "others", "otro", "otros", "outros", "unknown", "desconocido", "genre", "género",
	"misc", "miscellaneous", "general", "default", "none", "various", "varios", "music", "música",
	"uruguay", "uruguayo", "argentina", "argentino", "brasil", "brazil", "brasileiro", "chile", "cuba", "méxico",
	"colombia", "perú", "españa", "spain", "usa")

// aliases are the built-in spellings of taxonomy values.
var aliases = func() map[string]Mapping {
	m := map[string]Mapping{}
	add := func(genre, style string, raws ...string) {
		for _, r := range raws {
			m[Key(r)] = Mapping{Genre: genre, Style: style}
		}
	}
	add("Latin", "", "latino", "latina", "latin music", "música latina", "latinoamericana", "música latinoamericana")
	add("Folk, World, & Country", "Folk", "folclore", "folklore", "folklore argentino", "folclore argentino",
		"folklore uruguayo", "folclore uruguayo", "folk music")
	add("Folk, World, & Country", "", "world", "world music", "música del mundo", "música do mundo")
	add("Rock", "", "rock nacional", "rock argentino", "rock en español", "rock latino", "rock uruguayo", "rock brasileiro")
	add("Rock", "Rock & Roll", "rock and roll", "rock n roll", "rock'n'roll")
	add("Rock", "Alternative Rock", "alternative", "alternativo", "alternativa")
	add("Rock", "Indie Rock", "indie")
	add("Rock", "Heavy Metal", "metal")
	add("Rock", "Punk", "punk rock")
	add("Latin", "MPB", "música popular brasileira")
	add("Latin", "Bossanova", "bossa nova", "bossa")
	add("Latin", "Samba", "samba enredo", "samba de enredo", "sambas de enredo", "pagode", "samba-canção")
	add("Latin", "Tango", "tango nuevo", "tango argentino")
	add("Latin", "Cumbia", "cumbia colombiana", "cumbia villera", "cumbia argentina")
	add("Latin", "Nueva Trova", "trova", "nueva trova cubana")
	add("Latin", "Nueva Cancion", "nueva canción", "canto popular", "canto popular uruguayo")
	add("Latin", "Murga", "murgas")
	add("Jazz", "Latin Jazz", "jazz latino")
	add("Hip Hop", "", "hiphop", "rap", "hip hop rap")
	add("Funk / Soul", "Rhythm & Blues", "r&b", "rnb", "rhythm and blues")
	add("Electronic", "", "electrónica", "dance", "edm")
	add("Classical", "", "clásica", "música clásica")
	add("Stage & Screen", "Soundtrack", "banda sonora", "bso", "ost", "soundtracks", "trilha sonora")
	add("Children's", "", "infantil", "infantiles", "niños", "kids", "children", "children's music", "música infantil")
	add("Pop", "", "pop internacional", "pop latino", "pop en español")
	add("Non-Music", "Spoken Word", "audiolibro", "audiobook", "spoken")
	return m
}()

func keys(raws ...string) map[string]bool {
	m := map[string]bool{}
	for _, r := range raws {
		m[Key(r)] = true
	}
	return m
}
