package database

// MessageStore message store
type MessageStore interface {
	Save(chatMsg *ChatMsg) error
}
