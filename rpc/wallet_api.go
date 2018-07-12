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
	"sync"

	"github.com/AidosKuneen/aknode/imesh"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/address"
	"github.com/AidosKuneen/aklib/tx"

	"github.com/AidosKuneen/aknode/setting"
)

var mutex sync.RWMutex

const nConfirm = 100000

func getnewaddress(conf *setting.Setting, req *Request, res *Response) error {
	data, ok := req.Params.([]interface{})
	if !ok {
		return errors.New("invalid params")
	}
	acc := ""
	switch len(data) {
	case 1:
		acc, ok = data[0].(string)
		if !ok {
			return errors.New("invalid txid")
		}
	case 0:
	default:
		return errors.New("invalid params")
	}
	var err error
	mutex.Lock()
	res.Result, err = newAddress10(conf, acc)
	mutex.Unlock()
	if err != nil {
		return err
	}
	return nil
}

func listaddressgroupings(conf *setting.Setting, req *Request, res *Response) error {
	mutex.RLock()
	defer mutex.RUnlock()
	var result [][][]interface{}
	var r0 [][]interface{}
	for acc := range wallet.Accounts {
		utxos, _, err := getUTXO(conf, acc, false)
		if err != nil {
			return err
		}
		for _, utxo := range utxos {
			r1 := make([]interface{}, 3)
			r1[0] = utxo.addressName
			r1[1] = float64(utxo.value) / aklib.ADK
			r1[2] = acc
			r0 = append(r0, r1)
			result = append(result, r0)
		}
	}
	res.Result = result
	return nil
}

func getbalance(conf *setting.Setting, req *Request, res *Response) error {
	mutex.RLock()
	defer mutex.RUnlock()
	data, ok := req.Params.([]interface{})
	if !ok {
		return errors.New("param must be slice")
	}
	accstr := "*"
	switch len(data) {
	case 3:
		fallthrough
	case 2:
		n, okk := data[1].(float64)
		if !okk {
			return errors.New("invalid number")
		}
		if n == 0 {
			return errors.New("not support unconfirmed transactions")
		}
		fallthrough
	case 1:
		accstr, ok = data[0].(string)
		if !ok {
			return errors.New("invalid address")
		}
	case 0:
	default:
		return errors.New("invalid params")
	}
	var bal uint64
	var err error
	if accstr != "*" {
		_, bal, err = getUTXO(conf, accstr, false)
		if err != nil {
			return err
		}
	} else {
		for acc := range wallet.Accounts {
			_, ba, err := getUTXO(conf, acc, false)
			if err != nil {
				return err
			}
			bal += ba
		}
	}
	res.Result = float64(bal) / 100000000
	return nil
}

func listaccounts(conf *setting.Setting, req *Request, res *Response) error {
	mutex.RLock()
	defer mutex.RUnlock()
	ary, ok := req.Params.([]interface{})
	if !ok {
		return errors.New("invalid param")
	}
	if len(ary) > 0 {
		conf, ok := ary[0].(float64)
		if !ok {
			return errors.New("invalid param")
		}
		if conf == 0 {
			return errors.New("not support unconfirmed transacton")
		}
	}
	result := make(map[string]float64)
	for acc := range wallet.Accounts {
		_, ba, err := getUTXO(conf, acc, false)
		if err != nil {
			return err
		}
		result[acc] = float64(ba) / aklib.ADKSupply
	}
	res.Result = result
	return nil
}

type info struct {
	IsValid      bool    `json:"isvalid"`
	Address      string  `json:"address"`
	ScriptPubKey string  `json:"scriptPubkey"`
	IsMine       bool    `json:"ismine"`
	IsWatchOnly  *bool   `json:"iswatchonly,omitempty"`
	IsScript     *bool   `json:"isscript,omitempty"`
	Pubkey       *string `json:"pubkey,omitempty"`
	IsCompressed *bool   `json:"iscompressed,omitempty"`
	Account      *string `json:"account,omitempty"`
}

//only 'isvalid' params is valid, others may be incorrect.
func validateaddress(conf *setting.Setting, req *Request, res *Response) error {
	data, ok := req.Params.([]interface{})
	if !ok {
		return errors.New("invalid params")
	}
	if len(data) != 1 {
		return errors.New("length of param must be 1")
	}
	adrstr, ok := data[0].(string)
	if !ok {
		return errors.New("invalid address")
	}
	valid := false
	_, _, err := address.ParseAddress58(adrstr, conf.Config)
	if err == nil {
		valid = true
	}
	mutex.RLock()
	defer mutex.RUnlock()
	ac, isMine := findAddress(adrstr)
	infoi := info{
		IsValid: valid,
		Address: adrstr,
		IsMine:  isMine,
	}
	t := false
	empty := ""
	if isMine {
		infoi.Account = &ac
		infoi.IsWatchOnly = &t
		infoi.IsScript = &t
		infoi.Pubkey = &empty
		infoi.IsCompressed = &t
	}
	res.Result = &infoi
	return nil
}

func settxfee(conf *setting.Setting, req *Request, res *Response) error {
	res.Result = true
	return nil
}

type details struct {
	Account   string  `json:"account"`
	Address   string  `json:"address"`
	Category  string  `json:"category"`
	Amount    float64 `json:"amount"`
	Vout      int64   `json:"vout"`
	Fee       float64 `json:"fee"`
	Abandoned *bool   `json:"abandoned,omitempty"`
}

type gettx struct {
	Amount            float64    `json:"amount"`
	Fee               float64    `json:"fee"`
	Confirmations     int        `json:"confirmations"`
	Blockhash         *string    `json:"blockhash,omitempty"`
	Blockindex        *int64     `json:"blockindex,omitempty"`
	Blocktime         *int64     `json:"blocktime,omitempty"`
	Txid              string     `json:"txid"`
	WalletConflicts   []string   `json:"walletconflicts"`
	Time              int64      `json:"time"`
	TimeReceived      int64      `json:"timereceived"`
	BIP125Replaceable string     `json:"bip125-replaceable"`
	Details           []*details `json:"details"`
	Hex               string     `json:"hex"`
}

func gettransaction(conf *setting.Setting, req *Request, res *Response) error {
	data, ok := req.Params.([]interface{})
	if !ok {
		return errors.New("invalid params")
	}
	var txid tx.Hash
	var err error
	switch len(data) {
	case 2:
	case 1:
		str, ok := data[0].(string)
		if !ok {
			return errors.New("invalid txid")
		}
		txid, err = hex.DecodeString(str)
		if err != nil {
			return err
		}
	default:
		return errors.New("invalid params")
	}
	var amount int64
	var dt *transaction
	var detailss []*details
	tr, err := imesh.GetTxInfo(conf, txid)
	if err != nil {
		return err
	}
	detailss = make([]*details, 0, len(tr.Body.Inputs)+len(tr.Body.Outputs))
	for _, out := range tr.Body.Outputs {
		dt, errr := newTransaction(tr, txid, out, false)
		if errr != nil {
			return errr
		}
		amount += int64(out.Value)
		detailss = append(detailss, dt.toDetail())
	}
	for _, in := range tr.Body.Inputs {
		out, err := imesh.PreviousOutput(conf, in)
		if err != nil {
			return err
		}
		dt, errr := newTransaction(tr, txid, out, true)
		if errr != nil {
			return errr
		}
		amount -= int64(out.Value)
		detailss = append(detailss, dt.toDetail())
	}
	nconf := 0
	if tr.Status == imesh.StatusConfirmed {
		nconf = nConfirm
	}
	res.Result = &gettx{
		Amount:            float64(amount) / aklib.ADK,
		Confirmations:     nconf,
		Blocktime:         dt.Blocktime,
		Blockhash:         dt.Blockhash,
		Blockindex:        dt.Blockindex,
		Txid:              hex.EncodeToString(txid),
		WalletConflicts:   []string{},
		Time:              dt.Time,
		TimeReceived:      dt.TimeReceived,
		BIP125Replaceable: "no",
		Details:           detailss,
	}
	return nil
}

type transaction struct {
	Account  *string `json:"account"`
	Address  string  `json:"address"`
	Category string  `json:"category"`
	Amount   float64 `json:"amount"`
	// Label             string      `json:"label"`
	Vout          int64   `json:"vout"`
	Fee           float64 `json:"fee"`
	Confirmations int     `json:"confirmations"`
	Trusted       *bool   `json:"trusted,omitempty"`
	// Generated         bool        `json:"generated"`
	Blockhash       *string  `json:"blockhash,omitempty"`
	Blockindex      *int64   `json:"blockindex,omitempty"`
	Blocktime       *int64   `json:"blocktime,omitempty"`
	Txid            string   `json:"txid"`
	Walletconflicts []string `json:"walletconflicts"`
	Time            int64    `json:"time"`
	TimeReceived    int64    `json:"timereceived"`
	// Comment           string      `json:"string"`
	// To                string `json:"to"`
	// Otheraccount      string `json:"otheraccount"`
	BIP125Replaceable string `json:"bip125-replaceable"`
	Abandoned         *bool  `json:"abandoned,omitempty"`
}

func (dt *transaction) toDetail() *details {
	return &details{
		Account:   *dt.Account,
		Address:   dt.Address,
		Category:  dt.Category,
		Amount:    dt.Amount,
		Abandoned: dt.Abandoned,
	}
}

//dont supprt over 1000 txs.
func listtransactions(conf *setting.Setting, req *Request, res *Response) error {
	data, ok := req.Params.([]interface{})
	if !ok {
		return errors.New("invalid params")
	}
	acc := "*"
	num := 10
	skip := 0
	switch len(data) {
	case 4:
		fallthrough
	case 3:
		n, okk := data[2].(float64)
		if !okk {
			return errors.New("invalid number")
		}
		skip = int(n)
		fallthrough
	case 2:
		n, okk := data[1].(float64)
		if !okk {
			return errors.New("invalid number")
		}
		num = int(n)
		fallthrough
	case 1:
		acc, ok = data[0].(string)
		if !ok {
			return errors.New("invalid account")
		}
	case 0:
	default:
		return errors.New("invalid params")
	}
	mutex.RLock()
	defer mutex.RUnlock()
	hist, err := getHistory(conf)
	if err != nil {
		return err
	}
	var ltx []*transaction
	for skipped, i := 0, 0; i < len(hist) && len(ltx) < num; i++ {
		h := hist[len(hist)-i-1]
		if acc != "*" && h.Account != acc {
			continue
		}
		if skipped++; skipped < skip {
			continue
		}
		tr, err := imesh.GetTxInfo(conf, h.Hash)
		if err != nil {
			return err
		}
		out, err := h.GetOutput(conf)
		if err != nil {
			return err
		}
		dt, err := newTransaction(tr, h.Hash, out, h.Type == imesh.TypeIn)
		if err != nil {
			return err
		}
		ltx = append(ltx, dt)
	}
	res.Result = ltx
	return nil
}

func newTransaction(tr *imesh.TxInfo, hash tx.Hash, out *tx.Output, isInput bool) (*transaction, error) {
	adr := address.To58(out.Address)
	mutex.Lock()
	ac, ok := findAddress(adr)
	mutex.Unlock()
	f := false
	emp := ""
	var zero int64
	value := int64(out.Value)
	if isInput {
		value = -value
	}
	dt := &transaction{
		Address:           adr,
		Category:          "send",
		Amount:            float64(value) / aklib.ADK,
		Txid:              hex.EncodeToString(hash),
		Walletconflicts:   []string{},
		Time:              tr.Body.Time.Unix(),
		TimeReceived:      tr.Received.Unix(),
		BIP125Replaceable: "no",
		Abandoned:         &f,
	}
	if ok {
		dt.Account = &ac
	}
	if tr.Status == imesh.StatusConfirmed {
		dt.Blockhash = &emp
		dt.Blocktime = &dt.Time
		dt.Blockindex = &zero
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

func getaccount(conf *setting.Setting, req *Request, res *Response) error {
	data, ok := req.Params.([]interface{})
	if !ok {
		return errors.New("invalid params")
	}
	adr := ""
	switch len(data) {
	case 1:
		adr, ok = data[0].(string)
		if !ok {
			return errors.New("invalid account")
		}
	default:
		return errors.New("invalid params")
	}
	mutex.RLock()
	defer mutex.RUnlock()
	res.Result, ok = findAddress(adr)
	if !ok {
		return errors.New("address not found")
	}
	return nil
}
