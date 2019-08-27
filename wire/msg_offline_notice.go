package wire

import "io"

// MsgOfflineNotice client offline notice message
type MsgOfflineNotice struct {
	Peer Addr // target address
}

// Decode Decode
func (m *MsgOfflineNotice) Decode(r io.Reader) error {
	var err error
	if m.Peer.Decode(r); err != nil {
		return err
	}
	return nil
}

// Encode Encode
func (m *MsgOfflineNotice) Encode(w io.Writer) error {
	var err error
	if m.Peer.Encode(w); err != nil {
		return err
	}
	return nil
}
