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

//TODO: store unused outputs for each addresses after txs are confirmed

import (
	"bytes"
	"sort"

	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/setting"

	"github.com/dgraph-io/badger"
)

func updateAddressToTx(s *setting.Setting, txn *badger.Txn, adr []byte, addH, delH tx.Hash) error {
	var hashes []tx.Hash
	if err := db.Get(txn, adr, &hashes, db.HeaderAddressToTx); err != nil && err != badger.ErrKeyNotFound {
		return err
	}
	hashes = append(hashes, addH)
	sort.Slice(hashes, func(i, j int) bool {
		return bytes.Compare(hashes[i], hashes[j]) < 0
	})
	if delH != nil {
		i := sort.Search(len(hashes), func(i int) bool {
			return bytes.Compare(hashes[i], delH) >= 0
		})
		if i < len(hashes) && bytes.Equal(hashes[i], delH) {
			hashes = append(hashes[:i], hashes[i+1:]...)
		}
	}
	return db.Put(txn, adr, hashes, db.HeaderAddressToTx)
}

func putInputAddressToTx(s *setting.Setting, txn *badger.Txn, tr *tx.Transaction) error {
	if tr.TicketInput != nil {
		var ti TxInfo
		if err := db.Get(txn, tr.TicketInput, &ti, db.HeaderTxInfo); err != nil {
			return err
		}
		if err := updateAddressToTx(s, txn, ti.Body.TicketOutput, tr.Hash(), tr.TicketInput); err != nil {
			return err
		}
	}
	for _, inp := range tr.Inputs {
		var ti TxInfo
		if err := db.Get(txn, inp.PreviousTX, &ti, db.HeaderTxInfo); err != nil {
			return err
		}
		adr := ti.Body.Outputs[inp.Index].Address
		if err := updateAddressToTx(s, txn, adr, tr.Hash(), inp.PreviousTX); err != nil {
			return err
		}
	}
	return nil
}

func putMultisigInAddressToTx(s *setting.Setting, txn *badger.Txn, tr *tx.Transaction) error {
	for _, inp := range tr.MultiSigIns {
		var ti TxInfo
		if err := db.Get(txn, inp.PreviousTX, &ti, db.HeaderTxInfo); err != nil {
			return err
		}
		for _, adr := range ti.Body.MultiSigOuts[inp.Index].Addresses {
			if err := updateAddressToTx(s, txn, adr, tr.Hash(), inp.PreviousTX); err != nil {
				return err
			}
		}
	}
	return nil
}

func putOutputAddressToTx(s *setting.Setting, txn *badger.Txn, tr *tx.Transaction) error {
	if tr.TicketOutput != nil {
		if err := updateAddressToTx(s, txn, tr.TicketOutput, tr.Hash(), nil); err != nil {
			return err
		}
	}
	for _, inp := range tr.Outputs {
		if err := updateAddressToTx(s, txn, inp.Address, tr.Hash(), nil); err != nil {
			return err
		}
	}
	return nil
}

func putMultisigOutAddressToTx(s *setting.Setting, txn *badger.Txn, tr *tx.Transaction) error {
	for _, out := range tr.MultiSigOuts {
		for _, adr := range out.Addresses {
			if err := updateAddressToTx(s, txn, adr, tr.Hash(), nil); err != nil {
				return err
			}
		}
	}
	return nil
}

func putAddressToTx(s *setting.Setting, txn *badger.Txn, tr *tx.Transaction) error {
	return s.DB.Update(func(txn *badger.Txn) error {
		if err := putInputAddressToTx(s, txn, tr); err != nil {
			return err
		}
		if err := putMultisigInAddressToTx(s, txn, tr); err != nil {
			return err
		}
		if err := putOutputAddressToTx(s, txn, tr); err != nil {
			return err
		}
		return putMultisigOutAddressToTx(s, txn, tr)
	})
}

//GetTxsFromAddress returns txs  associated with  address adr.
//Spend txs are not included, so you should check input hashes in tx.
func GetTxsFromAddress(s *setting.Setting, adr []byte) ([]tx.Hash, error) {
	var hashes []tx.Hash
	err := s.DB.View(func(txn *badger.Txn) error {
		err := db.Get(txn, adr, &hashes, db.HeaderAddressToTx)
		if err == badger.ErrKeyNotFound {
			return nil
		}
		return err
	})
	return hashes, err
}
