package wire

import (
	"io"
)

// MsgQueryServers location message
type MsgQueryServers struct {
}

// Decode Decode
func (m *MsgQueryServers) Decode(r io.Reader) error {
	return nil
}

// Encode Encode
func (m *MsgQueryServers) Encode(w io.Writer) error {
	return nil
}
