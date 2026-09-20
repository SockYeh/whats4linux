package query

const (
	CreateCallHistoryTable = `
	CREATE TABLE IF NOT EXISTS call_history (
		call_id TEXT PRIMARY KEY,
		peer_jid TEXT NOT NULL,
		chat_jid TEXT NOT NULL,
		direction TEXT NOT NULL,
		state TEXT NOT NULL,
		reason TEXT DEFAULT '',
		started_at INTEGER NOT NULL,
		ended_at INTEGER DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_call_history_started ON call_history(started_at DESC);
	CREATE INDEX IF NOT EXISTS idx_call_history_chat ON call_history(chat_jid, started_at DESC);
	`

	InsertCallHistory = `
	INSERT OR REPLACE INTO call_history
	(call_id, peer_jid, chat_jid, direction, state, reason, started_at, ended_at)
	VALUES (?, ?, ?, ?, ?, '', ?, 0)
	`

	EndCallHistory = `
	UPDATE call_history
	SET state = ?, reason = ?, ended_at = ?
	WHERE call_id = ?
	`
)
