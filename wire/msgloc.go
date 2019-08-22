package wire

import "io"

// MsgLoc location message
type MsgLoc struct {
	Peer Addr // target Peer address
	In   Addr //Peer in server
}

// Decode Decode
func (m *MsgLoc) Decode(r io.Reader) error {
	var err error
	if m.Peer.Decode(r); err != nil {
		return err
	}
	if m.In.Decode(r); err != nil {
		return err
	}
	return nil
}

// Encode Encode
func (m *MsgLoc) Encode(w io.Writer) error {
	var err error
	if m.Peer.Encode(w); err != nil {
		return err
	}
	if m.In.Encode(w); err != nil {
		return err
	}
	return nil
}
