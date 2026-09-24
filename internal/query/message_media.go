package query

const (
	CreateMessageMediaTable = `
	CREATE TABLE IF NOT EXISTS message_media (
		message_id TEXT PRIMARY KEY,
		type INTEGER NOT NULL,
		url TEXT,
		mimetype TEXT,
		direct_path TEXT,
		media_key BLOB,
		file_sha256 BLOB,
		file_enc_sha256 BLOB,
		width INTEGER,
		height INTEGER,
		file_name TEXT,
		gif_playback INTEGER DEFAULT 0,
		thumbnail BLOB
	);
	`

	// AddGifPlaybackColumn / AddThumbnailColumn migrate existing message_media
	// tables that predate those columns. SQLite has no ADD COLUMN IF NOT EXISTS,
	// so the caller ignores the "duplicate column" error on subsequent runs.
	AddGifPlaybackColumn = `
	ALTER TABLE message_media ADD COLUMN gif_playback INTEGER DEFAULT 0;
	`

	AddThumbnailColumn = `
	ALTER TABLE message_media ADD COLUMN thumbnail BLOB;
	`

	InsertMessageMedia = `
	INSERT INTO message_media
	(message_id, type, url, mimetype, direct_path, media_key, file_sha256, file_enc_sha256, width, height, file_name, gif_playback, thumbnail)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(message_id) DO UPDATE SET
		type = excluded.type,
		url = excluded.url,
		mimetype = excluded.mimetype,
		direct_path = excluded.direct_path,
		media_key = excluded.media_key,
		file_sha256 = excluded.file_sha256,
		file_enc_sha256 = excluded.file_enc_sha256,
		width = excluded.width,
		height = excluded.height,
		file_name = excluded.file_name,
		gif_playback = excluded.gif_playback,
		thumbnail = COALESCE(excluded.thumbnail, message_media.thumbnail);
	`

	SelectGifPlaybackByMessageID = `
	SELECT gif_playback FROM message_media WHERE message_id = ?;
	`

	SelectDimensionsByMessageID = `
	SELECT width, height FROM message_media WHERE message_id = ?;
	`

	SelectThumbnailByMessageID = `
	SELECT thumbnail FROM message_media WHERE message_id = ?;
	`

	UpdateMessageMediaByMessageID = `
	UPDATE message_media
	SET type = ?, url = ?, mimetype = ?, direct_path = ?, media_key = ?, file_sha256 = ?, file_enc_sha256 = ?, width = ?, height = ?, file_name = ?
	WHERE message_id = ?;
	`

	SelectMessageMediaByMessageID = `
	SELECT type, url, mimetype, direct_path, media_key, file_sha256, file_enc_sha256, width, height, file_name
	FROM message_media
	WHERE message_id = ?;
	`

	UpdateThumbnailByMessageID = `
	UPDATE message_media SET thumbnail = ? WHERE message_id = ?;
	`
)
