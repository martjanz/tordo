// Help content in English. Same structure, ids, keys, links and flags as es.js:
// only title, term and text are translated.
export const SECTIONS = [
  {
    id: 'shortcuts',
    title: 'Shortcuts and commands',
    items: [
      { keys: ['Ctrl+K', '⌘K'], text: 'Opens search on any page. Arrow keys and Enter pick a result; Esc closes it.' },
      { keys: ['Space'], text: 'Plays or pauses (unless you are typing or on a button).' },
      { keys: ['/'], text: 'In Folders, jumps to the tree filter.' },
      { keys: ['Esc'], text: 'Closes open menus and panels.' },
      { keys: ['?'], text: 'Opens this help.' },
      { term: 'Player', text: 'Shuffle, previous, play or pause, next and repeat (off, the whole queue or this track).' },
      { term: 'Position and volume', text: 'One bar to skip ahead or back and another for the volume, with a mute button. Formats that are converted on the fly cannot be skipped ahead.' },
      { term: 'Queue', text: 'The queue button shows what is going to play; there you can remove tracks and reorder them by dragging (or with Alt + arrows).' },
      { term: 'Rating', text: 'With focus on the stars, the arrow keys change the rating.' },
    ],
  },
  {
    id: 'sections',
    title: 'Sections',
    items: [
      { term: 'Home', href: '/', text: 'Rows to pick up where you left off: your stations, keep listening, your favorite songs and your preferred albums, plus suggestions (such as recently added) that change, like the order, on every visit or with the refresh button.' },
      { term: 'Explore', href: '/explore', text: 'The whole collection by albums, artists or tracks, with combinable filters (decade, country, genre, format, ♥, ★…) and sorting, shuffle included.' },
      { term: 'Folders', href: '/folders', text: 'The collection as it is on the disk, with a folder tree.' },
      { term: 'Lists', href: '/lists', text: 'Your playlists and your ♥.' },
      { term: 'Review', href: '/review', blockedInListen: true, text: 'Maintenance: unidentified music, duplicates, genres to map and formats to convert.' },
      { term: 'Settings', href: '/settings', text: "Music folders, Discogs token, volume, this computer's audio, listen mode and local network." },
    ],
  },
  {
    id: 'concepts',
    title: 'Concepts',
    items: [
      { term: 'Stations', text: 'An endless queue with music similar to what you picked (an artist, a country, a genre, a folder, your ♥ and your preferred albums…). Tordo feeds it ten tracks at a time, varies the artists and gives more weight to what you like. You can save a station and you will see it in Home.' },
      { term: '♥, bookmark and ★', text: 'The ♥ marks your favorite songs (and the artists and collections you like); the bookmark marks preferred albums, a short list to come back to; the ★ rates tracks and albums from 1 to 5. They work as filters in Explore (in Albums, "Preferred"), they build stations and they weigh on them.' },
      { term: 'Duplicates', href: '/review/duplicates', blockedInListen: true, text: 'Tordo shows them and never deletes them. Exact: the same audio at two or more paths (shows the recoverable space). Formats: the same recording in several formats inside an album, with the BEST mark on the one with the highest quality. Suspects: same artist and title on different albums; it is informational and usually legitimate (a song on its album and on a compilation).' },
      { term: 'Listen mode', href: '/settings', text: 'A Settings option that hides Review and curation and locks identifications, scans and genre changes. Likes (♥), ratings (★), lists and stations keep working. It is useful for leaving the music to other people.' },
      { term: 'Jukebox', href: '/settings', text: 'On the computer running Tordo, turn on "Use this computer\'s audio for Tordo" in Settings → Playback and keep that tab open: the sound comes out there. From a phone on the network, the 🎵 Jukebox button in the top bar makes the player control the computer instead of the phone. If you reload that tab, one click on "Turn audio back on" brings the sound back.' },
      { term: 'Hosts and guests', href: '/settings#network', text: 'The computer is always a host; other devices are guests unless you mark them as "Jukebox host" in Settings → Local network. Hosts control everything. Guests can only contribute: ▶ adds to the queue (and starts playing if the queue was empty or finished), and they can pause and remove only what they added.' },
    ],
  },
  {
    id: 'how-it-works',
    title: 'How it works: scanning and identification',
    items: [
      { term: 'When it scans', text: 'When Tordo opens, every time you change the music folders or the folders to ignore in Settings and, every hour, to look for changes on the disk (the interval is chosen in Settings, and it can be turned off). That check compares sizes and dates without opening the files: if it finds no differences it ends there and you do not notice; if it does, it carries on as a normal scan. Scanning is incremental: it only reads new or modified files (by size and date), so later scans are much faster than the first. Meanwhile the top bar shows which step it is on: "Finding files" (the walk), "Reading files" with how many it has done and "Assembling albums"; hovering over or tapping that indicator opens a panel with the steps, the progress, how much is left and the catalog figures. Music shows up as it is read: the catalog is rebuilt every half minute and the pages update by themselves.' },
      { term: '1. Walk', text: 'Tordo walks the folders looking for audio (MP3, FLAC, M4A, Ogg, Opus, WMA, WAV, AIFF, APE, WavPack), cover images, lyrics and M3U lists. It skips the folders to ignore and those of other applications (Ableton, djay, Apple Music, half-finished Soulseek downloads). If a disk is not connected, it leaves it out and keeps what it already knew; a file that is gone is marked as missing, not forgotten.' },
      { term: '2. Reading', text: "From each new file it reads the tags (title, artist, album, track number, year, genres, cover included) and the technical data (format, duration, quality), and computes a fingerprint of the audio. The fingerprint ignores the tags: a file that was moved, renamed or re-tagged is still the same one, and two identical copies are recognized as duplicates. If ffprobe is installed, it also reads the formats Tordo cannot read by itself. An unreadable file is retried on the next scan." },
      { term: '3. Albums', text: 'It assembles the albums from the tags and, when they are missing, from folder and file names ("Artist - Album (1985)", "CD1"). Discs of one album in subfolders are put together; a few tracks with a different album tag are added to the album of their folder; a folder of loose tracks from different albums stays as a selection named after the folder. It picks the cover (cover, folder or front, otherwise the largest one in the folder) and detects collections: folders with several albums by different artists.' },
      { term: '4. Identification', text: 'When each scan ends, Tordo looks up each album on MusicBrainz, one query per second as MusicBrainz asks (with thousands of albums the first time it takes hours; if it is interrupted, it resumes where it left off). The top bar shows "Identifying" and how many it has done. If the tags already carry the MusicBrainz identifier, it uses it. If not, it searches by title and artist (also as the folder names them and without notes in parentheses) and scores each candidate: title 30%, track durations 25%, artist 20%, track count 15% and year 10%. At 80% or more it identifies the album by itself; otherwise it stays in Review → Unidentified with up to five candidates to choose from.' },
      { term: '5. Data it adds', text: "An identified album shows the MusicBrainz titles (for each track too), the original year, the artist's country and, if the folder has no cover, the one from the Cover Art Archive (saved in cache/). While it looks for this data the top bar shows \"Completing data\"; when it finishes, \"Up to date\". Genres and styles come from Discogs: through the links MusicBrainz has and, with a Discogs token in Settings, by searching too. Without Discogs, from MusicBrainz; without identification, from the tags. Genres in the tags that Tordo does not recognize stay in Review → Genres to map." },
      { term: 'Your corrections', text: "What you choose by hand (in Unidentified or with \"Correct the identification\" on the album page) is saved by each track's fingerprint: it survives if you move, rename or re-tag the files, and automatic identification never overwrites it. None of this touches your files: the tags are not rewritten." },
      { term: 'No connection', text: 'Scanning does not use the network. If there is no connection, identification pauses (the bar shows "No connection to MusicBrainz") and retries by itself, further apart each time; everything else works the same. What is already identified is not looked up again.' },
    ],
  },
  {
    id: 'guidelines',
    title: 'Usage guidelines',
    items: [
      { term: 'Your files', text: 'Tordo never writes, moves, renames or deletes your audio files. It only writes in its own folder: tordo.db (the catalog), config.json (the settings) and cache/.' },
      { term: 'Portability', text: 'The catalog stores paths relative to the Tordo folder, so you can move the whole folder (for example to another disk) and everything keeps working.' },
      { term: 'From other devices', href: '/settings#network', text: "If you turn on the local network in Settings, you can open Tordo from a phone or a tablet on the same network. Each device asks for the PIN you chose; changing it signs out every session. To make it play on the computer's speakers, use the Jukebox." },
      { term: 'Settings', text: 'They can only be changed from the computer running Tordo; from another device you can listen and browse, but not configure.' },
    ],
  },
]
