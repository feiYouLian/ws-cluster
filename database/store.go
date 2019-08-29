package database

// MessageStore message store
type MessageStore interface {
	SaveChatMsg(msgs []*ChatMsg) error
	SaveGroupMsg(msgs []*GroupMsg) error
}
