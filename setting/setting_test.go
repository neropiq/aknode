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

package setting

import (
	"strings"
	"testing"
)

func TestSetting(t *testing.T) {
	s, err := Load([]byte(`{}`))
	if err != nil {
		t.Error(err)
	}

	if !strings.Contains(s.BaseDir(), "mainnet") {
		t.Error("invalid basedir")
	}

	s, err = Load([]byte(`{
		"testnet":1,
		"blacklists":["www.google.com"],
		"default_nodes":["www.google.com:80"]
	}`))
	if err != nil {
		t.Error(err)
	}

	if !strings.Contains(s.BaseDir(), "testnet") {
		t.Error("invalid basedir")
	}
	if err := s.DB.Close(); err != nil {
		t.Error(err)
	}

	s, err = Load([]byte(`{
		"testnet":1,
		"blacklists":["123.24.11.12"],
		"default_nodes":["1.2.3.4:80"]
	}`))
	if err != nil {
		t.Error(err)
	}
	if !s.InBlacklist("123.24.11.12:1234") {
		t.Error("should be in blacklist")
	}
	if s.InBlacklist("123.24.11.123:1234") {
		t.Error("should not be in blacklist")
	}
	if !s.InBlacklist("123.24.11.12") {
		t.Error("should be in blacklist")
	}
	if s.InBlacklist("123.24.11.123") {
		t.Error("should not be in blacklist")
	}

	s, err = Load([]byte(`{
		"testnet":1,
		"blacklists":["1323.24.11.12"],
	}`))
	if err == nil {
		t.Error("should be error")
	}
}
