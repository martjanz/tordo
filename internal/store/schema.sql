CREATE TABLE IF NOT EXISTS files (
	id          INTEGER PRIMARY KEY,
	path        TEXT NOT NULL UNIQUE,
	size        INTEGER NOT NULL,
	mtime       INTEGER NOT NULL,
	kind        TEXT NOT NULL,
	fingerprint TEXT NOT NULL DEFAULT '',
	missing     INTEGER NOT NULL DEFAULT 0,
	first_seen  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS files_fingerprint ON files(fingerprint);

-- What reading an audio file produced; valid while size and mtime match.
CREATE TABLE IF NOT EXISTS audio (
	file_id      INTEGER PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,
	size         INTEGER NOT NULL,
	mtime        INTEGER NOT NULL,
	read_version INTEGER NOT NULL,
	with_ffprobe INTEGER NOT NULL,
	reader       TEXT NOT NULL DEFAULT '',
	error        TEXT NOT NULL DEFAULT '',
	format       TEXT NOT NULL DEFAULT '',
	codec        TEXT NOT NULL DEFAULT '',
	lossless     INTEGER NOT NULL DEFAULT 0,
	duration_ms  INTEGER NOT NULL DEFAULT 0,
	bitrate      INTEGER NOT NULL DEFAULT 0,
	sample_rate  INTEGER NOT NULL DEFAULT 0,
	bit_depth    INTEGER NOT NULL DEFAULT 0,
	channels     INTEGER NOT NULL DEFAULT 0,
	tags         TEXT NOT NULL DEFAULT '{}'
);

-- Derived tables, rebuilt from files + audio after every scan. The
-- identification columns of albums are written by ApplyIdentities and kept
-- across rebuilds (albums are upserted by key).
CREATE TABLE IF NOT EXISTS albums (
	id             INTEGER PRIMARY KEY,
	key            TEXT NOT NULL UNIQUE,
	dir            TEXT NOT NULL,
	title          TEXT NOT NULL,
	artist         TEXT NOT NULL,
	year           INTEGER NOT NULL,
	compilation    INTEGER NOT NULL,
	discs          INTEGER NOT NULL,
	tracks         INTEGER NOT NULL,
	duration_ms    INTEGER NOT NULL,
	position       INTEGER NOT NULL,
	cover_path     TEXT NOT NULL,
	cover_file_id  INTEGER NOT NULL,
	genres         TEXT NOT NULL,              -- raw tag values, JSON
	selection      INTEGER NOT NULL DEFAULT 0,
	seen           INTEGER NOT NULL DEFAULT 1,
	match_query    TEXT NOT NULL DEFAULT '',   -- identify.Query.Key()
	status         TEXT NOT NULL DEFAULT '',   -- '' | auto | manual | unmatched | ignored | selection
	release_id     TEXT NOT NULL DEFAULT '',   -- MusicBrainz release
	confidence     REAL NOT NULL DEFAULT 0,
	mb_title       TEXT NOT NULL DEFAULT '',
	mb_artist      TEXT NOT NULL DEFAULT '',
	mb_compilation INTEGER NOT NULL DEFAULT 0,
	original_year  INTEGER NOT NULL DEFAULT 0,
	country        TEXT NOT NULL DEFAULT '',   -- ISO 3166-1 alpha-2
	artist_id      TEXT NOT NULL DEFAULT '',   -- MusicBrainz artist
	cover_cache    TEXT NOT NULL DEFAULT ''    -- relative to the app folder
);
CREATE INDEX IF NOT EXISTS albums_dir ON albums(dir);

CREATE TABLE IF NOT EXISTS tracks (
	file_id      INTEGER PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,
	album_id     INTEGER REFERENCES albums(id),
	dir          TEXT NOT NULL,
	title        TEXT NOT NULL,
	artist       TEXT NOT NULL,
	disc         INTEGER NOT NULL,
	track        INTEGER NOT NULL,
	year         INTEGER NOT NULL,
	duration_ms  INTEGER NOT NULL,
	mb_title     TEXT NOT NULL DEFAULT '',
	recording_id TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS tracks_album ON tracks(album_id);
CREATE INDEX IF NOT EXISTS tracks_dir ON tracks(dir);

CREATE TABLE IF NOT EXISTS folders (
	path        TEXT PRIMARY KEY,
	parent      TEXT NOT NULL,
	name        TEXT NOT NULL,
	tracks      INTEGER NOT NULL,
	albums      INTEGER NOT NULL,
	duration_ms INTEGER NOT NULL,
	collection  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS folders_parent ON folders(parent);

-- Derived: an album's genres, from its best source, written by ApplyIdentities.
CREATE TABLE IF NOT EXISTS album_genres (
	album_id INTEGER NOT NULL REFERENCES albums(id) ON DELETE CASCADE,
	genre    TEXT NOT NULL,
	style    TEXT NOT NULL DEFAULT '',
	source   TEXT NOT NULL,                    -- tag | mb | discogs | manual
	PRIMARY KEY (album_id, genre, style)
);

-- Derived: raw tag genres nothing maps (the Géneros por mapear queue).
CREATE TABLE IF NOT EXISTS unmapped_genres (
	raw    TEXT PRIMARY KEY,
	albums INTEGER NOT NULL
);

-- The result of matching one album query against MusicBrainz, keyed by the
-- query: the same album found again (moved, rescanned) is not looked up again.
CREATE TABLE IF NOT EXISTS matches (
	query           TEXT PRIMARY KEY,
	status          TEXT NOT NULL,             -- auto | unmatched
	release_id      TEXT NOT NULL DEFAULT '',
	confidence      REAL NOT NULL DEFAULT 0,
	candidates      TEXT NOT NULL DEFAULT '[]',
	matcher_version INTEGER NOT NULL,
	updated_at      INTEGER NOT NULL
);

-- The user's identifications, per track fingerprint: they survive moving,
-- renaming and re-tagging. Never touched by the automatic identification.
CREATE TABLE IF NOT EXISTS corrections (
	fingerprint TEXT PRIMARY KEY,
	status      TEXT NOT NULL,                 -- manual | ignored
	release_id  TEXT NOT NULL DEFAULT '',
	updated_at  INTEGER NOT NULL
);

-- MusicBrainz releases and artists as fetched and enriched (JSON store.Release, store.Artist).
CREATE TABLE IF NOT EXISTS releases (
	id         TEXT PRIMARY KEY,
	data       TEXT NOT NULL,
	fetched_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS artists (
	id         TEXT PRIMARY KEY,
	data       TEXT NOT NULL,
	fetched_at INTEGER NOT NULL
);

-- The user's genre mappings: raw value (genres.Key) → genre/style.
CREATE TABLE IF NOT EXISTS genre_alias (
	raw   TEXT PRIMARY KEY,
	genre TEXT NOT NULL,                       -- '' = not a genre: ignore it
	style TEXT NOT NULL DEFAULT ''
);
