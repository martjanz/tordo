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

-- Derived tables, rebuilt from files + audio after every scan.
CREATE TABLE IF NOT EXISTS albums (
	id            INTEGER PRIMARY KEY,
	key           TEXT NOT NULL UNIQUE,
	dir           TEXT NOT NULL,
	title         TEXT NOT NULL,
	artist        TEXT NOT NULL,
	year          INTEGER NOT NULL,
	compilation   INTEGER NOT NULL,
	discs         INTEGER NOT NULL,
	tracks        INTEGER NOT NULL,
	duration_ms   INTEGER NOT NULL,
	position      INTEGER NOT NULL,
	cover_path    TEXT NOT NULL,
	cover_file_id INTEGER NOT NULL,
	genres        TEXT NOT NULL,
	seen          INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS albums_dir ON albums(dir);

CREATE TABLE IF NOT EXISTS tracks (
	file_id     INTEGER PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,
	album_id    INTEGER REFERENCES albums(id),
	dir         TEXT NOT NULL,
	title       TEXT NOT NULL,
	artist      TEXT NOT NULL,
	disc        INTEGER NOT NULL,
	track       INTEGER NOT NULL,
	year        INTEGER NOT NULL,
	duration_ms INTEGER NOT NULL
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
