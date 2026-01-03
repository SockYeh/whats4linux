package store

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"log"
	"time"

	"github.com/AnimeKaizoku/cacher"
	query "github.com/lugvitc/whats4linux/internal/db"
	"github.com/lugvitc/whats4linux/internal/misc"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

type Message struct {
	Info    types.MessageInfo
	Content *waE2E.Message
}

const MaxMessageCacheSize = 50

type MessageStore struct {
	db *sql.DB
	// [chatJID.User] = ChatMessage
	chatListMap *cacher.Cacher[string, ChatMessage]
	mCache      misc.VMap[string, uint8]
}

// ExtractMessageText extracts a text representation from a WhatsApp message
func ExtractMessageText(msg *waE2E.Message) string {
	if msg.GetConversation() != "" {
		return msg.GetConversation()
	} else if msg.GetExtendedTextMessage() != nil {
		return msg.GetExtendedTextMessage().GetText()
	} else {
		switch {
		case msg.GetImageMessage() != nil:
			return "image"
		case msg.GetVideoMessage() != nil:
			return "video"
		case msg.GetAudioMessage() != nil:
			return "audio"
		case msg.GetDocumentMessage() != nil:
			return "document"
		case msg.GetStickerMessage() != nil:
			return "sticker"
		default:
			return "message"
		}
	}
}

func NewMessageStore() (*MessageStore, error) {
	db, err := sql.Open("sqlite3", misc.GetSQLiteAddress("mdb"))
	if err != nil {
		return nil, err
	}

	// Configure chat list cache (decentralized, separate from messages)
	chatListCache := cacher.NewCacher[string, ChatMessage](
		&cacher.NewCacherOpts{
			TimeToLive:    10 * time.Minute,
			Revaluate:     true,
			CleanInterval: 15 * time.Minute,
		},
	)

	ms := &MessageStore{
		db:          db,
		chatListMap: chatListCache,
		mCache:      misc.NewVMap[string, uint8](),
	}

	if err := ms.initSchema(); err != nil {
		return nil, err
	}

	return ms, nil
}

func (ms *MessageStore) initSchema() error {
	_, err := ms.db.Exec(query.CreateSchema)
	return err
}

func (ms *MessageStore) ProcessMessageEvent(msg *events.Message) {
	chat := msg.Info.Chat.User

	m := Message{
		Info:    msg.Info,
		Content: msg.Message,
	}

	// Update chatListMap with the new latest message
	messageText := ExtractMessageText(m.Content)
	sender := msg.Info.PushName
	if sender == "" && msg.Info.Sender.User != "" {
		sender = msg.Info.Sender.User
	}

	if msg.Info.IsFromMe {
		sender = "You"
	}

	chatMsg := ChatMessage{
		JID:         msg.Info.Chat,
		MessageText: messageText,
		MessageTime: msg.Info.Timestamp.Unix(),
		Sender:      sender,
	}
	ms.chatListMap.Set(chat, chatMsg)

	if _, exists := ms.mCache.Get(msg.Info.ID); exists {
		err := ms.updateMessageInDB(&m)
		if err != nil {
			log.Println(err)
		}
		return
	}
	ms.mCache.Set(msg.Info.ID, 1)

	err := ms.insertMessageToDB(&m)
	if err != nil {
		log.Println(err)
	}
}

func getMessageArrayFromRows(rows *sql.Rows) []Message {
	var (
		messages  []Message
		minf      []byte
		raw       []byte
		timestamp int64
	)

	for rows.Next() {
		minf = minf[:0]
		raw = raw[:0]

		if err := rows.Scan(&minf, &raw, &timestamp); err != nil {
			continue
		}

		var messageInfo types.MessageInfo
		if err := gobDecode(minf, &messageInfo); err != nil {
			continue
		}

		var waMsg *waE2E.Message
		waMsg, err := unmarshalMessageContent(raw)
		if err != nil {
			continue
		}

		messages = append(messages, Message{
			Info:    messageInfo,
			Content: waMsg,
		})
	}

	return messages
}

func buildMessageFromRawData(minf []byte, raw []byte) *Message {
	var messageInfo types.MessageInfo
	if err := gobDecode(minf, &messageInfo); err != nil {
		return nil
	}

	waMsg, err := unmarshalMessageContent(raw)
	if err != nil {
		return nil
	}

	return &Message{
		Info:    messageInfo,
		Content: waMsg,
	}
}

// GetMessagesPaged returns a page of messages for a chat
// beforeTimestamp: only return messages before this timestamp (0 = latest)
// limit: max number of messages to return
// Returns messages in chronological order (oldest first within the page)
// todo: optimize with caching
func (ms *MessageStore) GetMessagesPaged(jid types.JID, beforeTimestamp int64, limit int) []Message {
	var rows *sql.Rows
	var err error

	if beforeTimestamp == 0 {
		// Get latest messages using the optimized query
		rows, err = ms.db.Query(query.SelectLatestMessagesByChat, jid.String(), limit)
	} else {
		// Get messages before timestamp using the optimized query
		rows, err = ms.db.Query(query.SelectMessagesByChatBeforeTimestamp, jid.String(), beforeTimestamp, limit)
	}

	if err != nil {
		return []Message{}
	}

	defer rows.Close()

	return getMessageArrayFromRows(rows)
}

func (ms *MessageStore) GetMessage(chatJID types.JID, messageID string) *Message {
	row := ms.db.QueryRow(query.SelectMessageByChatAndID, chatJID.String(), messageID)
	var (
		minf []byte
		raw  []byte
	)

	if err := row.Scan(&minf, &raw); err != nil {
		return nil
	}

	return buildMessageFromRawData(minf, raw)
}

// GetMessageByID searches for a message by ID across all chats
func (ms *MessageStore) GetMessageByID(messageID string) *Message {

	// Query database
	var chat, msgID string
	var ts int64
	var minf, raw []byte

	err := ms.db.QueryRow(query.SelectMessageByID, messageID).Scan(&chat, &msgID, &ts, &minf, &raw)
	if err != nil {
		return nil
	}

	var messageInfo types.MessageInfo
	if err := gobDecode(minf, &messageInfo); err != nil {
		return nil
	}

	waMsg, err := unmarshalMessageContent(raw)
	if err != nil {
		return nil
	}

	return &Message{Info: messageInfo, Content: waMsg}
}

type ChatMessage struct {
	JID         types.JID
	MessageText string
	MessageTime int64
	Sender      string
}

func (ms *MessageStore) GetChatList() []ChatMessage {
	rows, err := ms.db.Query(query.SelectChatList)
	if err != nil {
		return []ChatMessage{}
	}
	defer rows.Close()

	var chatList []ChatMessage

	var (
		chat string
		ts   int64
		minf []byte
		raw  []byte
	)

	for rows.Next() {
		minf = minf[:0]
		raw = raw[:0]

		if err := rows.Scan(&chat, &ts, &minf, &raw); err != nil {
			continue
		}

		chatJID, err := types.ParseJID(chat)
		if err != nil {
			continue
		}

		// Check per-chat cache first
		if cachedChat, ok := ms.chatListMap.Get(chatJID.User); ok {
			chatList = append(chatList, cachedChat)
			continue
		}

		var messageInfo types.MessageInfo
		if err := gobDecode(minf, &messageInfo); err != nil {
			continue
		}

		var waMsg *waE2E.Message
		waMsg, err = unmarshalMessageContent(raw)
		if err != nil {
			continue
		}

		var messageText string
		messageText = ExtractMessageText(waMsg)

		chatMsg := ChatMessage{
			JID:         chatJID,
			MessageText: messageText,
			MessageTime: ts,
		}

		// Cache per-chat entry
		ms.chatListMap.Set(chatJID.User, chatMsg)
		chatList = append(chatList, chatMsg)
	}

	return chatList
}

func (ms *MessageStore) insertMessageToDB(msg *Message) error {
	msgInfo, err := gobEncode(msg.Info)
	if err != nil {
		return err
	}

	rawMessage, err := marshalMessageContent(msg.Content)
	if err != nil {
		return err
	}

	_, err = ms.db.Exec(query.InsertMessage,
		msg.Info.Chat.String(),
		msg.Info.ID,
		msg.Info.Timestamp.Unix(),
		msgInfo,
		rawMessage,
	)
	return err
}

func (ms *MessageStore) updateMessageInDB(msg *Message) error {
	msgInfo, err := gobEncode(msg.Info)
	if err != nil {
		return err
	}

	rawMessage, err := marshalMessageContent(msg.Content)
	if err != nil {
		return err
	}

	_, err = ms.db.Exec(query.UpdateMessage,
		msgInfo,
		rawMessage,
		msg.Info.ID,
	)
	return err
}

func marshalMessageContent(msg *waE2E.Message) ([]byte, error) {
	return proto.Marshal(msg)
}

func unmarshalMessageContent(data []byte) (*waE2E.Message, error) {
	var msg waE2E.Message
	if err := proto.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func gobEncode(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(v)
	return buf.Bytes(), err
}

func gobDecode(data []byte, v any) error {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	return dec.Decode(v)
}

func init() {
	gob.Register(&types.MessageInfo{})
}
