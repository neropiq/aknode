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
	"encoding/hex"
	"errors"
	"log"
	"sync"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/address"
	"github.com/AidosKuneen/aklib/rpc"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/akconsensus"
	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/setting"
	"github.com/AidosKuneen/aknode/walletImpl"
	"github.com/AidosKuneen/consensus"
)

var mutex sync.RWMutex

const nConfirm = 100000

func getnewaddress(conf *setting.Setting, req *rpc.Request, res *rpc.Response) error {
	var acc string
	n, err := parseParam(req, &acc)
	if err != nil {
		return err
	}
	if n != 0 && n != 1 {
		return errors.New("invalid param length")
	}
	mutex.Lock()
	defer mutex.Unlock()
	if len(wallet.AddressPublic) == 0 {
		wallet.AccountName = acc
	} else {
		if wallet.AccountName != acc {
			return errors.New("invalid accout name")
		}
	}
	res.Result, err = wallet.NewPublicAddressFromPool(&conf.DBConfig)
	return err
}

func listaddressgroupings(conf *setting.Setting, req *rpc.Request, res *rpc.Response) error {
	mutex.RLock()
	defer mutex.RUnlock()
	var result [][][]interface{}
	var r0 [][]interface{}
	us := make(map[string]uint64)
	utxos, _, err := wallet.GetUTXO(&conf.DBConfig, pwd, true)
	if err != nil {
		return err
	}
	for _, utxo := range utxos {
		us[utxo.Address.String()] = utxo.Value
	}
	for _, adr := range wallet.AllAddress() {
		r1 := make([]interface{}, 0, 3)
		r1 = append(r1, adr)
		r1 = append(r1, float64(us[adr])/aklib.ADK)
		_, _, err := address.ParseAddress58(conf.Config, adr)
		if err != nil {
			return err
		}
		if _, ok := wallet.AddressPublic[adr]; ok {
			r1 = append(r1, wallet.AccountName)
		}
		r0 = append(r0, r1)
	}
	result = append(result, r0)
	res.Result = result
	return nil
}

func getbalance(conf *setting.Setting, req *rpc.Request, res *rpc.Response) error {
	accstr := "*"
	n, err := parseParam(req, &accstr)
	if err != nil {
		return err
	}
	if n > 3 {
		return errors.New("invalid param length")
	}
	mutex.RLock()
	defer mutex.RUnlock()
	if accstr != "*" && wallet.AccountName != accstr {
		return errors.New("invalid accout name")
	}
	_, bal, err := wallet.GetAllUTXO(&conf.DBConfig, pwd)
	res.Result = float64(bal) / 100000000
	return err
}

func listaccounts(conf *setting.Setting, req *rpc.Request, res *rpc.Response) error {
	mutex.RLock()
	defer mutex.RUnlock()
	result := make(map[string]float64)
	_, ba, err := wallet.GetAllUTXO(&conf.DBConfig, pwd)
	if err != nil {
		return err
	}
	result[wallet.AccountName] = float64(ba) / aklib.ADK
	res.Result = result
	return nil
}

//only 'isvalid' params is valid, others may be incorrect.
func validateaddress(conf *setting.Setting, req *rpc.Request, res *rpc.Response) error {
	var adrstr string
	n, err := parseParam(req, &adrstr)
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("invalid param length")
	}
	valid := false
	_, _, err = address.ParseAddress58(conf.Config, adrstr)
	if err == nil {
		valid = true
	}
	mutex.RLock()
	defer mutex.RUnlock()
	isMine := wallet.FindAddress(adrstr)
	infoi := rpc.Info{
		IsValid: valid,
		Address: adrstr,
		IsMine:  isMine,
	}
	t := false
	empty := ""
	if isMine {
		infoi.Account = &wallet.AccountName
		infoi.IsWatchOnly = &t
		infoi.IsScript = &t
		infoi.Pubkey = &empty
		infoi.IsCompressed = &t
	}
	res.Result = &infoi
	return nil
}

func settxfee(conf *setting.Setting, req *rpc.Request, res *rpc.Response) error {
	res.Result = true
	return nil
}

func gettransaction(conf *setting.Setting, req *rpc.Request, res *rpc.Response) error {
	var str string
	n, err := parseParam(req, &str)
	if err != nil {
		return err
	}
	if n != 1 && n != 2 {
		return errors.New("invalid param length")
	}
	txid, err := hex.DecodeString(str)
	if err != nil {
		return err
	}
	var amount int64
	var detailss []*rpc.Details
	tr, err := imesh.GetTxInfo(conf.DB, txid)
	if err != nil {
		return err
	}
	mutex.RLock()
	defer mutex.RUnlock()
	detailss = make([]*rpc.Details, 0, len(tr.Body.Inputs)+len(tr.Body.Outputs))
	for vout, out := range tr.Body.Outputs {
		dt, errr := newTransaction(conf, tr, out, int64(vout), false)
		if errr != nil {
			return errr
		}
		if dt.Account == nil {
			continue
		}
		amount += int64(out.Value)
		det, err := dt.ToDetail()
		if err != nil {
			return err
		}
		detailss = append(detailss, det)
	}
	for _, in := range tr.Body.Inputs {
		out, err := imesh.PreviousOutput(conf, in)
		if err != nil {
			return err
		}
		dt, errr := newTransaction(conf, tr, out, int64(in.Index), true)
		if errr != nil {
			return errr
		}
		if dt.Account == nil {
			continue
		}
		amount -= int64(out.Value)
		det, err := dt.ToDetail()
		if err != nil {
			return err
		}
		detailss = append(detailss, det)
	}
	nconf := 0
	var bt *int64
	var bh *string
	var bi *int64
	if tr.IsAccepted() {
		nconf = nConfirm
		var zero int64
		t := tr.Body.Time.Unix()
		emp := ""
		bt = &t
		bi = &zero
		bh = &emp
	}
	res.Result = &rpc.Gettx{
		Amount:            float64(amount) / aklib.ADK,
		Confirmations:     nconf,
		Blocktime:         bt,
		Blockhash:         bh,
		Blockindex:        bi,
		Txid:              hex.EncodeToString(txid),
		WalletConflicts:   []string{},
		Time:              tr.Body.Time.Unix(),
		TimeReceived:      tr.Received.Unix(),
		BIP125Replaceable: "no",
		Details:           detailss,
	}
	return nil
}

//dont supprt over 1000 txs.
func listtransactions(conf *setting.Setting, req *rpc.Request, res *rpc.Response) error {
	acc := "*"
	num := 10
	skip := 0
	n, err := parseParam(req, &acc, &num, &skip)
	if err != nil {
		return err
	}
	if n > 4 {
		return errors.New("invalid param length")
	}
	mutex.RLock()
	defer mutex.RUnlock()
	hist, err := walletImpl.GetHistory(&conf.DBConfig)
	if err != nil {
		return err
	}
	if acc != "*" && acc != wallet.AccountName {
		return errors.New("invalid accout name")
	}
	var ltx []*rpc.Transaction
	for skipped, i := 0, 0; i < len(hist) && len(ltx) < num; i++ {
		h := hist[len(hist)-i-1]
		if skipped++; skipped <= skip {
			continue
		}
		tr, err := imesh.GetTxInfo(conf.DB, h.Hash)
		if err != nil {
			return err
		}
		out, err := GetOutput(conf, h)
		if err != nil {
			return err
		}
		vout := h.Index
		if h.Type == tx.TypeIn {
			vout = tr.Body.Inputs[h.Index].Index
		}
		dt, err := newTransaction(conf, tr, out, int64(vout), h.Type == tx.TypeIn)
		if err != nil {
			return err
		}
		ltx = append(ltx, dt)
	}
	res.Result = ltx
	return nil
}

//not rpc func
func newTransaction(conf *setting.Setting, tr *imesh.TxInfo, out *tx.Output, vout int64, isInput bool) (*rpc.Transaction, error) {
	adr, err := address.Address58(conf.Config, out.Address)
	if err != nil {
		return nil, err
	}
	ok := wallet.FindAddress(adr)
	f := false
	value := int64(out.Value)
	if isInput {
		value = -value
	}
	dt := &rpc.Transaction{
		Address:           adr,
		Category:          "send",
		Amount:            float64(value) / aklib.ADK,
		Vout:              vout,
		Txid:              tr.Hash.String(),
		Walletconflicts:   []string{},
		Time:              tr.Body.Time.Unix(),
		TimeReceived:      tr.Received.Unix(),
		BIP125Replaceable: "no",
		Abandoned:         &f,
	}
	if ok {
		dt.Account = &wallet.AccountName
	}
	if tr.IsAccepted() {
		lid := hex.EncodeToString(tr.StatNo[:])
		l, err := akconsensus.GetLedger(conf, consensus.LedgerID(tr.StatNo))
		if err != nil {
			log.Println(err, lid)
			return nil, err
		}
		s := int64(l.Seq)
		t := l.CloseTime.Unix()
		dt.Blockhash = &lid
		dt.Blocktime = &t
		dt.Blockindex = &s
		dt.Confirmations = nConfirm
	} else {
		dt.Trusted = &f
	}
	if value > 0 {
		dt.Category = "receive"
		dt.Abandoned = nil
	}
	return dt, nil
}

func getaccount(conf *setting.Setting, req *rpc.Request, res *rpc.Response) error {
	adr := ""
	n, err := parseParam(req, &adr)
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("invalid param length")
	}
	mutex.RLock()
	defer mutex.RUnlock()
	ok := wallet.FindAddress(adr)
	if !ok {
		return errors.New("address not found")
	}
	res.Result = wallet.AccountName
	return nil
}
