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
	"log"
	"sync"
	"time"

	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/node"

	"github.com/AidosKuneen/aklib/address"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/imesh/leaves"
	"github.com/AidosKuneen/aknode/setting"
)

type trWallet struct {
	conf *setting.Setting
}

//NewChangeAddress returns a new address for change.
func (w *trWallet) NewChangeAddress() (*address.Address, error) {
	adrstr, err := newChangeAddress(w.conf)
	if err != nil {
		return nil, err
	}
	adr, err := getAddress(w.conf, adrstr)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return adr.address, nil
}

//GetUTXO returns UTXOs whose total is over outtotal.
func (w *trWallet) GetUTXO(outtotal uint64) ([]*tx.UTXO, error) {
	var utxos []*tx.UTXO
	var total uint64
	utxos, total, err := getUTXO102(w.conf, false)
	if err != nil {
		return nil, err
	}
	if outtotal > total {
		u, bal, err := getUTXO102(w.conf, true)
		if err != nil {
			return nil, err
		}
		total += bal
		utxos = append(utxos, u...)
	}
	return utxos, nil
}

//GetLeaves return leaves hashes.
func (w *trWallet) GetLeaves() ([]tx.Hash, error) {
	return leaves.Get(tx.DefaultPreviousSize), nil
}

var powmutex sync.Mutex

//Send sends token.
func Send(conf *setting.Setting, tag []byte, outputs ...*tx.RawOutput) (string, error) {
	w := &trWallet{
		conf: conf,
	}
	tr, err := tx.Build(conf.Config, w, tag, outputs)
	if err != nil {
		return "", err
	}
	log.Println("starting PoW...")
	powmutex.Lock()
	err = tr.PoW()
	powmutex.Unlock()
	if err != nil {
		return "", err
	}
	if err := imesh.IsValid(conf, tr, tx.TypeNormal); err != nil {
		return "", err
	}
	if err := imesh.CheckAddTx(conf, tr, tx.TypeNormal); err != nil {
		return "", err
	}
	node.Resolve()
	time.Sleep(6 * time.Second)
	log.Println("finished PoW. hash=", tr.Hash())
	return tr.Hash().String(), nil
}
