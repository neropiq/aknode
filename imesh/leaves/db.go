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
	"sort"
	"sync"

	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aklib/rand"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/setting"

	"github.com/dgraph-io/badger"
)

//leaves represents leaves in iMesh.
var leaves = struct {
	hash []tx.Hash
	sync.RWMutex
}{}

//Init loads leaves from DB.
func Init(s *setting.Setting) error {
	leaves.hash = nil
	err := s.DB.View(func(txn *badger.Txn) error {
		return db.Get(txn, nil, &leaves.hash, db.HeaderLeaves)
	})
	if err != nil && err != badger.ErrKeyNotFound {
		return err
	}
	return nil
}

//Get gets n random leaves. if <=0, it returns all leaves.
func Get(n int) []tx.Hash {
	leaves.RLock()
	defer leaves.RUnlock()
	r := make([]tx.Hash, len(leaves.hash))
	copy(r, leaves.hash)

	for i := len(r) - 1; i >= 0; i-- {
		j := rand.R.Intn(i + 1)
		r[i], r[j] = r[j], r[i]
	}
	if n >= len(leaves.hash) || n <= 0 {
		return r
	}
	return r[:n]
}

//GetAll gets all leaves after sorting.
func GetAll() []tx.Hash {
	leaves.RLock()
	defer leaves.RUnlock()
	r := make([]tx.Hash, len(leaves.hash))
	copy(r, leaves.hash)
	sort.Slice(r, func(i, j int) bool {
		return bytes.Compare(r[i], r[j]) < 0
	})
	return r
}

type txsearch struct {
	*tx.Transaction
	visited bool
}

//CheckAdd checks trs and leaves if these are leaves and add them.
func CheckAdd(s *setting.Setting, trs ...*tx.Transaction) error {
	leaves.Lock()
	defer leaves.Unlock()
	txs := isVisited(trs)
	leaves.hash = leaves.hash[:0]
	//h is reused (i.e. always same object) in for loop, so need to clone it.
	for h, tr := range txs {
		if !tr.visited {
			hh := make(tx.Hash, 32)
			copy(hh, h[:])
			leaves.hash = append(leaves.hash, hh)
		}
	}
	return put(s)
}

func put(s *setting.Setting) error {
	return s.DB.Update(func(txn *badger.Txn) error {
		return db.Put(txn, nil, leaves.hash, db.HeaderLeaves)
	})
}

func isVisited(trs []*tx.Transaction) map[[32]byte]*txsearch {
	txs := make(map[[32]byte]*txsearch)
	for _, tr := range trs {
		txs[tr.Hash().Array()] = &txsearch{
			Transaction: tr,
		}
	}
	for _, l := range leaves.hash {
		txs[l.Array()] = &txsearch{}
	}
	for _, tr := range trs {
		for _, prev := range tr.Previous {
			if t, ok := txs[prev.Array()]; ok {
				t.visited = true
			}
		}
	}
	return txs
}
