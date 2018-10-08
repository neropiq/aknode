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

type leaf struct {
	Hash      tx.Hash
	Confirmed bool
}

//leaves represents leaves in iMesh.
var leaves = struct {
	leaves []*leaf
	sync.RWMutex
}{}

//Init loads leaves from DB.
func Init(s *setting.Setting) error {
	leaves.leaves = nil
	err := s.DB.View(func(txn *badger.Txn) error {
		return db.Get(txn, nil, &leaves.leaves, db.HeaderLeaves)
	})
	if err != nil && err != badger.ErrKeyNotFound {
		return err
	}
	return nil
}

//Size return # of leave
func Size() int {
	leaves.RLock()
	defer leaves.RUnlock()
	return len(leaves.leaves)
}

//SetConfirmed set leaves whose hash is h to be confirmed.
func SetConfirmed(s *setting.Setting, h tx.Hash) error {
	leaves.Lock()
	defer leaves.Unlock()
	for _, l := range leaves.leaves {
		if bytes.Equal(l.Hash, h) {
			l.Confirmed = true
		}
	}
	return put(s)
}

func gethash() ([]tx.Hash, []tx.Hash) {
	ncs := make([]tx.Hash, 0, len(leaves.leaves))
	cs := make([]tx.Hash, 0, len(leaves.leaves))
	for _, l := range leaves.leaves {
		if !l.Confirmed {
			ncs = append(ncs, l.Hash)
		} else {
			cs = append(cs, l.Hash)
		}
	}
	return ncs, cs
}

//Get gets n random leaves. if <=0, it returns all leaves.
//Unconfirmed txs are prior to confirmed ones.
func Get(n int) []tx.Hash {
	leaves.RLock()
	defer leaves.RUnlock()
	ncs, cs := gethash()

	for i := len(ncs) - 1; i >= 0; i-- {
		j := rand.R.Intn(i + 1)
		ncs[i], ncs[j] = ncs[j], ncs[i]
	}
	for i := len(cs) - 1; i >= 0; i-- {
		j := rand.R.Intn(i + 1)
		cs[i], cs[j] = cs[j], cs[i]
	}
	if len(ncs) >= n {
		return ncs[:n]
	}
	if len(ncs)+len(cs) < n {
		return append(ncs, cs...)
	}
	return append(ncs, cs[:n-len(ncs)]...)
}

//GetAllUnconfirmed gets all unconfirmed leaves after sorting.
func GetAllUnconfirmed() []tx.Hash {
	leaves.RLock()
	defer leaves.RUnlock()
	ncs, _ := gethash()
	sort.Slice(ncs, func(i, j int) bool {
		return bytes.Compare(ncs[i], ncs[j]) < 0
	})
	return ncs
}

//GetAll gets all leaves after sorting.
func GetAll() []tx.Hash {
	leaves.RLock()
	defer leaves.RUnlock()
	ncs, cs := gethash()
	ncs = append(ncs, cs...)
	sort.Slice(ncs, func(i, j int) bool {
		return bytes.Compare(ncs[i], ncs[j]) < 0
	})
	return ncs
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
	leaves.leaves = leaves.leaves[:0]
	//h is reused (i.e. always same object) in for loop, so need to clone it.
	for h, tr := range txs {
		if !tr.visited {
			hh := make(tx.Hash, 32)
			copy(hh, h[:])
			leaves.leaves = append(leaves.leaves, &leaf{
				Hash: hh,
			})
		}
	}
	return put(s)
}

func put(s *setting.Setting) error {
	return s.DB.Update(func(txn *badger.Txn) error {
		return db.Put(txn, nil, leaves.leaves, db.HeaderLeaves)
	})
}

func isVisited(trs []*tx.Transaction) map[[32]byte]*txsearch {
	txs := make(map[[32]byte]*txsearch)
	for _, tr := range trs {
		txs[tr.Hash().Array()] = &txsearch{
			Transaction: tr,
		}
	}
	for _, l := range leaves.leaves {
		txs[l.Hash.Array()] = &txsearch{}
	}
	for _, tr := range trs {
		for _, prev := range tr.Parent {
			if t, ok := txs[prev.Array()]; ok {
				t.visited = true
			}
		}
		for _, prev := range tr.Inputs {
			if t, ok := txs[prev.PreviousTX.Array()]; ok {
				t.visited = true
			}
		}
		for _, prev := range tr.MultiSigIns {
			if t, ok := txs[prev.PreviousTX.Array()]; ok {
				t.visited = true
			}
		}
		if t, ok := txs[tr.TicketInput.Array()]; ok {
			t.visited = true
		}
	}
	return txs
}
