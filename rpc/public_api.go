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
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log"
	"time"

	"github.com/AidosKuneen/aklib"

	"github.com/AidosKuneen/aknode/imesh/leaves"

	"github.com/AidosKuneen/aklib/arypack"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/msg"
	"github.com/AidosKuneen/aknode/node"
	"github.com/AidosKuneen/aknode/setting"
)

func sendrawtx(conf *setting.Setting, req *Request, res *Response) error {
	data, ok := req.Params.([]interface{})
	if !ok {
		return errors.New("invalid params")
	}
	txid := ""
	switch len(data) {
	case 2:
		fallthrough
	case 1:
		txid, ok = data[0].(string)
		if !ok {
			return errors.New("invalid txid")
		}
	case 0:
	default:
		return errors.New("invalid params")
	}
	id, err := base64.StdEncoding.DecodeString(txid)
	if err != nil {
		return err
	}
	var tr tx.Transaction
	if err := arypack.Unmarshal(id, &tr); err != nil {
		return err
	}
	if err := imesh.IsValid(conf, &tr, tx.TxNormal); err != nil {
		return err
	}
	if err := imesh.CheckAddTx(conf, &tr, tx.TxNormal); err != nil {
		return err
	}
	node.Resolve()
	res.Result = hex.EncodeToString(tr.Hash())
	return nil
}

type nodeInfo struct {
	Version         int     `json:"version"`
	ProtocolVersion int     `json:"protocolversion"`
	WalletVersion   int     `json:"walletversion"`
	Balance         *uint64 `json:"balance,omitempty"`
	Connections     int     `json:"connections"`
	Proxy           string  `json:"proxy"`
	Testnet         byte    `json:"testnet"`
	KeyPoolSize     int     `json:"keypoolsize"`
	Leaves          int     `json:"leaves"`
	Time            int64   `json:"time"`
	//TODO:some value for consensus
}

func getnodeinfo(conf *setting.Setting, req *Request, res *Response) error {
	var bal *uint64
	if conf.RPCUser != "" {
		mutex.Lock()
		defer mutex.Unlock()
		var total uint64
		for ac := range wallet.Accounts {
			_, b, err := getUTXO(conf, ac, false)
			if err != nil {
				return err
			}
			total += b
		}
		bal = &total
	}
	res.Result = &nodeInfo{
		Version:         setting.Version,
		ProtocolVersion: msg.MessageVersion,
		WalletVersion:   walletVersion,
		Balance:         bal,
		Connections:     node.ConnSize(),
		Proxy:           conf.Proxy,
		Testnet:         conf.Testnet,
		KeyPoolSize:     len(wallet.Pool.Address),
		Leaves:          leaves.Size(),
		Time:            time.Now().Unix(),
	}
	return nil
}

func getleaves(conf *setting.Setting, req *Request, res *Response) error {
	data, ok := req.Params.([]interface{})
	if !ok {
		return errors.New("invalid params")
	}
	num := 0
	switch len(data) {
	case 1:
		num, ok = data[0].(int)
		if !ok {
			return errors.New("invalid txid")
		}
	case 0:
		num = tx.DefaultPreviousSize
	default:
		return errors.New("invalid params")
	}
	ls := leaves.Get(num)
	hls := make([]string, len(ls))
	for i := range ls {
		hls[i] = hex.EncodeToString(ls[i])
	}
	res.Result = hls
	return nil
}

type inoutHash struct {
	Hash  tx.Hash `json:"hash"`
	Type  string  `json:"type"`
	Index byte    `json:"index"`
}

func getlasthistory(conf *setting.Setting, req *Request, res *Response) error {
	data, ok := req.Params.([]interface{})
	if !ok {
		return errors.New("invalid params")
	}
	adr := ""
	switch len(data) {
	case 1:
		adr, ok = data[0].(string)
		if !ok {
			return errors.New("invalid txid")
		}
	default:
		return errors.New("invalid params")
	}
	ihs, err := imesh.GetHisoty(conf, adr, true)
	if err != nil {
		return err
	}
	r := make([]*inoutHash, len(ihs))
	for i, ih := range ihs {
		r[i] = &inoutHash{
			Hash:  ih.Hash,
			Type:  ih.Type.String(),
			Index: ih.Index,
		}
	}
	res.Result = r
	return nil
}
func getrawtx(conf *setting.Setting, req *Request, res *Response) error {
	data, ok := req.Params.([]interface{})
	if !ok {
		return errors.New("invalid params")
	}
	txid := ""
	format := false
	switch len(data) {
	case 2:
		format, ok = data[1].(bool)
		if !ok {
			return errors.New("invalid format")
		}
		fallthrough
	case 1:
		txid, ok = data[0].(string)
		if !ok {
			return errors.New("invalid txid")
		}
	default:
		return errors.New("invalid params")
	}
	id, err := hex.DecodeString(txid)
	if err != nil {
		return err
	}
	tr, err := imesh.GetTx(conf, id)
	if err != nil {
		return err
	}
	if !format {
		res.Result = base64.StdEncoding.EncodeToString(arypack.Marshal(tr))
		return nil
	}
	res.Result = tr
	return nil
}

func getminabletx(conf *setting.Setting, req *Request, res *Response) error {
	data, ok := req.Params.([]interface{})
	if !ok {
		return errors.New("invalid params")
	}
	typ := tx.TxRewardFee
	min := 0.0
	switch len(data) {
	case 1:
		switch v := data[0].(type) {
		case string:
			if v != "ticket" {
				return errors.New("invalid type")
			}
			typ = tx.TxRewardTicket
			log.Println("ticket")
		case float64:
			min = v
			log.Println("fee", min)
		default:
			return errors.New("invalid type")
		}
	}
	var err error
	var tr *tx.Transaction
	if typ == tx.TxRewardTicket {
		tr, err = imesh.GetRandomTicketTx(conf)
		if err != nil {
			return err
		}
	} else {
		tr, err = imesh.GetRandomFeeTx(conf, uint64(min*aklib.ADK))
		if err != nil {
			return err
		}
	}
	res.Result = base64.StdEncoding.EncodeToString(arypack.Marshal(tr))
	return nil
}
