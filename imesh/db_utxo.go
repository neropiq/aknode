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
	"bytes"
	"encoding/hex"
	"errors"
	"log"
	"sort"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/address"
	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/setting"

	"github.com/dgraph-io/badger"
)

func updateAddressToTx(txn *badger.Txn, adr []byte, addH, delH []byte) error {
	var hashes [][]byte
	if err := db.Get(txn, adr, &hashes, db.HeaderAddressToTx); err != nil && err != badger.ErrKeyNotFound {
		return err
	}
	i := sort.Search(len(hashes), func(i int) bool {
		return bytes.Compare(hashes[i], addH) >= 0
	})
	if i >= len(hashes) || !bytes.Equal(hashes[i], addH) {
		hashes = append(hashes[:i], append([][]byte{addH}, hashes[i:]...)...)
	}
	if delH != nil {
		i := sort.Search(len(hashes), func(i int) bool {
			return bytes.Compare(hashes[i], delH) >= 0
		})
		if i < len(hashes) && bytes.Equal(hashes[i], delH) {
			hashes = append(hashes[:i], hashes[i+1:]...)
		} else {
			for i := range hashes {
				log.Println(hex.EncodeToString(hashes[i]), hex.EncodeToString(adr))
			}
			log.Println("not found", hex.EncodeToString(delH), "maybe the address ins not in the wallet or double spend")
		}
	}
	return db.Put(txn, adr, hashes, db.HeaderAddressToTx)
}

func putInputAddressToTx(txn *badger.Txn, tr *tx.Transaction) error {
	if tr.TicketInput != nil {
		var ti TxInfo
		if err := db.Get(txn, tr.TicketInput, &ti, db.HeaderTxInfo); err != nil {
			return err
		}
		addH := tx.Inout2key(tr.Hash(), tx.TypeTicketin, 0)
		delH := tx.Inout2key(tr.TicketInput, tx.TypeTicketout, 0)
		if err := updateAddressToTx(txn, ti.Body.TicketOutput, addH, delH); err != nil {
			return err
		}
	}
	for i, inp := range tr.Inputs {
		var ti TxInfo
		if err := db.Get(txn, inp.PreviousTX, &ti, db.HeaderTxInfo); err != nil {
			return err
		}
		adr := ti.Body.Outputs[inp.Index].Address
		addH := tx.Inout2key(tr.Hash(), tx.TypeIn, byte(i))
		delH := tx.Inout2key(inp.PreviousTX, tx.TypeOut, inp.Index)
		if err := updateAddressToTx(txn, adr, addH, delH); err != nil {
			return err
		}
	}
	return nil
}

func putMultisigInAddressToTx(txn *badger.Txn, tr *tx.Transaction) error {
	for i, inp := range tr.MultiSigIns {
		var ti TxInfo
		if err := db.Get(txn, inp.PreviousTX, &ti, db.HeaderTxInfo); err != nil {
			return err
		}
		for _, adr := range ti.Body.MultiSigOuts[inp.Index].Addresses {
			addH := tx.Inout2key(tr.Hash(), tx.TypeMulin, byte(i))
			delH := tx.Inout2key(inp.PreviousTX, tx.TypeMulout, inp.Index)
			if err := updateAddressToTx(txn, adr, addH, delH); err != nil {
				return err
			}
		}
	}
	return nil
}

func putOutputAddressToTx(txn *badger.Txn, tr *tx.Transaction) error {
	if tr.TicketOutput != nil {
		addH := tx.Inout2key(tr.Hash(), tx.TypeTicketout, 0)
		if err := updateAddressToTx(txn, tr.TicketOutput, addH, nil); err != nil {
			return err
		}
	}
	for i, inp := range tr.Outputs {
		addH := tx.Inout2key(tr.Hash(), tx.TypeOut, byte(i))
		if err := updateAddressToTx(txn, inp.Address, addH, nil); err != nil {
			return err
		}
	}
	return nil
}

func putMultisigOutAddressToTx(txn *badger.Txn, tr *tx.Transaction) error {
	for i, out := range tr.MultiSigOuts {
		for _, adr := range out.Addresses {
			addH := tx.Inout2key(tr.Hash(), tx.TypeMulout, byte(i))
			if err := updateAddressToTx(txn, adr, addH, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

//PutAddressToTx stores related addresses with tr.
//should be called synchonously
func PutAddressToTx(txn *badger.Txn, tr *tx.Transaction) error {
	if err := putInputAddressToTx(txn, tr); err != nil {
		return err
	}
	if err := putMultisigInAddressToTx(txn, tr); err != nil {
		return err
	}
	if err := putOutputAddressToTx(txn, tr); err != nil {
		return err
	}
	return putMultisigOutAddressToTx(txn, tr)
}

//GetHisoty returns utxo (or all outputs) and input hashes associated with  address adr.
func GetHisoty(s *setting.Setting, adrstr string, utxoOnly bool) ([]*tx.InoutHash, error) {
	return GetHisoty2(&s.DBConfig, adrstr, utxoOnly)
}

//GetHisoty2 returns utxo (or all outputs) and input hashes associated with  address adr.
func GetHisoty2(s *aklib.DBConfig, adrstr string, utxoOnly bool) ([]*tx.InoutHash, error) {
	adrbyte, _, err := address.ParseAddress58(s.Config, adrstr)
	if err != nil {
		return nil, err
	}
	var hashes [][]byte
	err = s.DB.View(func(txn *badger.Txn) error {
		err2 := db.Get(txn, adrbyte, &hashes, db.HeaderAddressToTx)
		if err2 == badger.ErrKeyNotFound {
			return nil
		}
		return err2
	})
	if err != nil {
		return nil, err
	}
	ihs := make([]*tx.InoutHash, len(hashes))
	for i, h := range hashes {
		ih, err := tx.NewInoutHash(h)
		if err != nil {
			return nil, err
		}
		ihs[i] = ih
	}
	if !utxoOnly {
		ihs2 := make([]*tx.InoutHash, len(ihs))
		copy(ihs2, ihs)
		for _, ih := range ihs2 {
			if ih.Type == tx.TypeOut || ih.Type == tx.TypeMulout || ih.Type == tx.TypeTicketout {
				continue
			}
			tr, err := GetTxInfo(s.DB, ih.Hash)
			if err != nil {
				return nil, err
			}
			var ih2 *tx.InoutHash
			switch ih.Type {
			case tx.TypeIn:
				ih2 = &tx.InoutHash{
					Type:  tx.TypeOut,
					Hash:  tr.Body.Inputs[ih.Index].PreviousTX,
					Index: tr.Body.Inputs[ih.Index].Index,
				}
			case tx.TypeMulin:
				ih2 = &tx.InoutHash{
					Type:  tx.TypeMulout,
					Hash:  tr.Body.MultiSigIns[ih.Index].PreviousTX,
					Index: tr.Body.MultiSigIns[ih.Index].Index,
				}
			case tx.TypeTicketin:
				ih2 = &tx.InoutHash{
					Type:  tx.TypeTicketout,
					Hash:  tr.Body.TicketInput,
					Index: 0,
				}
			default:
				return nil, errors.New("invalid type")
			}
			i := sort.Search(len(ihs), func(i int) bool {
				return bytes.Compare(ihs[i].Bytes(), ih2.Bytes()) >= 0
			})
			if i >= len(hashes) || !bytes.Equal(ihs[i].Bytes(), ih2.Bytes()) {
				ihs = append(ihs[:i], append([]*tx.InoutHash{ih2}, ihs[i:]...)...)
			}
		}
	}
	return ihs, nil
}
