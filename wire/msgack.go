package wire

import (
	"io"
)

const (
	// AckStateFail 发送失败
	AckStateFail = uint8(1)
	// AckStateSent 发送成功，表示数据写 db 成功
	AckStateSent = uint8(2)
	//AckStateRead 已读，依赖接受方的响应
	AckStateRead = uint8(3)
)

// MsgAck 单聊消息应答
type MsgAck struct {
	header *MessageHeader
	State  uint8 //
	Desc   string
}

// decode Decode
func (m *MsgAck) decode(r io.Reader) error {
	var err error
	if m.State, err = ReadUint8(r); err != nil {
		return err
	}
	if m.Desc, err = ReadString(r); err != nil {
		return err
	}
	return nil
}

// encode Encode
func (m *MsgAck) encode(w io.Writer) error {
	var err error
	if err = WriteUint8(w, m.State); err != nil {
		return err
	}
	if err = WriteString(w, m.Desc); err != nil {
		return err
	}
	return nil
}

// Header 头信息
func (m *MsgAck) Header() *MessageHeader {
	return &MessageHeader{m.header.ID, MsgTypeAck, ScopeChat, m.header.To}
}
