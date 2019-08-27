package wire

import "io"

// MsgLoc location message
type MsgLoc struct {
	Target Addr // the peer be noticed
	Peer   Addr // this Peer
	In     Addr //Peer in server
}

// Decode Decode
func (m *MsgLoc) Decode(r io.Reader) error {
	var err error
	if m.Target.Decode(r); err != nil {
		return err
	}
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
	if m.Target.Encode(w); err != nil {
		return err
	}
	if m.Peer.Encode(w); err != nil {
		return err
	}
	if m.In.Encode(w); err != nil {
		return err
	}
	return nil
}
