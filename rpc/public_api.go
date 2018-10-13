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
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/address"
	"github.com/AidosKuneen/aklib/arypack"
	"github.com/AidosKuneen/aklib/rpc"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/akconsensus"
	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/imesh/leaves"
	"github.com/AidosKuneen/aknode/msg"
	"github.com/AidosKuneen/aknode/node"
	"github.com/AidosKuneen/aknode/setting"
	"github.com/AidosKuneen/consensus"
)

func sendrawtx(conf *setting.Setting, req *rpc.Request, res *rpc.Response) error {
	var txent []byte
	typ := tx.TypeNormal
	n, err := parseParam(req, &txent, &typ)
	if err != nil {
		return err
	}
	if n != 1 && n != 2 {
		return errors.New("invalid #params")
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

func getnodeinfo(conf *setting.Setting, req *rpc.Request, res *rpc.Response) error {
	var bal *uint64
	if conf.RPCUser != "" {
		mutex.Lock()
		defer mutex.Unlock()
		_, total, err := wallet.GetAllUTXO(&conf.DBConfig, nil)
		if err != nil {
			return err
		}
		bal = &total
	}
	lid := akconsensus.LatestLedger().ID()
	res.Result = &rpc.NodeInfo{
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
		TxNo:            imesh.GetTxNo(),
		LatestLedger:    hex.EncodeToString(lid[:]),
	}
	return nil
}

func getleaves(conf *setting.Setting, req *rpc.Request, res *rpc.Response) error {
	num := tx.DefaultPreviousSize
	n, err := parseParam(req, &num)
	if err != nil {
		return err
	}
	if n != 1 && n != 0 {
		return errors.New("invalid #params")
	}
	ls := leaves.Get(num)
	hls := make([]string, len(ls))
	for i := range ls {
		hls[i] = hex.EncodeToString(ls[i])
	}
	res.Result = hls
	return nil
}

func getlasthistory(conf *setting.Setting, req *rpc.Request, res *rpc.Response) error {
	adr := ""
	n, err := parseParam(req, &adr)
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("invalid #params")
	}
	ihs, err := imesh.GetHisoty(conf, adr, true)
	if err != nil {
		return err
	}
	r := make([]*rpc.InoutHash, len(ihs))
	for i, ih := range ihs {
		r[i] = &rpc.InoutHash{
			Hash:  ih.Hash.String(),
			Type:  ih.Type,
			Index: ih.Index,
		}
	}
	res.Result = r
	return nil
}
func getrawtx(conf *setting.Setting, req *rpc.Request, res *rpc.Response) error {
	txid := ""
	jsonformat := false
	n, err := parseParam(req, &txid, &jsonformat)
	if err != nil {
		return err
	}
	if n != 1 && n != 2 {
		return errors.New("invalid #params")
	}
	id, err := hex.DecodeString(txid)
	if err != nil {
		return err
	}
	tr, err := imesh.GetTx(conf.DB, id)
	if err != nil {
		return err
	}
	if !jsonformat {
		res.Result = arypack.Marshal(tr)
		return nil
	}
	res.Result = tr
	return nil
}

func getminabletx(conf *setting.Setting, req *rpc.Request, res *rpc.Response) error {
	var v interface{}
	n, err := parseParam(req, &v)
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("invalid #params")
	}

	typ := tx.TypeRewardFee
	jsonformat := false
	min := 0.0
	switch v := v.(type) {
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
	if !jsonformat {
		res.Result = arypack.Marshal(tr)
	} else {
		res.Result = tr
	}
	return nil
}

func gettxsstatus(conf *setting.Setting, req *rpc.Request, res *rpc.Response) error {
	if len(req.Params) == 0 {
		return errors.New("must specify txid")
	}
	var data []string
	if err := json.Unmarshal(req.Params, &data); err != nil {
		return err
	}
	if len(data) == 0 {
		return errors.New("need txids")
	}
	if len(data) > 50 {
		return errors.New("array is too big")
	}

	r := make([]*rpc.TxStatus, 0, len(data))
	for _, txid := range data {
		tid, err := hex.DecodeString(txid)
		if err != nil {
			return err
		}
		ok, err := imesh.Has(conf, tid)
		if err != nil {
			return err
		}
		if !ok {
			r = append(r, &rpc.TxStatus{
				Hash: txid,
			})
			continue
		}
		tr, err := imesh.GetTxInfo(conf.DB, tid)
		if err != nil {
			return err
		}
		switch tr.StatNo {
		case imesh.StatusPending:
			r = append(r, &rpc.TxStatus{
				Hash:   txid,
				Exists: true,
			})
		default:
			if tr.IsRejected {
				r = append(r, &rpc.TxStatus{
					Hash:        txid,
					Exists:      true,
					IsRejected:  true,
					IsConfirmed: true,
					LedgerID:    hex.EncodeToString(tr.StatNo[:]),
				})
			} else {
				r = append(r, &rpc.TxStatus{
					Hash:        txid,
					Exists:      true,
					IsConfirmed: true,
					LedgerID:    hex.EncodeToString(tr.StatNo[:]),
				})
			}
		}
	}
	res.Result = r
	return nil
}

func getmultisiginfo(conf *setting.Setting, req *rpc.Request, res *rpc.Response) error {
	if len(req.Params) == 0 {
		return errors.New("must specify mutisig address")
	}
	var data []string
	if err := json.Unmarshal(req.Params, &data); err != nil {
		return err
	}
	if len(data) != 1 {
		return errors.New("mus specify one multisig address")
	}
	madr, err := address.ParseMultisigAddress(conf.Config, data[0])
	if err != nil {
		return err
	}
	mul, err := imesh.GetMultisig(conf.DB, madr)
	if err != nil {
		return err
	}
	res.Result = mul
	return nil
}

func getledger(conf *setting.Setting, req *rpc.Request, res *rpc.Response) error {
	txid := ""
	n, err := parseParam(req, &txid)
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("invalid #params")
	}
	id, err := hex.DecodeString(txid)
	if err != nil {
		return err
	}
	var led consensus.LedgerID
	copy(led[:], id)
	tr, err := akconsensus.GetLedger(conf, led)
	if err != nil {
		return err
	}
	res.Result = rpc.NewLedger(tr)
	return nil
}
