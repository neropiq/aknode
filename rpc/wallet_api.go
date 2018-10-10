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

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/address"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/setting"
	"github.com/AidosKuneen/aknode/walletImpl"
)

var mutex sync.RWMutex

const nConfirm = 100000

func getnewaddress(conf *setting.Setting, req *Request, res *Response) error {
	var acc string
	n, err := req.parseParam(&acc)
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

func listaddressgroupings(conf *setting.Setting, req *Request, res *Response) error {
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

func getbalance(conf *setting.Setting, req *Request, res *Response) error {
	accstr := "*"
	n, err := req.parseParam(&accstr)
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

func listaccounts(conf *setting.Setting, req *Request, res *Response) error {
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

//Info is a struct for validateaddress RPC.
type Info struct {
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
	var adrstr string
	n, err := req.parseParam(&adrstr)
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
	infoi := Info{
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

func settxfee(conf *setting.Setting, req *Request, res *Response) error {
	res.Result = true
	return nil
}

//Details is a struct for gettransaction RPC.
type Details struct {
	Account   string  `json:"account"`
	Address   string  `json:"address"`
	Category  string  `json:"category"`
	Amount    float64 `json:"amount"`
	Vout      int64   `json:"vout"`
	Fee       float64 `json:"fee"`
	Abandoned *bool   `json:"abandoned,omitempty"`
}

//Gettx is a struct for gettransaction RPC.
type Gettx struct {
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
	Details           []*Details `json:"details"`
	Hex               string     `json:"hex"`
}

func gettransaction(conf *setting.Setting, req *Request, res *Response) error {
	var str string
	n, err := req.parseParam(&str)
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
	var detailss []*Details
	tr, err := imesh.GetTxInfo(conf.DB, txid)
	if err != nil {
		return err
	}
	mutex.RLock()
	defer mutex.RUnlock()
	detailss = make([]*Details, 0, len(tr.Body.Inputs)+len(tr.Body.Outputs))
	for vout, out := range tr.Body.Outputs {
		dt, errr := newTransaction(conf, tr, out, int64(vout), false)
		if errr != nil {
			return errr
		}
		if dt.Account == nil {
			continue
		}
		amount += int64(out.Value)
		det, err := dt.toDetail()
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
		det, err := dt.toDetail()
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
	res.Result = &Gettx{
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

//Transaction is a struct for listtransactions RPC.
type Transaction struct {
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

//not RPC func
func (dt *Transaction) toDetail() (*Details, error) {
	if dt.Account == nil {
		return nil, errors.New("Account is nil")
	}
	return &Details{
		Account:   *dt.Account,
		Address:   dt.Address,
		Category:  dt.Category,
		Amount:    dt.Amount,
		Vout:      dt.Vout,
		Abandoned: dt.Abandoned,
	}, nil
}

//dont supprt over 1000 txs.
func listtransactions(conf *setting.Setting, req *Request, res *Response) error {
	acc := "*"
	num := 10
	skip := 0
	n, err := req.parseParam(&acc, &num, &skip)
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
	var ltx []*Transaction
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
func newTransaction(conf *setting.Setting, tr *imesh.TxInfo, out *tx.Output, vout int64, isInput bool) (*Transaction, error) {
	adr, err := address.Address58(conf.Config, out.Address)
	if err != nil {
		return nil, err
	}
	ok := wallet.FindAddress(adr)
	f := false
	emp := ""
	var zero int64
	value := int64(out.Value)
	if isInput {
		value = -value
	}
	dt := &Transaction{
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
	adr := ""
	n, err := req.parseParam(&adr)
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
