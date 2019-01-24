package wire

import (
	"bytes"
	"io"
	"reflect"
	"testing"
)

var b1 = []byte{0, 0, 0, 3, 1, 0, 0, 0, 0, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 0, 3, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0}
var b2 = []byte{0, 0, 0, 3, 1, 0, 0, 0, 0, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 0, 3, 0, 0, 0, 0, 0, 0, 0, 2, 5, 0, 0, 0, 104, 101, 108, 108, 111}

var m1 = &Msgchat{1, 2, 3, 1, ""}
var m2 = &Msgchat{1, 2, 3, 2, "hello"}

func TestWriteMessage(t *testing.T) {
	type args struct {
		msg Message
	}
	tests := []struct {
		name    string
		args    args
		wantW   []byte
		wantErr bool
	}{
		// TODO: Add test cases.
		{"def", args{m1}, b1, false},
		{"def2", args{m2}, b2, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &bytes.Buffer{}
			if err := WriteMessage(w, tt.args.msg); (err != nil) != tt.wantErr {
				t.Errorf("WriteMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(w.Bytes(), tt.wantW) {
				t.Error(w.Bytes(), tt.wantW)
			}
		})
	}
}

func TestReadMessage(t *testing.T) {
	type args struct {
		r io.Reader
	}
	tests := []struct {
		name    string
		args    args
		want    Message
		wantErr bool
	}{
		// TODO: Add test cases.
		{"t1", args{bytes.NewReader(b1)}, m1, false},
		{"t2", args{bytes.NewReader(b2)}, m2, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadMessage(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReadMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}
