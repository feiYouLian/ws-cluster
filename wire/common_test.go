// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"log"
	"reflect"
	"testing"
)

func TestReadString(t *testing.T) {
	bys := []byte{1, 2, 3}
	buf := bytes.NewReader(bys)
	val, _ := ReadUint8(buf)
	val2, _ := ReadUint8(buf)
	log.Println(val, val2)
	log.Println(bys)
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
