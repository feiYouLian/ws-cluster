package wire

import "io"

// MsgEmpty empty message
type MsgEmpty struct {
}

// Decode Decode
func (m *MsgEmpty) Decode(r io.Reader) error {
	return nil
}

// Encode Encode
func (m *MsgEmpty) Encode(w io.Writer) error {
	return nil
}
