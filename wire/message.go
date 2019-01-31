package wire

import (
	"bytes"
	"fmt"
	"io"
)

const (
	// MsgTypeChat 单聊消息
	MsgTypeChat = uint8(3)
	// MsgTypeChatAck ack
	MsgTypeChatAck = uint8(4)
	// MsgTypeGroup group
	MsgTypeGroup = uint8(5)
	// MsgTypeJoinGroup join group
	MsgTypeJoinGroup = uint8(7)
	// MsgTypeLeaveGroup leave group
	MsgTypeLeaveGroup = uint8(9)
)

const (
	// ScopeNull no target
	ScopeNull = uint8(0)
	// ScopeChat msg to client
	ScopeChat = uint8(1)
	// ScopeGroup msg to a group
	ScopeGroup = uint8(3)
)

// MessageHeader MessageHeader
type MessageHeader struct {
	ID      uint32
	Msgtype uint8
	Scope   uint8
	To      []byte
}

// StringTo convert to string
func (h *MessageHeader) StringTo() (string, error) {
	buf := bytes.NewReader(h.To)
	val, err := ReadString(buf)
	if err != nil {
		return "", err
	}
	return val, nil
}

// Uint64To convert to  uint64
func (h *MessageHeader) Uint64To() (uint64, error) {
	buf := bytes.NewReader(h.To)
	val, err := ReadUint64(buf)
	if err != nil {
		return 0, err
	}
	return val, nil
}

// Message 定义了消息接口，消息必须有序列化和反序列化方法
type Message interface {
	decode(io.Reader) error
	encode(io.Writer) error
	Header() *MessageHeader
}

// ReadHeader read header
func ReadHeader(r io.Reader) (*MessageHeader, error) {
	header := &MessageHeader{}
	var err error
	if header.ID, err = ReadUint32(r); err != nil {
		return nil, err
	}
	if header.Msgtype, err = ReadUint8(r); err != nil {
		return nil, err
	}
	if header.Scope, err = ReadUint8(r); err != nil {
		return nil, err
	}
	if header.To, err = ReadBytes(r); err != nil {
		return nil, err
	}
	return header, nil
}

// ReadMessage 从reader 中读取消息
func ReadMessage(r io.Reader) (Message, error) {
	header, err := ReadHeader(r)
	if err != nil {
		return nil, err
	}
	msg, err := MakeEmptyMessage(header)
	if err != nil {
		return nil, err
	}
	if err = msg.decode(r); err != nil {
		return nil, err
	}
	return msg, nil
}

// WriteHeader write header to writer
func WriteHeader(w io.Writer, msg Message) error {
	header := msg.Header()
	if err := WriteUint32(w, header.ID); err != nil {
		return err
	}
	if err := WriteUint8(w, header.Msgtype); err != nil {
		return err
	}
	if err := WriteUint8(w, header.Scope); err != nil {
		return err
	}
	if err := WriteBytes(w, header.To); err != nil {
		return err
	}
	return nil
}

// WriteMessage 把 msg 写到 w 中
func WriteMessage(w io.Writer, msg Message) error {
	if err := WriteHeader(w, msg); err != nil {
		return err
	}
	if err := msg.encode(w); err != nil {
		return err
	}
	return nil
}

// MakeEmptyMessage 创建一个空的消息体
func MakeEmptyMessage(header *MessageHeader) (Message, error) {
	var msg Message
	switch uint8(header.Msgtype) {
	case MsgTypeChat:
		msg = &Msgchat{header: header}
	case MsgTypeChatAck:
		msg = &MsgchatAck{header: header}
	case MsgTypeGroup:
		msg = &Msggroup{header: header}
	case MsgTypeJoinGroup:
		msg = &MsgJoinGroup{header: header}
	case MsgTypeLeaveGroup:
		msg = &MsgLeaveGroup{header: header}
	default:
		return nil, fmt.Errorf("unhandled msgType[%d]", header.Msgtype)
	}

	return msg, nil
}
