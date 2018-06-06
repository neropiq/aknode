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

package imesh

import (
	"log"
	"sync"
	"time"

	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/imesh/leaves"
	"github.com/AidosKuneen/aknode/setting"

	"github.com/dgraph-io/badger"
)

type unresolvedTx struct {
	body       *tx.Body
	unresolved bool
	visited    bool
	broken     bool
	minable    bool
}

//Noexist represents a non-existence transaction.
type Noexist struct {
	Hash     tx.Hash
	Sleep    time.Duration
	Searched time.Time
	Minable  bool
}

var unresolved = struct {
	Txs      map[[32]byte]*unresolvedTx
	Noexists map[[32]byte]*Noexist
	sync.RWMutex
}{}

//Init initialize unresolved txs.
func Init(s *setting.Setting) error {
	err := s.DB.View(func(txn *badger.Txn) error {
		return db.Get(txn, nil, &unresolved, db.HeaderUnresolvedInfo)
	})
	if err != nil && err != badger.ErrKeyNotFound {
		return err
	}
	for h, tr := range unresolved.Txs {
		t, err := GetTx(s, h[:])
		if err != nil {
			return nil
		}
		tr.body = t.Body
		if err := t.Check(s.Config); err != nil {
			if _, err2 := t.CheckMinable(s.Config); err2 != nil {
				return err2
			}
			tr.minable = true
		}
	}
	return nil
}

func put(s *setting.Setting) error {
	return s.DB.Update(func(txn *badger.Txn) error {
		return db.Put(txn, nil, &unresolved, db.HeaderUnresolvedInfo)
	})
}

//AddTxHash adds a h as unresolved tx hash.
func AddTxHash(s *setting.Setting, h tx.Hash, isMinable bool) error {
	unresolved.Lock()
	defer unresolved.Unlock()
	has, err := Has(s, h)
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	unresolved.Noexists[h.Array()] = &Noexist{
		Hash:    h,
		Sleep:   time.Minute,
		Minable: isMinable,
	}
	return nil
}

//CheckAddTx adds trs into imeash if they are already resolved.
//If not adds to search cron.
func CheckAddTx(s *setting.Setting, trs []*tx.Transaction) error {
	unresolved.Lock()
	defer unresolved.Unlock()
	for _, tr := range trs {
		ng, err := isBrokenTx(s, tr.Hash())
		if err != nil {
			return err
		}
		if ng {
			continue
		}
		minable := false
		if err := tr.Check(s.Config); err != nil {
			if _, err2 := tr.CheckMinable(s.Config); err2 == nil {
				minable = true
			} else {
				if err1 := putBrokenTx(s, tr.Hash()); err1 != nil {
					return err1
				}
				log.Println(err)
				continue
			}
		}
		has, err := Has(s, tr.Hash())
		if err != nil {
			return err
		}
		if has {
			continue
		}
		if err := putUnresolvedTx(s, tr); err != nil {
			return err
		}
		u := &unresolvedTx{
			body:    tr.Body,
			minable: minable,
		}
		unresolved.Txs[tr.Hash().Array()] = u
	}
	return put(s)
}

//GetSearchingTx returns txs which are need to be searched.
func GetSearchingTx(s *setting.Setting) ([]Noexist, error) {
	unresolved.Lock()
	defer unresolved.Unlock()
	r := make([]Noexist, 0, len(unresolved.Noexists))
	for _, n := range unresolved.Noexists {
		if !n.Searched.IsZero() && n.Searched.Add(n.Sleep).Before(time.Now()) {
			continue
		}
		r = append(r, *n)
		n.Searched = time.Now()
		n.Sleep *= 2
	}
	return r, put(s)
}

//Resolve checks all reference of unresolvev txs
//and add to imesh if all are resolved.
func Resolve(s *setting.Setting) ([]tx.Hash, []tx.Hash, error) {
	unresolved.Lock()
	defer unresolved.Unlock()
	ns, err := isResolved(s)
	if err != nil {
		return nil, nil, err
	}
	for _, n := range ns {
		if _, ok := unresolved.Noexists[n.Hash.Array()]; !ok {
			unresolved.Noexists[n.Hash.Array()] = n
		}
	}
	var txH, minableH []tx.Hash
	for h, tr := range unresolved.Txs {
		tr.visited = false
		if !tr.broken && tr.unresolved {
			tr.unresolved = false
			continue
		}
		tra, err := getUnresolvedTx(s, h[:])
		if err != nil {
			return nil, nil, err
		}
		if err := deleteUnresolvedTx(s, h[:]); err != nil {
			return nil, nil, err
		}
		delete(unresolved.Txs, h)
		if tr.broken {
			if err := putBrokenTx(s, h[:]); err != nil {
				return nil, nil, err
			}
			continue
		}
		switch tr.minable {
		case true:
			minableH = append(minableH, h[:])
			if _, err := tra.CheckAllMinable(getTxFunc(s), s.Config); err != nil {
				log.Println(err)
				continue
			}
			if err := PutMinableTx(s, tra); err != nil {
				return nil, nil, err
			}
			continue
		case false:
			txH = append(txH, h[:])
			if err := tra.CheckAll(getTxFunc(s), s.Config); err != nil {
				log.Println(err)
				continue
			}
			if err := putTx(s, tra); err != nil {
				return nil, nil, err
			}
			if err := leaves.CheckAdd(s, tra); err != nil {
				return nil, nil, err
			}
		}
	}
	return txH, minableH, put(s)
}

func isResolved(s *setting.Setting) ([]*Noexist, error) {
	var ns []*Noexist
	var err error
	for _, tr := range unresolved.Txs {
		ns, err = tr.dfs(s, nil)
		if err != nil {
			return nil, err
		}
	}
	return ns, nil
}
func (tr *unresolvedTx) dfs(s *setting.Setting, ns []*Noexist) ([]*Noexist, error) {
	if tr.visited {
		return ns, nil
	}
	tr.visited = true
	prevs := make([]tx.Hash, 0, len(tr.body.Previous)+1+
		len(tr.body.Inputs)+len(tr.body.MultiSigIns))
	for _, prev := range tr.body.Previous {
		prevs = append(prevs, prev)
	}
	prevs = append(prevs, tr.body.TicketInput)
	for _, prev := range tr.body.Inputs {
		prevs = append(prevs, prev.PreviousTX)
	}
	for _, prev := range tr.body.MultiSigIns {
		prevs = append(prevs, prev.PreviousTX)
	}

	for _, prev := range prevs {
		has, err := Has(s, prev)
		if err != nil {
			return nil, err
		}
		if has {
			continue
		}
		ng, err := isBrokenTx(s, prev)
		if err != nil {
			return nil, err
		}
		if ng {
			tr.broken = true
			return ns, nil
		}
		if ptr, ok := unresolved.Txs[prev.Array()]; !ok || ptr.minable {
			tr.unresolved = true
			if _, ok1 := unresolved.Noexists[prev.Array()]; !ok1 {
				ns = append(ns, &Noexist{
					Hash:  prev,
					Sleep: time.Minute,
				})
			}
		}
	}
	return ns, nil
}
