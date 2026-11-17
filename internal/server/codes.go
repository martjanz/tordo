package server

// errCode is the stable identifier of an API error. The web app translates it
// as error.<code> in web/src/lib/i18n; TestErrorCodesTranslated keeps the two
// lists in step, so add the key to all three message files with a new code.
type errCode string

const (
	// Generic.
	codeInternal         errCode = "internal"
	codeForbidden        errCode = "forbidden"
	codeNotFound         errCode = "not_found"
	codeInvalidBody      errCode = "invalid_body"
	codeInvalidID        errCode = "invalid_id"
	codeJSONRequired     errCode = "json_required"
	codeReadOnly         errCode = "read_only"
	codeListenMode       errCode = "listen_mode"
	codeSetupPending     errCode = "setup_pending"
	codeSettingsLocal    errCode = "settings_local_only"
	codeAudioLocal       errCode = "audio_local_only"
	codeScanRunning      errCode = "scan_running"
	codeConfigSaveFailed errCode = "config_save_failed"

	// Things that do not exist.
	codeAlbumNotFound         errCode = "album_not_found"
	codeArtistNotFound        errCode = "artist_not_found"
	codeFolderNotFound        errCode = "folder_not_found"
	codeTrackNotFound         errCode = "track_not_found"
	codePlaylistNotFound      errCode = "playlist_not_found"
	codePlaylistTrackNotFound errCode = "playlist_track_not_found"
	codeStationNotFound       errCode = "station_not_found"
	codeNoCover               errCode = "no_cover"
	codeReleaseNotFound       errCode = "release_not_found"

	// Bad requests.
	codeInvalidPeriod      errCode = "invalid_period"
	codeInvalidKind        errCode = "invalid_kind"
	codeInvalidStation     errCode = "invalid_station"
	codeInvalidRating      errCode = "invalid_rating"
	codeInvalidMBID        errCode = "invalid_musicbrainz_id"
	codeInvalidTrackCount  errCode = "invalid_track_count"
	codeMissingID          errCode = "missing_id"
	codeMissingKey         errCode = "missing_key"
	codeMissingToken       errCode = "missing_token"
	codeMissingRaw         errCode = "missing_raw"
	codeMissingRelease     errCode = "missing_release"
	codeNothingToPlay      errCode = "nothing_to_play"
	codeUnknownGenre       errCode = "unknown_genre"
	codeUnknownAddress     errCode = "unknown_address"
	codeMusicBrainzOffline errCode = "musicbrainz_offline"

	// Names and playlists.
	codePlaylistNameRequired errCode = "playlist_name_required"
	codePlaylistFull         errCode = "playlist_full"
	codePlaylistOrder        errCode = "playlist_order_mismatch"
	codeStationNameRequired  errCode = "station_name_required"

	// Network access.
	codePINRequired     errCode = "pin_required"
	codePINWrong        errCode = "pin_wrong"
	codePINNotNeeded    errCode = "pin_not_needed"
	codePINFormat       errCode = "invalid_pin_format"
	codePINNeededForLAN errCode = "pin_needed_for_lan"
	codeTooManyAttempts errCode = "too_many_attempts"
	codeLANDisabled     errCode = "lan_disabled"
	codeInvalidPort     errCode = "invalid_port"
	codePortUnavailable errCode = "port_unavailable"
	codeInvalidRescan   errCode = "invalid_rescan"
	codeUnknownLanguage errCode = "unknown_language"
	codeInvalidIgnore   errCode = "invalid_ignore_pattern"
	codeTooManyIgnore   errCode = "too_many_ignore"
	codeRootRequired    errCode = "root_required"
	codeRootNotFound    errCode = "root_not_found"
	codeRootNotDir      errCode = "root_not_directory"
	codeRootOtherDisk   errCode = "root_other_disk"
	codeRootIsApp       errCode = "root_is_app"
	codeRootContainsApp errCode = "root_contains_app"
	codeRootInsideApp   errCode = "root_inside_app"
	codeRootsRequired   errCode = "roots_required"
	codeRootMalformed   errCode = "root_malformed"
	codeRootDuplicate   errCode = "root_duplicate"
	codeRootsNested     errCode = "roots_nested"

	// Jukebox.
	codeHostOnly       errCode = "host_only"
	codeNoSpeaker      errCode = "no_speaker"
	codeNotSpeaker     errCode = "not_speaker"
	codeUnknownCommand errCode = "unknown_command"
	codeUnknownRemote  errCode = "unknown_remote"
	codeTooManyRemotes errCode = "too_many_remotes"
	codeSpeakerBusy    errCode = "speaker_busy"
)
