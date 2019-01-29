package wire

import (
	"fmt"
	"io"
)

const (
	// MsgTypeChat 单聊消息
	MsgTypeChat = uint8(3)
	// MsgTypeChatAck ack
	MsgTypeChatAck = uint8(4)
)

// Message 定义了消息接口，消息必须有序列化和反序列化方法
type Message interface {
	Decode(io.Reader) error
	Encode(io.Writer) error
	// Msgtype
	Msgtype() uint8
}

// 消息头中前8位是 msgType
// 00000000 00000000 00000000 00000000
// msgtype
func readHeader(r io.Reader) (uint32, error) {
	header, err := ReadUint32(r)
	if err != nil {
		return 0, err
	}
	return header, nil
}

// ReadMessage 从reader 中读取消息
func ReadMessage(r io.Reader) (Message, error) {
	header, err := readHeader(r)
	if err != nil {
		return nil, err
	}
	msgType := uint8(header >> 24)
	msg, err := makeEmptyMessage(msgType)
	if err != nil {
		return nil, err
	}
	if err = msg.Decode(r); err != nil {
		return nil, err
	}
	return msg, nil
}

// WriteHeader write header to writer
func WriteHeader(w io.Writer, msg Message) error {
	header := uint32(msg.Msgtype()) << 24
	if err := WriteUint32(w, header); err != nil {
		return err
	}
	return nil
}

// WriteMessage 把 msg 写到 w 中
func WriteMessage(w io.Writer, msg Message) error {
	if err := WriteHeader(w, msg); err != nil {
		return err
	}
	if err := msg.Encode(w); err != nil {
		return err
	}
	return nil
}

func makeEmptyMessage(msgType uint8) (Message, error) {
	var msg Message
	switch uint8(msgType) {
	case MsgTypeChat:
		msg = &Msgchat{}
	default:
		return nil, fmt.Errorf("unhandled msgType[%d]", msgType)
	}

	return msg, nil
}
