package wire

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHeader_Decode(t *testing.T) {
	type fields struct {
		Source  Addr
		Dest    Addr
		Seq     uint32
		AckSeq  uint32
		Command uint8
	}
	type args struct {
		r io.Reader
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Header{
				Source:  tt.fields.Source,
				Dest:    tt.fields.Dest,
				Seq:     tt.fields.Seq,
				AckSeq:  tt.fields.AckSeq,
				Command: tt.fields.Command,
			}
			if err := h.Decode(tt.args.r); (err != nil) != tt.wantErr {
				t.Errorf("Header.Decode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMessage_encode_decode(t *testing.T) {

	msg, err := MakeEmptyHeaderMessage(MsgTypeChat, &Msgchat{Text: "hello"})
	assert.NoError(t, err)
	buf := &bytes.Buffer{}
	msg.Encode(buf)

	chat := new(Message)
	chat.Decode(buf)

	assert.Equal(t, MsgTypeChat, chat.Header.Command)
	assert.Equal(t, "hello", chat.Body.(*Msgchat).Text)
}
