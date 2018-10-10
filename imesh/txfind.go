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
	"errors"
	"log"
	"time"

	"github.com/AidosKuneen/aklib"

	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/imesh/leaves"
	"github.com/AidosKuneen/aknode/setting"

	"github.com/dgraph-io/badger"
)

type unresolvedTx struct {
	prevs      []tx.Hash
	Type       tx.Type
	unresolved bool
	visited    bool
	broken     bool
}

//Noexist represents a non-existence transaction.
type Noexist struct {
	*tx.HashWithType
	Count    byte
	Searched time.Time
}

var unresolved = struct {
	Txs      map[[32]byte]*unresolvedTx
	Noexists map[[32]byte]*Noexist
}{
	Txs:      make(map[[32]byte]*unresolvedTx),
	Noexists: make(map[[32]byte]*Noexist),
}

//Init initialize imesh db and unresolved txs.
func Init(s *setting.Setting) error {
	txno.TxNo = 0
	unresolved.Txs = make(map[[32]byte]*unresolvedTx)
	unresolved.Noexists = make(map[[32]byte]*Noexist)

	var total uint64
	tr := tx.New(s.Config)
	for adr, val := range s.Config.Genesis {
		if err := tr.AddOutput(s.Config, adr, val); err != nil {
			return err
		}
		total += val
	}
	if total != aklib.ADKSupply {
		return errors.New("invalid total supply")
	}
	has, err2 := Has(s, tr.Hash())
	if err2 != nil {
		return err2
	}
	if !has {
		if err := putTxSub(s, tr); err != nil {
			return err
		}
		t, err := GetTxInfo(s.DB, tr.Hash())
		if err != nil {
			return err
		}
		t.StatNo = statusGenesis
		if err := t.put(s.DB); err != nil {
			return err
		}
		if err := leaves.CheckAdd(s, tr); err != nil {
			return err
		}
	}
	err2 = s.DB.View(func(txn *badger.Txn) error {
		return db.Get(txn, nil, &unresolved, db.HeaderUnresolvedInfo)
	})
	if err2 != nil && err2 != badger.ErrKeyNotFound {
		return err2
	}
	for h, ut := range unresolved.Txs {
		t, err := getUnresolvedTx(s, h[:])
		if err != nil {
			return nil
		}
		tr := &unresolvedTx{
			prevs: prevs(t),
			Type:  ut.Type,
		}
		if err := t.Check(s.Config, tr.Type); err != nil {
			return err
		}
		unresolved.Txs[h] = tr
	}
	return getTxNo(s)
}

//locked by mutex (unresolved)
func put(s *setting.Setting) error {
	return s.DB.Update(func(txn *badger.Txn) error {
		return db.Put(txn, nil, &unresolved, db.HeaderUnresolvedInfo)
	})
}

//AddNoexistTxHash adds a h as unresolved tx hash.
func AddNoexistTxHash(s *setting.Setting, h tx.Hash, typ tx.Type) error {
	mutex.Lock()
	defer mutex.Unlock()
	has, err := Has(s, h)
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	if _, exist := unresolved.Noexists[h.Array()]; exist {
		return nil
	}
	unresolved.Noexists[h.Array()] = &Noexist{
		HashWithType: &tx.HashWithType{
			Hash: h,
			Type: typ,
		},
	}
	return nil
}

//CheckAddTx adds trs into imeash if they are already resolved.
//If not adds to search cron.
func CheckAddTx(s *setting.Setting, tr *tx.Transaction, typ tx.Type) error {
	switch typ {
	case tx.TypeNormal, tx.TypeRewardFee, tx.TypeRewardTicket:
	default:
		return errors.New("unknows type")
	}
	mutex.Lock()
	defer mutex.Unlock()
	has, err := Has(s, tr.Hash())
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	ng, err := isBrokenTx(s, tr.Hash())
	if err != nil {
		return err
	}
	if ng {
		return errors.New("tx is broken")
	}
	if err := tr.Check(s.Config, typ); err != nil {
		log.Println(err)
		if err1 := putBrokenTx(s, tr.Hash()); err1 != nil {
			return err1
		}
		return err
	}
	if err := putUnresolvedTx(s, tr); err != nil {
		return err
	}
	u := &unresolvedTx{
		prevs: prevs(tr),
		Type:  typ,
	}
	unresolved.Txs[tr.Hash().Array()] = u
	return put(s)
}

//GetSearchingTx returns txs which are need to be searched.
func GetSearchingTx(s *setting.Setting) ([]Noexist, error) {
	mutex.Lock()
	defer mutex.Unlock()
	r := make([]Noexist, 0, len(unresolved.Noexists))
	for h, n := range unresolved.Noexists {
		sleep := (1 << (n.Count - 1)) * time.Minute
		if !n.Searched.IsZero() && !n.Searched.Add(sleep).Before(time.Now()) {
			continue
		}
		r = append(r, *n)
		n.Searched = time.Now()
		if n.Count++; n.Count > 10 {
			if err := putBrokenTx(s, n.Hash); err != nil {
				return nil, err
			}
			delete(unresolved.Noexists, h)
		}
	}
	return r, put(s)
}

//Resolve checks all reference of unresolvev txs
//and add to imesh if all are resolved.
func Resolve(s *setting.Setting) ([]*tx.HashWithType, error) {
	mutex.Lock()
	defer mutex.Unlock()
	if err := isResolved(s); err != nil {
		return nil, err
	}
	var trs []*tx.HashWithType
	for hs, tr := range unresolved.Txs {
		if !tr.broken && tr.unresolved {
			tr.visited = false
			tr.unresolved = false
			continue
		}
		delete(unresolved.Txs, hs)
		if tr.broken {
			continue
		}
		h := make(tx.Hash, 32)
		copy(h, hs[:])
		trs = append(trs, &tx.HashWithType{
			Hash: h,
			Type: tr.Type,
		})
	}
	return trs, put(s)
}

func isResolved(s *setting.Setting) error {
	for h, tr := range unresolved.Txs {
		if err := tr.dfs(s, h); err != nil {
			return err
		}
	}
	return nil
}
func (tr *unresolvedTx) dfs(s *setting.Setting, h [32]byte) error {
	if tr.visited {
		return nil
	}
	tr.visited = true
	for _, prev := range tr.prevs {
		has, err := Has(s, prev)
		if err != nil {
			return err
		}
		if has {
			continue
		}
		ng, err := isBrokenTx(s, prev)
		if err != nil {
			return err
		}
		if ng {
			tr.broken = true
			return nil
		}
		if ptr, ok := unresolved.Txs[prev.Array()]; !ok || ptr.Type != tx.TypeNormal {
			tr.unresolved = true
			if _, ok1 := unresolved.Noexists[prev.Array()]; !ok1 {
				unresolved.Noexists[prev.Array()] = &Noexist{
					HashWithType: &tx.HashWithType{
						Hash: prev,
						Type: tx.TypeNormal,
					},
				}
			}
		} else {
			if err := ptr.dfs(s, prev.Array()); err != nil {
				return err
			}
			if ptr.broken {
				tr.broken = true
			}
			if ptr.unresolved {
				tr.unresolved = true
			}
		}
	}
	if tr.broken || !tr.unresolved {
		if err := resolved(s, tr, h[:]); err != nil {
			return err
		}
	}
	return nil
}

func resolved(s *setting.Setting, tr *unresolvedTx, hs tx.Hash) error {
	tra, err := getUnresolvedTx(s, hs)
	if err != nil {
		return err
	}
	if err := deleteUnresolvedTx(s, hs); err != nil {
		return err
	}
	if tr.broken {
		return putBrokenTx(s, hs)
	}
	if err := IsValid(s, tra, tr.Type); err != nil {
		tr.broken = true
		log.Println(err)
		return putBrokenTx(s, hs)
	}
	if tr.Type == tx.TypeNormal {
		//We must add to imesh after adding to leave.
		//If not and if an user stops aknode before adding to leave afeter adding to imesh,
		//leaves will be broekn.
		if err := leaves.CheckAdd(s, tra); err != nil {
			return err
		}
		return putTx(s, tra)
	}
	return putMinableTx(s, tra, tr.Type)
}

func prevs(tr *tx.Transaction) []tx.Hash {
	prevs := make([]tx.Hash, 0, len(tr.Parent)+1+
		len(tr.Inputs)+len(tr.MultiSigIns))
	prevs = append(prevs, tr.Parent...)
	if tr.TicketInput != nil {
		prevs = append(prevs, tr.TicketInput)
	}
	for _, prev := range tr.Inputs {
		prevs = append(prevs, prev.PreviousTX)
	}
	for _, prev := range tr.MultiSigIns {
		prevs = append(prevs, prev.PreviousTX)
	}
	return prevs
}
