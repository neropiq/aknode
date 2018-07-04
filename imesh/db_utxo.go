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
	"encoding/hex"
	"errors"
	"log"
	"sort"

	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/setting"

	"github.com/dgraph-io/badger"
)

func updateAddressToTx(s *setting.Setting, txn *badger.Txn, adr []byte, addH, delH []byte) error {
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
			log.Println("not found", hex.EncodeToString(delH), "maybe double spend")
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
		addH := inout2key(tr.Hash(), TypeTicketin, 0)
		delH := inout2key(tr.TicketInput, TypeTicketout, 0)
		if err := updateAddressToTx(s, txn, ti.Body.TicketOutput, addH, delH); err != nil {
			return err
		}
	}
	for i, inp := range tr.Inputs {
		var ti TxInfo
		if err := db.Get(txn, inp.PreviousTX, &ti, db.HeaderTxInfo); err != nil {
			return err
		}
		adr := ti.Body.Outputs[inp.Index].Address
		addH := inout2key(tr.Hash(), TypeIn, byte(i))
		delH := inout2key(inp.PreviousTX, TypeOut, inp.Index)
		if err := updateAddressToTx(s, txn, adr, addH, delH); err != nil {
			return err
		}
	}
	return nil
}

func putMultisigInAddressToTx(s *setting.Setting, txn *badger.Txn, tr *tx.Transaction) error {
	for i, inp := range tr.MultiSigIns {
		var ti TxInfo
		if err := db.Get(txn, inp.PreviousTX, &ti, db.HeaderTxInfo); err != nil {
			return err
		}
		for _, adr := range ti.Body.MultiSigOuts[inp.Index].Addresses {
			addH := inout2key(tr.Hash(), TypeMulin, byte(i))
			delH := inout2key(inp.PreviousTX, TypeMulout, inp.Index)
			if err := updateAddressToTx(s, txn, adr, addH, delH); err != nil {
				return err
			}
		}
	}
	return nil
}

func putOutputAddressToTx(s *setting.Setting, txn *badger.Txn, tr *tx.Transaction) error {
	if tr.TicketOutput != nil {
		addH := inout2key(tr.Hash(), TypeTicketout, 0)
		if err := updateAddressToTx(s, txn, tr.TicketOutput, addH, nil); err != nil {
			return err
		}
	}
	for i, inp := range tr.Outputs {
		addH := inout2key(tr.Hash(), TypeOut, byte(i))
		if err := updateAddressToTx(s, txn, inp.Address, addH, nil); err != nil {
			return err
		}
	}
	return nil
}

func putMultisigOutAddressToTx(s *setting.Setting, txn *badger.Txn, tr *tx.Transaction) error {
	for i, out := range tr.MultiSigOuts {
		for _, adr := range out.Addresses {
			addH := inout2key(tr.Hash(), TypeMulout, byte(i))
			if err := updateAddressToTx(s, txn, adr, addH, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

//should be called synchonously, but check for sure.
func putAddressToTx(s *setting.Setting, txn *badger.Txn, tr *tx.Transaction) error {
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
}

//GetHisoty returns utxo (or all outputs) and input hashes associated with  address adr.
func GetHisoty(s *setting.Setting, adr []byte, utxoOnly bool) ([]*InoutHash, error) {
	var hashes [][]byte
	err := s.DB.View(func(txn *badger.Txn) error {
		err := db.Get(txn, adr, &hashes, db.HeaderAddressToTx)
		if err == badger.ErrKeyNotFound {
			return nil
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	ihs := make([]*InoutHash, len(hashes))
	for i, h := range hashes {
		ih, err := newInoutHash(h)
		if err != nil {
			return nil, err
		}
		ihs[i] = ih
	}
	if !utxoOnly {
		ihs2 := make([]*InoutHash, len(ihs))
		copy(ihs2, ihs)
		for _, ih := range ihs2 {
			if ih.Type == TypeOut || ih.Type == TypeMulout || ih.Type == TypeTicketout {
				continue
			}
			tr, err := GetTxInfo(s, ih.Hash)
			if err != nil {
				return nil, err
			}
			var ih2 *InoutHash
			switch ih.Type {
			case TypeIn:
				ih2 = &InoutHash{
					Type:  TypeOut,
					Hash:  tr.Body.Inputs[ih.Index].PreviousTX,
					Index: tr.Body.Inputs[ih.Index].Index,
				}
			case TypeMulin:
				ih2 = &InoutHash{
					Type:  TypeMulout,
					Hash:  tr.Body.MultiSigIns[ih.Index].PreviousTX,
					Index: tr.Body.MultiSigIns[ih.Index].Index,
				}
			case TypeTicketin:
				ih2 = &InoutHash{
					Type:  TypeTicketout,
					Hash:  tr.Body.TicketInput,
					Index: 0,
				}
			default:
				return nil, errors.New("invalid type")
			}
			i := sort.Search(len(ihs), func(i int) bool {
				return bytes.Compare(ihs[i].bytes(), ih2.bytes()) >= 0
			})
			if i >= len(hashes) || !bytes.Equal(ihs[i].bytes(), ih2.bytes()) {
				ihs = append(ihs[:i], append([]*InoutHash{ih2}, ihs[i:]...)...)
			}
		}
	}
	return ihs, nil
}
