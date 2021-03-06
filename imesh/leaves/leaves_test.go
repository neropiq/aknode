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

package leaves

import (
	"bytes"
	"encoding/hex"
	"os"
	"testing"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/setting"
)

var s setting.Setting

func setup(t *testing.T) {
	var err2 error
	if err := os.RemoveAll("./test_db"); err != nil {
		t.Error(err)
	}
	s.DB, err2 = db.Open("./test_db")
	if err2 != nil {
		t.Error(err2)
	}
	s.Config = aklib.TestConfig
}
func teardown(t *testing.T) {
	if err := os.RemoveAll("./test_db"); err != nil {
		t.Error(err)
	}
}

func TestLeaves(t *testing.T) {
	setup(t)
	defer teardown(t)
	var trs [10]*tx.Transaction
	for i := range trs {
		trs[i] = &tx.Transaction{
			Body: &tx.Body{
				Message: []byte{byte(i)}, //to change hash for each txs
				Parent:  make([]tx.Hash, 2),
			},
		}
	}
	trs[0].Parent[0] = make(tx.Hash, 32)
	trs[0].Parent[1] = make(tx.Hash, 32)
	trs[1].Parent[0] = make(tx.Hash, 32)
	trs[1].Parent[1] = make(tx.Hash, 32)
	trs[2].Parent[0] = trs[0].Hash()
	trs[2].Parent[1] = trs[0].Hash()
	trs[3].Parent[0] = trs[2].Hash()
	trs[3].Parent[1] = make(tx.Hash, 32)
	trs[4].Parent[0] = trs[2].Hash()
	trs[4].Parent[1] = trs[2].Hash()
	trs[5].Parent[0] = trs[0].Hash()
	trs[5].Parent[1] = trs[2].Hash()

	for _, tr := range trs {
		t.Log(hex.EncodeToString(tr.Hash()))
	}

	if err := CheckAdd(&s, nil, trs[:6]...); err != nil {
		t.Error(err)
	}
	for _, tr := range leaves.leaves {
		t.Log(hex.EncodeToString(tr.Hash))
	}

	t.Log(len(leaves.leaves))
	for _, i := range []int{0, 2} {
		for _, tr := range leaves.leaves {
			if bytes.Equal(trs[i].Hash(), tr.Hash) {
				t.Error("should not be a leaf")
			}
		}
	}
	for _, i := range []int{1, 3, 4, 5} {
		ok := false
		for _, tr := range leaves.leaves {
			if bytes.Equal(trs[i].Hash(), tr.Hash) {
				ok = true
			}
		}
		if !ok {
			t.Error("should be a leaf", i)
		}
	}

	leaves.leaves = leaves.leaves[:0]
	if err := Init(&s); err != nil {
		t.Error(err)
	}
	if len(leaves.leaves) != 4 {
		t.Error("invalid init")
	}

	rs := Get(6)
	if len(rs) != 4 {
		t.Error("invalid init")
	}

	trs[6].Parent[0] = trs[3].Hash()
	trs[6].Parent[1] = trs[2].Hash()
	if err := CheckAdd(&s, nil, trs[6]); err != nil {
		t.Error(err)
	}
	for _, i := range []int{0, 2, 3} {
		for _, tr := range leaves.leaves {
			if bytes.Equal(trs[i].Hash(), tr.Hash) {
				t.Error("should not be a leaf")
			}
		}
	}
	for _, i := range []int{1, 4, 5} {
		ok := false
		for _, tr := range leaves.leaves {
			if bytes.Equal(trs[i].Hash(), tr.Hash) {
				ok = true
			}
		}
		if !ok {
			t.Error("should be a leaf", i)
		}
	}

	if Size() != 4 {
		t.Error("invalid size", Size())
	}
	if err := SetConfirmed(&s, trs[1].Hash()); err != nil {
		t.Error(err)
	}
	rs = Get(6)
	if !bytes.Equal(rs[len(rs)-1], trs[1].Hash()) {
		t.Error("invalid get")
	}
	rs = GetAllUnconfirmed()
	if len(rs) != 3 {
		t.Error("invalid getall")
	}
	for _, i := range []int{4, 5, 6} {
		ok := false
		for _, tr := range GetAll() {
			if bytes.Equal(trs[i].Hash(), tr) {
				ok = true
			}
		}
		if !ok {
			t.Error("should be a leaf", i)
		}
	}

}
