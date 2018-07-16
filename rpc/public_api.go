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
	var txent []byte
	typ := tx.TypeNormal
	switch len(data) {
	case 2:
		typ, ok = data[1].(tx.Type)
		if !ok {
			return errors.New("invalid type")
		}
		fallthrough
	case 1:
		txent, ok = data[0].([]byte)
		if !ok {
			return errors.New("invalid txid")
		}
	default:
		return errors.New("invalid params")
	}
	if typ == tx.TypeNotPoWed && !conf.RPCAllowPublicPoW {
		return errors.New("PoW is not allowed")
	}
	var tr tx.Transaction
	if err := arypack.Unmarshal(txent, &tr); err != nil {
		return err
	}
	if err2 := imesh.IsValid(conf, &tr, typ); err2 != nil {
		return err2
	}
	if typ == tx.TypeNotPoWed {
		log.Println("started PoW for sendrawtx")
		if err2 := tr.PoW(); err2 != nil {
			return err2
		}
		typ = tx.TypeNormal
	}
	if err := imesh.CheckAddTx(conf, &tr, typ); err != nil {
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
	Hash  string              `json:"hash"`
	Type  imesh.InOutHashType `json:"type"`
	Index byte                `json:"index"`
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
			Hash:  ih.Hash.String(),
			Type:  ih.Type,
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
		res.Result = arypack.Marshal(tr)
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
	typ := tx.TypeRewardFee
	min := 0.0
	switch len(data) {
	case 1:
		switch v := data[0].(type) {
		case string:
			if v != "ticket" {
				return errors.New("invalid type")
			}
			typ = tx.TypeRewardTicket
		case float64:
			min = v
		default:
			return errors.New("invalid type")
		}
	default:
		return errors.New("invalid params")
	}
	var err error
	var tr *tx.Transaction
	if typ == tx.TypeRewardTicket {
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
	res.Result = arypack.Marshal(tr)
	return nil
}
