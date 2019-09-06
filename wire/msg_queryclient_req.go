package wire

import (
	"io"
)

// MsgQueryClient location message
type MsgQueryClient struct {
	Peer Addr // target Peer address
}

// Decode Decode
func (m *MsgQueryClient) Decode(r io.Reader) error {
	var err error
	if m.Peer.Decode(r); err != nil {
		return err
	}
	return nil
}

// Encode Encode
func (m *MsgQueryClient) Encode(w io.Writer) error {
	var err error
	if m.Peer.Encode(w); err != nil {
		return err
	}
	return nil
}
