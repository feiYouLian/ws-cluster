package database

// MessageStore message store
type MessageStore interface {
	Save(msgs ...interface{}) error
}
