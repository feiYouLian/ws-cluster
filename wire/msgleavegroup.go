package wire

import (
	"io"
	"strings"
)

// MsgLeaveGroup 单聊消息应答
type MsgLeaveGroup struct {
	header *MessageHeader
	Groups []string
}

// decode Decode
func (m *MsgLeaveGroup) decode(r io.Reader) error {
	var err error
	grouparr, err := ReadString(r)
	if err != nil {
		return err
	}
	m.Groups = strings.Split(grouparr, ",")
	return nil
}

// encode Encode
func (m *MsgLeaveGroup) encode(w io.Writer) error {
	var err error
	if err = WriteString(w, strings.Join(m.Groups, ",")); err != nil {
		return err
	}
	return nil
}

// Header 头信息
func (m *MsgLeaveGroup) Header() *MessageHeader {
	return &MessageHeader{m.header.ID, MsgTypeLeaveGroup, 0, ""}
}
