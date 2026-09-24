package store

import (
	"database/sql"
	"log"

	"github.com/lugvitc/whats4linux/internal/query"
)

func (ms *MessageStore) CallStarted(id, peer, chatJID, direction, state string, startedAt int64) {
	if ms == nil {
		return
	}
	if err := ms.runSync(func(tx *sql.Tx) error {
		_, err := tx.Exec(query.InsertCallHistory, id, peer, chatJID, direction, state, startedAt)
		return err
	}); err != nil {
		log.Println("call history start failed:", err)
	}
}

func (ms *MessageStore) CallEnded(id, state, reason string, endedAt int64) {
	if ms == nil {
		return
	}
	if err := ms.runSync(func(tx *sql.Tx) error {
		_, err := tx.Exec(query.EndCallHistory, state, reason, endedAt, id)
		return err
	}); err != nil {
		log.Println("call history end failed:", err)
	}
}
