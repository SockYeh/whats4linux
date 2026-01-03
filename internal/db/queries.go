package query

const (
	CreateGroupsTable = `
	CREATE TABLE IF NOT EXISTS whats4linux_groups (
		jid TEXT PRIMARY KEY,
		name TEXT,
		topic TEXT,
		owner_jid TEXT,
		participant_count INTEGER
	);
	`

	InsertOrReplaceGroup = `
	INSERT OR REPLACE INTO whats4linux_groups
	(jid, name, topic, owner_jid, participant_count)
	VALUES (?, ?, ?, ?, ?);
	`

	SelectAllGroups = `
	SELECT jid, name, topic, owner_jid, participant_count
	FROM whats4linux_groups;
	`

	SelectGroupByJID = `
	SELECT jid, name, topic, owner_jid, participant_count
	FROM whats4linux_groups
	WHERE jid = ?;
	`

	CreateSchema = `
	CREATE TABLE IF NOT EXISTS messages (
		chat TEXT NOT NULL,
		message_id TEXT PRIMARY KEY,
		timestamp INTEGER,
		msg_info BLOB,
		raw_message BLOB
	);

	CREATE INDEX IF NOT EXISTS idx_messages_chat_time
	ON messages(chat, timestamp DESC);
	`

	InsertMessage = `
	INSERT INTO messages
	(chat, message_id, timestamp, msg_info, raw_message)
	VALUES (?, ?, ?, ?, ?)
	`

	UpdateMessage = `
	UPDATE messages
	SET msg_info = ?, raw_message = ?
	WHERE message_id = ?;
	`

	SelectChatList = `
	SELECT chat, timestamp, msg_info, raw_message
	FROM (
		SELECT 
			chat, timestamp, msg_info, raw_message,
			ROW_NUMBER() OVER (
				PARTITION BY chat
				ORDER BY timestamp DESC, rowid DESC
			) AS rn
		FROM messages
	)
	WHERE rn = 1
	ORDER BY timestamp DESC;
	`

	SelectMessagesByChatBeforeTimestamp = `
	SELECT msg_info, raw_message, timestamp
	FROM (
		SELECT msg_info, raw_message, timestamp
		FROM messages
		WHERE chat = ? AND timestamp < ?
		ORDER BY timestamp DESC
		LIMIT ?
	)
	ORDER BY timestamp ASC
	`

	SelectLatestMessagesByChat = `
	SELECT msg_info, raw_message, timestamp
	FROM (
		SELECT msg_info, raw_message, timestamp
		FROM messages
		WHERE chat = ?
		ORDER BY timestamp DESC
		LIMIT ?
	)
	ORDER BY timestamp ASC
	`

	SelectMessageByChatAndID = `
	SELECT msg_info, raw_message
	FROM messages
	WHERE chat = ? AND message_id = ?
	LIMIT 1
	`

	SelectMessageByID = `
	SELECT chat, message_id, timestamp, msg_info, raw_message
	FROM messages
	WHERE message_id = ?
	LIMIT 1
	`

	// Image cache queries
	CreateImageIndexTable = `
	CREATE TABLE IF NOT EXISTS image_index (
		message_id TEXT PRIMARY KEY,
		sha256     TEXT NOT NULL,
		mime       TEXT,
		width      INTEGER,
		height     INTEGER,
		created_at INTEGER
	);
	CREATE INDEX IF NOT EXISTS idx_sha ON image_index (sha256);
	`

	SaveImageIndex = `
	INSERT OR REPLACE INTO image_index
	(message_id, sha256, mime, width, height, created_at)
	VALUES (?, ?, ?, ?, ?, ?)
	`

	DeleteImageIndex = `
	DELETE FROM image_index
	WHERE message_id = ?
	`

	GetImageByID = `
	SELECT message_id, sha256, mime, width, height, created_at
	FROM image_index
	WHERE message_id = ?
	`

	// GetImagesByIDsPrefix is the prefix used to query multiple image IDs.
	// Use it with a dynamically built placeholder list, e.g.
	// q := query.GetImagesByIDsPrefix + strings.Join(placeholders, ",") + ")"
	GetImagesByIDsPrefix = `
	SELECT message_id, sha256, mime, width, height, created_at
	FROM image_index
	WHERE message_id IN (
	`

	GetImagesByIDs = `
	SELECT message_id, sha256, mime, width, height, created_at
	FROM image_index
	WHERE message_id IN (?)
	`
)
