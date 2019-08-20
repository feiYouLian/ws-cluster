package wire

import "io"

// MsgKill 通知连接下线
type MsgKill struct {
	PeerID string
}

// Decode Decode
func (m *MsgKill) Decode(r io.Reader) error {
	var err error
	if m.PeerID, err = ReadString(r); err != nil {
		return err
	}
	return nil
}

// Encode Encode
func (m *MsgKill) Encode(w io.Writer) error {
	var err error
	if err = WriteString(w, m.PeerID); err != nil {
		return err
	}
	return nil
}
