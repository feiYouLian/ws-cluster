package wire

import "io"

// MsgOffline client offline notice message
type MsgOffline struct {
	Peer Addr // target address
}

// Decode Decode
func (m *MsgOffline) Decode(r io.Reader) error {
	var err error
	if m.Peer.Decode(r); err != nil {
		return err
	}
	return nil
}

// Encode Encode
func (m *MsgOffline) Encode(w io.Writer) error {
	var err error
	if m.Peer.Encode(w); err != nil {
		return err
	}
	return nil
}
