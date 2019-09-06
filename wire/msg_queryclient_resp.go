package wire

import (
	"encoding/json"
	"io"
)

// MsgQueryClientResp location message
type MsgQueryClientResp struct {
	LoginAt uint32 // 0 not online
}

// Decode Decode
func (m *MsgQueryClientResp) Decode(r io.Reader) error {
	dec := json.NewDecoder(r)
	if err := dec.Decode(&m.LoginAt); err != nil {
		return err
	}
	return nil
}

// Encode Encode
func (m *MsgQueryClientResp) Encode(w io.Writer) error {
	coder := json.NewEncoder(w)
	if err := coder.Encode(m.LoginAt); err != nil {
		return err
	}
	return nil
}
