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

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/dgraph-io/badger"
)

func updateMulsigAddress(cfg *aklib.Config, txn *badger.Txn, tr *tx.Transaction) error {
	for i, out := range tr.MultiSigOuts {
		madr := out.AddressByte(cfg)
		var tmp tx.InoutHash
		if err := db.Get(txn, madr, &tmp, db.HeaderMultisigAddress); err == nil {
			continue
		}
		ih := &tx.InoutHash{
			Hash:  tr.Hash(),
			Type:  tx.TypeMulout,
			Index: byte(i),
		}
		//don't care if other routines wrote the address.
		if err := db.Put(txn, madr, ih, db.HeaderMultisigAddress); err != nil && err != badger.ErrConflict {
			return err
		}
	}
	return nil
}

//GetMultisig returns a Multisig structure whose address is madr.
func GetMultisig(bdb *badger.DB, madr []byte) (*tx.MultisigStruct, error) {
	var msig *tx.MultiSigOut
	err2 := bdb.View(func(txn *badger.Txn) error {
		var ih tx.InoutHash
		if err := db.Get(txn, madr, &ih, db.HeaderMultisigAddress); err != nil {
			return err
		}
		var ti TxInfo
		if err := db.Get(txn, ih.Hash, &ti, db.HeaderTxInfo); err != nil {
			return err
		}
		if len(ti.Body.MultiSigOuts) < int(ih.Index) {
			return errors.New("invalid multisig index")
		}
		msig = ti.Body.MultiSigOuts[ih.Index]
		return nil
	})
	if err2 != nil {
		return nil, err2
	}
	return &msig.MultisigStruct, nil
}
