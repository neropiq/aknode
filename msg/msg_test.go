// Copyright (c) 2018 Aidos Developer

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package msg

import (
	"bytes"
	"testing"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aknode/setting"
)

func TestMsg(t *testing.T) {
	s := &setting.Setting{
		DBConfig: aklib.DBConfig{
			Config: aklib.TestConfig,
		},
	}
	var buf bytes.Buffer
	var nonce Nonce
	for i := range nonce {
		nonce[i] = byte(i)
	}
	if err := Write(s, &nonce, CmdPing, &buf); err != nil {
		t.Error(err)
	}
	cmd, body, err := ReadHeader(s, &buf)
	if err != nil {
		t.Error(err)
	}
	if cmd != CmdPing {
		t.Error("invalid write/read")
	}
	n, err := ReadNonce(body)
	if err != nil {
		t.Error(err)
	}
	if *n != nonce {
		t.Error("invalid readnonce")
	}
}

func TestMsgErr(t *testing.T) {
	s := &setting.Setting{
		DBConfig: aklib.DBConfig{
			Config: aklib.TestConfig,
		}}
	var buf bytes.Buffer
	var nonce Nonce
	for i := range nonce {
		nonce[i] = byte(i)
	}
	if err := Write(s, &nonce, CmdPing, &buf); err != nil {
		t.Error(err)
	}
	buf.Truncate(1)
	if _, _, err := ReadHeader(s, &buf); err == nil {
		t.Error("should be error")
	}
}
