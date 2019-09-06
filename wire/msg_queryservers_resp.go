package wire

import (
	"encoding/json"
	"io"
)

// Server Server
type Server struct {
	Addr      string // logic address
	ClientURL string
	ServerURL string
}

// MsgQueryServersResp location message
type MsgQueryServersResp struct {
	Servers []Server
}

// Decode Decode
func (m *MsgQueryServersResp) Decode(r io.Reader) error {
	coder := json.NewDecoder(r)
	if err := coder.Decode(&m.Servers); err != nil {
		return err
	}
	return nil
}

// Encode Encode
func (m *MsgQueryServersResp) Encode(w io.Writer) error {
	// m.Servers = append(m.Servers, Server{Addr: "/c/1/1/1"})
	coder := json.NewEncoder(w)
	if err := coder.Encode(m.Servers); err != nil {
		return err
	}
	return nil
}
