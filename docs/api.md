# HTTP API

Tordo serves a JSON API on the same address as the web app. Requests from other devices on the
network need a session (see [Network access](how-it-works.md#network-access)); without one, `/api/`
and `/stream/` answer 401.

URLs, query parameters, facet names and API paths are in English whatever the interface
language (`/explore`, `/folders`, `/settings`, `/help`…; `?view=albums&order=year`,
`decade`, `listened=never`…).

| Method and path | What it returns |
|---|---|
| `GET /api/status` | scan progress, catalog counts (the last known ones when the catalog is busy; none before the first), `preparing` while the first catalog is built, normalization status, `setupPending` flag, `language` |
| `POST /api/scan` | starts a scan (`Content-Type: application/json`) |
| `GET /api/albums?folder=` | albums, all or inside a folder |
| `GET /api/albums/{id}` | an album with its tracks (with `sizeBytes` per track), versions (other albums sharing duplicate groups), and technical data |
| `GET /api/folders?path=` | a folder: subfolders (with size), albums, loose tracks and the files directly in it (no `path`: the roots) |
| `GET /api/folders/tree?path=` | one level of the Folders tree: subfolders, pass-through chains merged |
| `GET /api/folders/tree/search?q=` | folders by name, with the tree rows leading to them |
| `GET /api/collections` | folders detected as collections, with their albums in order |
| `PUT /api/folders/collection` | `{"path", "collection": false}`: not a collection (`true` takes it back) |
| `GET /api/explore?view=&<facets>&order=&dir=&from=&limit=` | a page of Explore (albums, artists or tracks) with the facets' counts |
| `GET /api/artists/{key}` | an artist: albums by year, appearances on others' albums, and their 5 most-played tracks in `top` |
| `GET /api/search?q=&limit=` | artists, albums, tracks and folders matching `q` |
| `GET /api/genres` | the taxonomy of genres and styles |
| `GET /api/genres/unmapped` | raw tag genres no mapping knows (Genres to map) |
| `PUT /api/genres/aliases`, `DELETE /api/genres/aliases?raw=` | map a raw genre (or ignore it), or forget the mapping |
| `GET /api/review/unidentified` | albums awaiting identification review |
| `PUT /api/albums/{id}/identification` | choose the identification for an album (`DELETE` goes back to automatic) |
| `GET /api/review/duplicates?kind=` | duplicate groups; `kind` is `exact`, `formats` or `suspected` (default: `exact` when missing or unknown); returns `{counts, recoverable, groups}` |
| `GET /api/review/formats` | tracks needing conversion by format |
| `GET /api/marks` | likes and ratings for tracks and albums (by id), and liked artists and collections |
| `PUT /api/likes` | set or unset a like (♥) (`{kind, id|key, liked}`; `kind` is track/album/artist/collection; `album` is the Preferred bookmark) |
| `PUT /api/ratings` | set or clear a rating for a track or album (`{kind, id, stars}`; `stars` 0–5, 0 clears) |
| `POST /api/plays` | record a play of a track (`{id, listenedMs, device}`); ignored in browse-only mode |
| `GET /api/liked` | the liked tracks (♥), preferred albums (bookmark), liked artists and collections, most recent first |
| `GET /api/gains?track=` | each track's gains in dB (`{id: {trackGain, albumGain}}`, from ReplayGain tags or measured loudness); unmeasured tracks go first in the analysis queue; tracks marked "unmeasurable" (no ReplayGain tag and measurement failed permanently) are not re-analyzed |
| `GET /api/playlists` | user's playlists |
| `POST /api/playlists` | create a new playlist |
| `GET /api/playlists/{id}` | entries of a playlist |
| `PUT /api/playlists/{id}` | rename a playlist |
| `DELETE /api/playlists/{id}` | delete a playlist |
| `POST /api/playlists/{id}/items` | add an entry to a playlist |
| `DELETE /api/playlists/{id}/items/{item}` | remove an entry |
| `PUT /api/playlists/{id}/order` | reorder entries |
| `GET /api/stations` | saved stations, most recently updated first |
| `POST /api/stations` | save a station (body: `{name, source, mode, artistGap, avoidDays}`) |
| `PUT /api/stations/{id}` | rename or update a station |
| `DELETE /api/stations/{id}` | delete a saved station |
| `GET /api/station/next?def=<JSON>&recent=1,2,3` | the next batch of tracks from the station (at least 10 tracks, possibly more in albums mode to include complete albums), with mime and gains; `recent` is a comma-separated list of up to 200 recently-played track ids; the response includes `station` (the station definition as the server applied it) |
| `GET /api/home?seed=N` | Home: fixed rows, rotating rows, and saved stations; `seed` draws the rotating rows, missing or invalid gives a random one |
| `GET /api/stats?period=&limit=` | Listening history: `period` is `week`/`month`/`year`/`all` (default `month`); `limit` is the length of each ranking (default 10, range 1–100) |
| `GET /api/play?album=&folder=&artist=&track=&shuffle=1&playlist=&liked=1` | the tracks to queue, in order (at most 5000) |
| `GET /stream/{trackId}` | a track's audio (`Range` for seeking; `?transcode=1` converts it) |
| `GET /api/covers/album/{id}?size=` | an album's picture, whole or as a 160/320/640 px thumbnail |
| `GET /api/covers/track/{id}?size=` | the picture embedded in a track's file |
| `GET /api/config` | current configuration (settings only readable from the local computer) |
| `PUT /api/config` | save configuration: roots, ignore patterns, rescan interval, Discogs token, normalization, language, LAN settings (local computer only) |
| `POST /api/config/root` | check whether a folder can be a root: returns its canonical path or an error |
| `POST /api/config/token` | check whether Discogs accepts a token: returns `{valid: true/false/null}` (null when offline) |
| `GET /api/lan` | LAN info: enabled, port, listening, and URLs for other devices to reach Tordo |
| `GET /api/lan/qr?url=` | SVG QR code of a URL |
| `GET /api/auth` | always 200: `{local, session, device, pinRequired, language}` (page shows PIN screen when `pinRequired` is true; other `/api/` and `/stream/` paths answer 401 without a session) |
| `POST /api/auth/login` | authenticate with PIN from a LAN device: `{pin, device}` |
| `POST /api/auth/logout` | end this session |
| `GET /api/sessions` | list active sessions (local computer only) |
| `DELETE /api/sessions/{id}` | end a session by id (local computer only; LAN devices can only end their own session with `POST /api/auth/logout`) |

Errors are JSON, `{"code", "message", "params"}`, with the HTTP status unchanged: `message` is in
English and `code` is a stable identifier (declared in `internal/server/codes.go`) that the web app
translates.
