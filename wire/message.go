package wire

import (
	"bytes"
	"fmt"
	"io"
)

const (
	// MsgTypeAck ack
	MsgTypeAck = uint8(1)
	// MsgTypeChat 单聊消息
	MsgTypeChat = uint8(3)
	// MsgTypeGroupInOut join or leave group
	MsgTypeGroupInOut = uint8(5)
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
	ID      uint32 //消息序列号，非全局唯一主健
	Msgtype uint8  // 指定消息的类型
	Scope   uint8  //指定消息是发给单人还是群
	To      string
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
	if header.Scope == ScopeNull {
		return header, nil
	}
	if header.To, err = ReadString(r); err != nil {
		return nil, err
	}
	return header, nil
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
	if header.Scope == ScopeNull {
		return nil
	}
	if err := WriteString(w, header.To); err != nil {
		return err
	}
	return nil
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
	case MsgTypeAck:
		msg = &MsgAck{header: header}
	case MsgTypeGroupInOut:
		msg = &MsgGroupInOut{header: header}
	default:
		return nil, fmt.Errorf("unhandled msgType[%d]", header.Msgtype)
	}

	return msg, nil
}

// MakeAckMessage make a ChatAck message
func MakeAckMessage(id uint32, state uint8) ([]byte, error) {
	ackHeader := &MessageHeader{ID: id, Msgtype: MsgTypeAck, Scope: ScopeNull}
	ackMessage, _ := MakeEmptyMessage(ackHeader)
	msgAck, _ := ackMessage.(*MsgAck)
	// set state sent
	msgAck.State = state

	buf := &bytes.Buffer{}
	err := WriteMessage(buf, msgAck)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
