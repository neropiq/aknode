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

package rpc

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"testing"
)

func TestEnc(t *testing.T) {
	pt := make([]byte, 32)
	if _, err := rand.Read(pt); err != nil {
		t.Error(err)
	}
	pwd := make([]byte, 32)
	if _, err := rand.Read(pwd); err != nil {
		t.Error(err)
	}
	enc := encrypt(pt, pwd)
	t.Log(hex.EncodeToString(enc))
	r, err := decrypt(enc, pwd)
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(r, pt) {
		t.Error("invalid enc and dec")
	}
	if _, err := decrypt(enc, []byte{0}); err == nil {
		t.Error("invalid enc and dec")
	}
}
