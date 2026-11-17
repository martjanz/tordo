# Tordo — Interface languages (Spanish, English, Brazilian Portuguese)

Date: 2026-11-16
Status: agreed in brainstorming, pending written review

This design is written in English (the main spec, `2026-10-01-tordo-design.md`, stays in Spanish).

## 1. Intent

Tordo is open source, and people who download it should be able to use it without knowing Spanish. Today every
user-facing string is hard-coded Spanish. This change makes the interface available in **Spanish, English and
Brazilian Portuguese**, chosen automatically from the browser or set once per installation, and makes the
project's own vocabulary (URLs, API, logs, CLI) English so it reads as a standard tool.

Success: a person whose browser is in English or Portuguese opens Tordo for the first time and everything they read
(the welcome assistant, the PIN page, every page, the help, notices and errors) is in their language, without
touching a setting.

### Goals

- Every string the web app shows comes from a per-language message file; three languages ship: `es`, `en`, `pt-BR`.
- Language selection: an installation-wide setting in Ajustes, default "automatic" (the browser's language).
- Numbers, dates, durations, sizes, plurals and country names follow the UI language.
- Server errors that reach the UI are shown in the UI language.
- The help page exists in the three languages.
- `README.md` (English) plus `README.es.md` and `README.pt-BR.md`, linked to each other.
- English everywhere the project's vocabulary is visible to developers or in the address bar: page URLs, query
  parameters, facet names and values, API parameters, server log lines, CLI flag help.
- Adding a fourth language later means adding one message file (plus its help file) and registering it.

### Non-goals

- Translating genre and style names. They stay Discogs's English taxonomy names in every language ("Folk, World, &
  Country", "Funk / Soul"), as Spanish users already see them today.
- Translating data: album, artist and track titles, folder names, raw tags, user-typed names (playlists, saved
  stations).
- Per-browser language override. When the installation's setting is not "automatic", every browser follows it.
- Redirects from the old Spanish URLs (no bookmarks exist yet).
- Renaming Spanish source file names (`Ajustes.svelte`, `ayuda.js`, `pages/`, `components/`): churn with no user
  benefit. Code identifiers and comments are already English.
- New README screenshots in English (possible follow-up; the existing images are reused).
- Professional translation review. Translations are drafted in this work; the owner reviews Spanish, and may
  review the others.
- Translating the design specs and plans in `docs/`.

### Anti-goals (a failure even if it works)

- A new runtime or build dependency for i18n (the project keeps Go to `modernc.org/sqlite` and the web to Svelte,
  Vite, Vitest).
- Strings that silently show a raw key (`album.title`) or an empty label to the user.
- The first paint waiting on a network request just to know the strings.
- Mixed languages on a page because some component was missed: Spanish leftovers in English UI.

## 2. Constraints

- Repo rules from `CLAUDE.md`: no cgo, minimal dependencies, TDD, `go test ./...` and `(cd web && npm test)` pass
  with no network, commit messages without `Co-Authored-By:` trailers.
- Read-only on the music collection; the only writes are to the app folder (`config.json`, `tordo.db`, `cache/`).
- Configuration endpoints answer only from localhost (§9 of the main spec): the language is set from the computer
  running Tordo; phones on the LAN follow it.
- The PIN page (Entrar) is shown before a LAN session exists: it can only read `GET /api/auth` (see `openPath` in
  `internal/server/lan.go`).

## 3. Approaches considered

1. **Own small translation layer + error codes from the server** — *chosen.* One message module per language in
   the web app, a `t(key, params)` function with plural support from the browser's `Intl.PluralRules`, a reactive
   current locale. The server sends a stable error code plus an English message; the web translates the code. No
   dependency; a test guards that all languages have the same keys and placeholders.
2. **A library (Paraglide)** — compile-time, typed message functions, a missing key fails the build. Rejected: adds
   a build plugin and dependency against the project's rule, and the long help prose fits it awkwardly; the key
   parity test of (1) gives most of the safety.
3. **Go holds and serves the strings** (`/api/strings`), and translates its own errors. Rejected: every page waits
   on a fetch before it can show text, web tests need fixtures, and the strings live in Go though nearly all are
   the web's.

## 4. Decisions

### 4.1 Languages and how one is chosen

- Supported locales: `es` (Rioplatense Spanish, with voseo, as today: "podés", "activá"), `en` (plain US
  English), `pt-BR` (Brazilian Portuguese, "você").
- `config.json` gets `"language": ""` (absent or `""`: automatic) or one of `"es"`, `"en"`, `"pt-BR"`. Unknown
  values are treated as automatic (and logged once).
- **Resolution** (in the web app): if the installation setting is a supported locale, use it. Otherwise
  automatic: walk `navigator.languages` in order and take the first whose primary subtag is `es`, `en` or `pt`
  (`pt-PT` maps to `pt-BR`, `es-MX` to `es`, `en-GB` to `en`). If none matches, **English**.
- The setting is exposed where every page can read it:
  - `GET /api/auth` (open, so the PIN page and the welcome assistant know it) gains `language`.
  - `GET /api/status` gains `language`, so open browsers switch within one status poll when it changes in Ajustes.
  - `GET /api/config` / `PUT /api/config` carry it like the other settings (localhost only).
- **Ajustes** gets an "Idioma / Language / Idioma" control at the top (a select: Automatic (browser language),
  Español, English, Português (Brasil)). Each language's own name is written in that language. Changing the
  language must not trigger a rescan or restart the engine's background work: it is a display setting, so
  `Language` joins the fields that `sameParts` in `internal/engine/engine.go` ignores.
- **Bienvenida** (first run, no `config.json`) shows the UI in the automatic language and offers the same select;
  the choice is saved with the rest of the assistant's answers.
- Before the language is known (first paint, before `/api/auth` answers) the app renders with the browser-derived
  locale; when the server's setting differs, it switches. No blank screen waits for it.
- `<html lang>` is set at runtime to the resolved locale (`index.html` keeps a static default of `en`). Page
  `<title>`s are translated.

### 4.2 The web translation layer

Location: `web/src/lib/i18n/`.

- `es.js`, `en.js`, `pt-BR.js`: each default-exports a flat or nested object of messages keyed by stable English
  dotted names grouped by area (`nav.explore`, `album.play`, `player.repeat.one`, `error.album_not_found`). **English
  (`en.js`) is the reference**: it defines the key set; a missing key in another language falls back to English at
  runtime, and to the key itself only as a last resort (and a test makes that unreachable for shipped files).
- `i18n.svelte.js` (or similar): the current locale in a `$state`, `setLocale`, `resolveLocale(setting,
  navigatorLanguages)` (pure), and `t(key, params)`.
- **Messages:** plain strings with `{name}` placeholders. Plural messages are objects keyed by
  `Intl.PluralRules(locale)` categories, at least `one` and `other` (`{ one: '{n} pista', other: '{n} pistas' }`);
  `t` picks the category with `PluralRules` and falls back to `other`. Numbers passed as params are formatted with
  the locale's `Intl.NumberFormat` before substitution. No ICU syntax, no gender or select forms (not needed so far).
- Markup inside messages (links, `<kbd>`, emphasis): avoided by splitting messages around the markup in the
  component; where that is unworkable, the builder decides a minimal convention and documents it in the module.
- All three locales are bundled (they are small); no lazy loading.

### 4.3 Formatting helpers

`web/src/lib/format.js` and every `toLocaleString('es')` / `Intl.*(['es'])` / `'es-AR'` call (about 15 sites:
`format.js`, `status.js`, `dupes.js`, `folders.js`, `ArbolCarpetas`, `Busqueda`, `ContenidoCarpeta`,
`Dispositivos`, `ListaCarpeta`) use the current locale instead.

- Numbers: `Intl.NumberFormat` ("1.234" in es, "1,234" in en, "1.234" in pt-BR). Sizes keep their decimal
  rule ("7,2 MB" / "7.2 MB"). *Design call:* Spanish formats with the `es-AR` locale, not `es`: `es` writes "1234"
  without a separator for four digits, `es-AR` writes "1.234" (as `status.js` already does). This changes
  four-digit counts on the screens that use `'es'` today.
- Dates: `toLocaleString(locale, …)` with the same options as today.
- Relative times (`since`: "recién", "hace 25 min", "ayer", "hace 3 días"): translated messages with plurals; the
  builder may use `Intl.RelativeTimeFormat` if its output matches the current wording closely enough, otherwise
  messages.
- Durations ("3:07", "48 min", "1 h 12 min"): unit words become messages.
- Country names: `Intl.DisplayNames([locale], { type: 'region' })`, already the mechanism; it just follows the locale.
- Collation for folder sorting (`folders.js`): stays natural/numeric, using the current locale.
- Decade names ("Los 80", "Los 2000") become messages ("The 80s", "Anos 80"…).
- `count(n, one, many)` is replaced by plural messages.

### 4.4 Words that come from data

- **"Varios artistas"** (`grouping.Various`) is stored today as the album artist of compilations. The catalog
  stores a language-neutral marker instead (the builder chooses: an empty artist with `compilation = true`, or a
  fixed sentinel), the API keeps the `compilation` flag, and the web shows `t('album.variousArtists')`. Search keeps
  finding compilations by "varios artistas", "various artists" and "vários artistas". `nameparse.IsVarious`
  detection of folder names (`VA`, `AAVV`, `Varios`) is unchanged. Existing rows are updated by the migration, and
  a rebuild of the derived tables is not enough on its own: the identification path (`internal/identify/resolve.go`)
  also writes `grouping.Various` into identification data that survives a rebuild, so that data is migrated too.
- **Generated station names** (`stations.js`: "Tus me gusta", "Tus canciones favoritas", "Álbumes preferidos",
  "Toda la colección", decade names, facet labels joined with " · ") are produced in the current language when the
  station is created in the browser. **Saved stations keep the name they were saved with** (the user can edit it
  in the save dialog; it is user text from then on).
- **Inicio rows**: titles come from the web (`kind` + value), so they are translated there; collection labels are
  folder names (data).
- **Facet value labels** (`facets.js` `LABELS`, `VIEWS`, `ORDERS`, `FACETS`): messages.

### 4.5 English vocabulary: URLs, API, stored definitions

One English vocabulary replaces the Spanish one everywhere — page routes, query parameters, facet names and values,
API parameters and the station definitions stored in the database. The builder keeps the Go constants
(`internal/catalog/facets.go`, `home.go`, `station.go`, `duplicates.go`, `internal/server/home.go`, `review.go`,
`stations.go`) and the web (`router.js`, `facets.js`, `stations.js`, `home.js`, `review.js`, `api.js`) in step.

| Area | Today | New |
|---|---|---|
| Routes | `/explorar` `/carpetas` `/listas` `/listas/{id}` `/buscar` `/revisar` `/revisar/duplicados` `/revisar/formatos` `/revisar/generos` `/ajustes` `/ayuda` `/bienvenida` `/estadisticas` `/album/{id}` `/artista/{key}` | `/explore` `/folders` `/lists` `/lists/{id}` `/search` `/review` `/review/duplicates` `/review/formats` `/review/genres` `/settings` `/help` `/welcome` `/stats` `/album/{id}` `/artist/{key}` |
| Query params | `ver` `orden` `dir` `semilla` `ruta` `periodo` `tipo` `recientes` | `view` `order` `dir` `seed` `path` `period` `kind` `recent` |
| Views / station modes | `albumes` `artistas` `pistas` | `albums` `artists` `tracks` |
| Facets | `decada` `anio` `artista` `pais` `genero` `estilo` `coleccion` `carpeta` `calidad` `formato` `estado` `megusta` `puntaje` `escucha` | `decade` `year` `artist` `country` `genre` `style` `collection` `folder` `quality` `format` `state` `liked` `rating` `listened` |
| Facet values | `sin-perdida` `con-perdida` · `sin-identificar` `no-reproducible` `con-duplicados` · `si` · `nunca` `hace-un-anio` | `lossless` `lossy` · `unidentified` `unplayable` `duplicated` · `yes` · `never` `year-ago` |
| Orders | `anio` `titulo` `artista` `agregado` `cantidad` `escuchado` `azar` | `year` `title` `artist` `added` `count` `plays` `random` |
| Inicio row kinds | `decada` `pais` `genero` `estilo` `coleccion` `nunca` `hace-un-anio` `azar` `recientes` | `decade` `country` `genre` `style` `collection` `never` `year-ago` `random` `recent` |
| Stats periods | `semana` `mes` `anio` `siempre` | `week` `month` `year` `all` |
| Duplicate kinds / review tabs | `exactas` `formatos` `sospechas` · `sin-identificar` `generos` `formatos` | `exact` `formats` `suspected` · `unidentified` `genres` `formats` |

The builder finishes the inventory (any remaining Spanish value in a URL or API parameter follows the same rule)
and keeps the table in the plan up to date.

- **Database:** a schema migration (next version after the current one in `internal/store/store.go`) rewrites
  saved stations' stored definitions (`mode`, `view`, facet names and values) from the old vocabulary to the new.
  It is idempotent and leaves station names alone. As with every earlier version bump, a catalog that was not
  migrated cannot be opened in browse-only (read-only) mode (`ErrOldCatalog`); that is accepted. Anything else persisted with these words (the builder checks:
  marks, home cache, `startcache`) is migrated or rebuilt the same way.
- **Browser storage:** the queue and running station in `localStorage` may hold old station definitions. On load,
  an old definition is mapped to the new vocabulary once (a small pure function with tests); if it cannot be
  mapped, the running station is dropped and the queue kept.
- The Spanish `ARCHITECTURE`-style comments that say "as they appear in Explorar's URL" are updated with the
  constants.

### 4.6 Server errors that reach the UI

Today `http.Error(w, "álbum no encontrado", 404)` sends Spanish text that `api.js` turns into `ApiError.message`
and the pages show through `notify()`. About 95 `http.Error` call sites in `internal/server`, plus ~60
`errors.New` / `fmt.Errorf` with Spanish text across `internal/` and `cmd/`, some of which surface through
`err.Error()` (`jukebox.go`, `personal.go`, `playlists.go`, `settings.go`, `stations.go`, scan's `lastError`).

- Error responses become JSON: `{"code": "album_not_found", "message": "album not found", "params": {...}}` with
  the same HTTP status as today. `code` is a stable snake_case identifier; `message` is English; `params` carries
  values a message needs (`{"seconds": 12}` for the PIN lockout "Demasiados intentos. Probá de nuevo en %d s.").
- A single Go helper writes them (e.g. `writeError(w, status, code, message, params)`), and the codes are declared
  in one Go file so they can be listed by a test.
- Errors from deeper packages that the UI shows (validation in `personal`, `playlists`, `stations`, `settings`,
  jukebox role checks, root/token checks) get typed errors or sentinel errors that the server maps to codes. Errors
  with no code (a raw OS error, an unexpected failure) are sent with a generic code (`internal`, or a
  context-specific one) and their English `message`.
- `api.js` parses the JSON body when present: `ApiError` gains `code` and `params`; its `message` becomes
  `t('error.' + code, params)` when that key exists, else the server's English `message`, else
  `t('error.http', {status})`. A non-JSON body (proxy, old server) still works as today.
- The client-side network failure ("No se pudo contactar a Tordo. ¿Sigue abierto?") is a message.
- Scan's `lastError` stays a raw English string in `/api/status`; the status panel shows a translated lead-in
  ("The last scan failed") followed by that detail.
- Jukebox notices sent from the speaker to a remote (`POST /api/jukebox/notice`, `{remote, text}`) are produced by
  the speaker tab in its own language. They become `{remote, key, params}` so the phone shows them in **its**
  language (in practice the same, since the setting is per installation, but the speaker may still be on the
  automatic browser language). The builder keeps `text` accepted for compatibility only if it simplifies the
  change.
- Server-side 403s in listen mode and the jukebox "Solo el anfitrión puede hacer esto" follow the same code scheme.

### 4.7 Server logs and CLI

- All log lines (`log.Printf`/`Println`, ~48 sites) and CLI flag help (`cmd/tordo/main.go`) are rewritten in
  English. They are not translated: operators read them in English.
- `errors.New` / `fmt.Errorf` texts become English (they end up in logs, or as the English fallback `message`).

### 4.8 Help page

- `web/src/lib/ayuda.js` keeps `isHelpKey` and the section structure. The content moves to one module per language
  (e.g. `lib/i18n/help/es.js`, `en.js`, `pt-BR.js`), each exporting `SECTIONS` with the same `id`s, `href`s,
  `keys` and `blockedInListen` flags, and its own `title`, `intro`, `term` and `text`. *Design call:* `keys` hold
  the same stable tokens in every language (`'Ctrl+K'`, `'⌘K'`, `'?'`, `'/'`, `'Esc'`, `'Space'`); the page shows a
  token that is a word (`Space`) through a message (`key.space` → "Espacio", "Space", "Espaço"), so the parity
  test can compare `keys` exactly.
- `hrefs` use the new English routes.
- `Ayuda.svelte` picks the current language's sections (falling back to English for a missing language) and
  translates its own chrome ("Ayuda", the intro, "o", "No disponible en modo escucha.").

### 4.9 README

- `README.md` stays English and gets a language line at the top: `English · [Español](README.es.md) ·
  [Português](README.pt-BR.md)`.
- `README.es.md` and `README.pt-BR.md` are full translations with the same line (current language unlinked).
  Screenshots are shared. Code blocks (config example, commands) are not translated; the `config.json` example
  gains `"language": ""`.
- The English README's mentions of Spanish page names ("Bienvenida", "Inicio") are replaced by the English UI
  names.

### 4.10 Documentation rules

- `CLAUDE.md`'s rule "User-facing text (UI, log and error messages) in **Spanish**" is rewritten to: UI text lives
  in `web/src/lib/i18n/` in es, en and pt-BR (English is the reference; every new string goes in all three); server
  logs, CLI help and error `message`s are English; errors the UI shows carry a code; code, identifiers and comments
  in English.
- The main spec's "Interfaz en **español**" (§1) and the Spanish URL/facet names in §7–§8 are updated to point at
  this design.

## 5. Testing

All tests run with no network.

**Web (Vitest):**
- Key parity: every key in `en.js` exists in `es.js` and `pt-BR.js` and vice versa; the same `{placeholders}` per
  key; plural messages have at least `one` and `other` in each language.
- `t`: substitution, number formatting per locale, plural category choice (including `pt-BR` and `es` for 0, 1, 2,
  1000), fallback to English, then to the key.
- `resolveLocale`: setting wins; automatic walks `navigator.languages`; `pt-PT` → `pt-BR`; unknown → `en`.
- Existing tests of `format.js`, `status.js`, `facets.js`, `stations.js`, `router.js`, `ayuda.js`, `review.js`,
  `home.js`, `dupes.js` are updated to the new vocabulary and run for at least `es` and `en` where output is text.
- Help content parity: the three `SECTIONS` have the same ids, hrefs, keys and flags in the same order.
- `api.js`: JSON error with a known code → translated message; unknown code → server `message`; non-JSON body →
  as today.
- Old station definition mapping (localStorage).
- A leftover check: a test scans `web/src/**/*.{svelte,js}` outside `lib/i18n/` and tests for Spanish-looking text
  (accented letters `áéíóúñ¿¡` in string literals or template text), with a short allowlist (e.g. symbols, data
  samples). It catches most missed strings.

**Go:**
- The error helper writes `{code, message, params}` with the right status; handlers' existing tests assert codes
  instead of Spanish text.
- Every error code declared in Go has a key `error.<code>` in each of `web/src/lib/i18n/{en,es,pt-BR}.js` (a Go
  test that reads those files as text).
- `config`: `language` round-trips; absent and unknown values mean automatic.
- `/api/auth` and `/api/status` include `language`; `PUT /api/config` with only a language change does not start a
  scan.
- Store migration: old-vocabulary station definitions are rewritten; running it twice is a no-op; names untouched.
- Compilation marker: existing "Varios artistas" rows are migrated; search finds compilations by the three
  phrasings.

**Manual:** first run with the browser in each language; switch the setting in Ajustes and watch a second browser
(phone) follow within a status poll; PIN page on a phone; help page; a notice and an error in each language.

## 6. Left to the builder

- Message key naming within the dotted scheme, and how the message files are split if one gets unwieldy.
- How markup inside a sentence is handled where splitting is unworkable (§4.2).
- The compilation marker's exact representation (§4.4) and whether a migration or a rebuild updates old rows.
- The full inventory of Spanish vocabulary beyond the table in §4.5, following its rule.
- Whether `since` uses `Intl.RelativeTimeFormat` or messages (§4.3).
- Error codes' names and which deeper-package errors get typed errors versus a generic code (§4.6).
- Milestone order. A suggested split: (1) i18n layer, locale setting and formatting; (2) English vocabulary and
  store migration; (3) error codes; (4) move all UI strings and write the English and Portuguese translations;
  (5) help page, README, logs and CLI, CLAUDE.md and spec updates.
