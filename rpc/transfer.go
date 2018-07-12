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
	"errors"
	"log"
	"sort"
	"sync"

	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/node"

	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/imesh/leaves"
	"github.com/AidosKuneen/aknode/setting"
)

func buildTx(conf *setting.Setting, ac string, outputs ...output) (*tx.Transaction, error) {
	tr := tx.New(conf.Config, leaves.Get(tx.DefaultPreviousSize)...)
	var outtotal uint64
	for _, o := range outputs {
		if err := tr.AddOutput(conf.Config, o.address, o.value); err != nil {
			return nil, err
		}
		outtotal += o.value
	}

	var utxos []*utxo
	var total uint64
	var account *account
	if ac == "*" {
		for acname, acc := range wallet.Accounts {
			account = acc
			u, bal, err := getUTXO(conf, acname, true)
			if err != nil {
				return nil, err
			}
			total += bal
			utxos = append(utxos, u...)
		}
	} else {
		account = wallet.Accounts[ac]
		u, bal, err := getUTXO(conf, ac, true)
		if err != nil {
			return nil, err
		}
		total = bal
		utxos = u
	}
	if outtotal > total {
		return nil, errors.New("insufficient balance")
	}
	sort.Slice(utxos, func(i, j int) bool {
		return utxos[i].value < utxos[i].value
	})
	i := sort.Search(len(utxos), func(i int) bool {
		return utxos[i].value >= outtotal
	})
	change := outtotal
	for ; i >= 0 && change > 0; i-- {
		tr.AddInput(utxos[i].Hash, utxos[i].Index)
		a, err := utxos[i].address.getAddress()
		if err != nil {
			return nil, err
		}
		if err := tr.Sign(a); err != nil {
			return nil, err
		}
		change -= utxos[i].value
	}
	if change > 0 {
		return nil, errors.New("insufficient balance")
	}
	if change == 0 {
		return tr, nil
	}
	adr, err := newAddress2(conf, account)
	if err != nil {
		return nil, err
	}
	return tr, tr.AddOutput(conf.Config, adr.Address58(), -change)
}

var powMutex sync.Mutex

type output struct {
	address string
	value   uint64
}

//Send sends token.
func Send(conf *setting.Setting, ac string, tag []byte, outputs ...output) (tx.Hash, error) {
	powMutex.Lock()
	defer powMutex.Unlock()
	tr, err := buildTx(conf, ac, outputs...)
	if err != nil {
		return nil, err
	}
	tr.Message = tag
	log.Println("starting PoW...")
	if err := tr.PoW(); err != nil {
		return nil, err
	}
	if err := imesh.IsValid(conf, tr, tx.TxNormal); err != nil {
		return nil, err
	}
	if err := imesh.CheckAddTx(conf, tr, tx.TxNormal); err != nil {
		return nil, err
	}
	node.Resolve()
	log.Println("finished PoW. hash=", tr.Hash())
	return tr.Hash(), nil
}
