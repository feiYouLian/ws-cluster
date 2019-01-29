// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
)

func TestReadString(t *testing.T) {
	var buf bytes.Buffer

	str := "hello 你好"
	err := WriteString(&buf, str)
	if err != nil {
		t.Error(err)
	}

	err = WriteString(&buf, str)
	if err != nil {
		t.Error(err)
	}

	str2, err := ReadString(&buf)
	if err != nil {
		t.Error(err)
	}
	if str2 != str {
		t.Error(str2)
	}

	str2, _ = ReadString(&buf)
	if str2 != str {
		t.Error(str2)
	}
	fmt.Println(str2)

	b1 := int8(-127)
	fmt.Println(b1)

	b2 := uint8(b1)
	fmt.Println(b2)

	b1 = int8(b2)
	fmt.Println(b1)
}

func TestWriteInt8(t *testing.T) {
	type args struct {
		val uint8
	}
	tests := []struct {
		name    string
		args    args
		wantW   []byte
		wantErr bool
	}{
		// TODO: Add test cases.
		{"def", args{1}, []byte{1}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &bytes.Buffer{}
			if err := WriteUint8(w, tt.args.val); (err != nil) != tt.wantErr {
				t.Errorf("WriteInt8() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotW := w.Bytes(); !reflect.DeepEqual(gotW, tt.wantW) {
				t.Errorf("WriteInt8() = %v, want %v", gotW, tt.wantW)
			}
		})
	}
}
