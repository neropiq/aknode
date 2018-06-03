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
	"fmt"
	"log"
	"sort"
	"sync"

	"github.com/AidosKuneen/aklib/tx"

	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aklib/rand"
	"github.com/AidosKuneen/aknode/setting"
	"github.com/dgraph-io/badger"
)

//leaves represents leaves in iMesh.
var leaves = struct {
	hash []tx.Hash
	sync.RWMutex
}{}

//Init loads leaves from DB.
func Init(s *setting.Setting) {
	err := s.DB.View(func(txn *badger.Txn) error {
		return db.Get(txn, nil, &leaves.hash, db.HeaderLeaves)
	})
	if err != nil {
		fmt.Println(err)
		log.Fatal(err)
	}
}

//Get gets n random leaves. if <=0, it returns all leaves.
func Get(n int) ([]tx.Hash, error) {
	leaves.RLock()
	defer leaves.Unlock()
	r := make([]tx.Hash, 0, len(leaves.hash))
	copy(r, leaves.hash)

	for i := n - 1; i >= 0; i-- {
		j := rand.R.Intn(i + 1)
		r[i], r[j] = r[j], r[i]
	}
	if n < len(leaves.hash) || n <= 0 {
		return r, nil
	}
	return r[:n], nil
}

type txsearch struct {
	*tx.Transaction
	hash    []byte
	visited bool
}

//CheckAdd checks trs and leaves if these are leaves and add them.
func CheckAdd(s *setting.Setting, trs ...*tx.Transaction) error {
	leaves.Lock()
	defer leaves.Unlock()
	txs := isVisited(trs, leaves.hash)
	leaves.hash = leaves.hash[:0]
	for _, tr := range txs {
		if !tr.visited {
			leaves.hash = append(leaves.hash, tr.hash)
		}
	}
	return put(s)
}

func put(s *setting.Setting) error {
	return s.DB.Update(func(txn *badger.Txn) error {
		return db.Put(txn, nil, leaves.hash, db.HeaderLeaves)
	})
}

func isVisited(trs []*tx.Transaction, leaves []tx.Hash) []*txsearch {
	txs := make([]*txsearch, len(trs)+len(leaves))
	for i, tr := range trs {
		txs[i].Transaction = tr
		txs[i].hash = tr.Hash()
	}
	for i, l := range leaves {
		txs[len(trs)+i].hash = l
	}
	sort.Slice(txs, func(i, j int) bool {
		return bytes.Compare(txs[i].hash, txs[j].hash) < 0
	})
	for _, tr := range trs {
		for _, prev := range tr.Previous {
			i := sort.Search(len(txs), func(i int) bool {
				return bytes.Compare(txs[i].hash, prev) >= 0
			})
			if i < len(txs) && bytes.Equal(txs[i].hash, prev) {
				txs[i].visited = true
			}
		}
	}
	return txs
}
