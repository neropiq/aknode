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
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/AidosKuneen/aknode/akconsensus"
	"github.com/AidosKuneen/consensus"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/address"
	"github.com/AidosKuneen/aklib/arypack"
	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aklib/rand"
	"github.com/AidosKuneen/aklib/rpc"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/node"

	"github.com/dgraph-io/badger"
)

var (
	ledger *consensus.Ledger
)

func confirmAll(t *testing.T, notify chan []tx.Hash, confirm bool) {
	var txs []tx.Hash
	ledger = &consensus.Ledger{
		ParentID:  consensus.GenesisID,
		Seq:       1,
		CloseTime: time.Now(),
	}
	id := ledger.ID()
	t.Log("ledger id", hex.EncodeToString(id[:]))
	if err := akconsensus.PutLedger(&s, ledger); err != nil {
		t.Fatal(err)
	}
	err := s.DB.Update(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek([]byte{byte(db.HeaderTxInfo)}); it.ValidForPrefix([]byte{byte(db.HeaderTxInfo)}); it.Next() {
			var dat []byte
			err2 := it.Item().Value(func(d []byte) error {
				dat = make([]byte, len(d))
				copy(dat, d)
				return nil
			})
			if err2 != nil {
				return err2
			}
			var ti imesh.TxInfo
			if err := arypack.Unmarshal(dat, &ti); err != nil {
				return err
			}
			if confirm && ti.StatNo != imesh.StatusGenesis {
				ti.StatNo = imesh.StatNo(id)
			}
			if !confirm && ti.StatNo != imesh.StatusGenesis {
				ti.StatNo = imesh.StatusPending
			}
			h := it.Item().Key()[1:]
			if err := db.Put(txn, h, &ti, db.HeaderTxInfo); err != nil {
				return err
			}
			if !bytes.Equal(h, genesis) {
				txs = append(txs, h)
			}
		}
		return nil
	})
	if err != nil {
		t.Error(err)
	}

	if notify != nil {
		select {
		case notify <- txs:
		case <-time.After(5 * time.Second):
			t.Fatal("failed to notify")
		}
		t.Log("notifird", len(txs))
		for _, tr := range txs {
			select {
			case str := <-debugNotify:
				str = strings.TrimSpace(str)
				if str != tr.String() {
					t.Error("invalid walletnotify", str, tr)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("failed to get notify")
			}
		}
	}
}

func TestWalletAPI2(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	setup(ctx, t)
	defer teardown(t)
	defer cancel()
	pwd = []byte("pwd")
	if err := New(&s, pwd); err != nil {
		t.Error(err)
	}
	_, err := wallet.DecryptSeed(pwd)
	if err != nil {
		t.Error(err)
	}
	pwd = nil
	newAddressT(t, "")
	req := &rpc.Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "getnewaddress",
		Params:  json.RawMessage{},
	}
	params := []interface{}{"ac"}
	req.Params, err = json.Marshal(params)
	if err != nil {
		t.Error(err)
	}
	var resp rpc.Response
	if err := getnewaddress(&s, req, &resp); err == nil {
		t.Error("should  be error")
	}
}
func TestWalletAPI(t *testing.T) {
	debugNotify = make(chan string)
	ctx, cancel := context.WithCancel(context.Background())
	setup(ctx, t)
	defer teardown(t)
	defer cancel()
	pwdd := []byte("pwd")
	if err := New(&s, pwdd); err != nil {
		t.Fatal(err)
	}
	_, err := wallet.DecryptSeed(pwdd)
	if err != nil {
		t.Error(err)
	}
	pwd = nil
	var cnotify chan []tx.Hash
	GoNotify(ctx, &s, node.RegisterTxNotifier, func(ch chan []tx.Hash) {
		cnotify = ch
	})
	acs := []string{""}
	adr2ac := make(map[string]string)
	adr2val := make(map[string]uint64)
	ac2val := make(map[string]uint64)
	adrr := ""
	for _, ac := range acs {
		for _, adr := range newAddressT(t, ac) {
			t.Log(adr)
			adrr = adr
			adr2ac[adr] = ac
			adr2val[adr] = uint64(rand.R.Int31()) * 2
			ac2val[ac] += adr2val[adr]
		}
	}
	h := genesis
	var remain = aklib.ADKSupply
	var tss []*rpc.Transaction
	var tr *tx.Transaction
	var preadr string
	var prev uint64
	var amount int64
	ac2ts := make(map[string][]*rpc.Transaction)
	for adr, v := range adr2val {
		amount = 0
		ac := adr2ac[adr]
		tr = tx.New(s.Config, genesis)
		tr.AddInput(h, 0)
		if v >= aklib.ADKSupply {
			t.Fatal(v)
		}
		if preadr != "" {
			remain -= prev / 2
		}
		if err := tr.AddOutput(s.Config, a.Address58(s.Config), remain-v); err != nil {
			t.Error(err)
		}
		if err := tr.AddOutput(s.Config, adr, v); err != nil {
			t.Error(err)
		}
		if preadr != "" {
			tr.AddInput(h, 1)
			if err := tr.AddOutput(s.Config, preadr, prev/2); err != nil {
				t.Error(err)
			}
		}
		if err := tr.Sign(a); err != nil {
			t.Error(err)
		}
		if preadr != "" {
			pwd = pwdd
			gadr, err := wallet.GetAddress(&s.DBConfig, preadr, pwd)
			if err != nil {
				t.Error(err)
			}
			if err := gadr.Sign(tr); err != nil {
				t.Fatal(err)
			}
			pwd = nil
			ac2val[adr2ac[preadr]] -= prev / 2
			adr2val[preadr] /= 2
		}
		if err := tr.PoW(); err != nil {
			t.Error(err)
		}
		if err := imesh.CheckAddTx(&s, tr, tx.TypeNormal); err != nil {
			t.Fatal(preadr, err)
		}

		ts := &rpc.Transaction{
			Account: &ac,
			Address: adr,
			Amount:  float64(v) / aklib.ADK,
			Txid:    tr.Hash().String(),
			Time:    tr.Time.Unix(),
			Vout:    1,
		}
		amount += int64(v)
		t.Log(tr.Hash(), v)
		tss = append(tss, ts)
		ac2ts[ac] = append(ac2ts[ac], ts)
		if preadr != "" {
			preac := adr2ac[preadr]
			ts := &rpc.Transaction{
				Account: &preac,
				Address: preadr,
				Amount:  float64(prev/2) / aklib.ADK,
				Txid:    tr.Hash().String(),
				Time:    tr.Time.Unix(),
				Vout:    2,
			}
			tss = append(tss, ts)
			ac2ts[preac] = append(ac2ts[preac], ts)
			amount += int64(prev / 2)
			t.Log(tr.Hash(), prev/2)

			ts = &rpc.Transaction{
				Account: &preac,
				Address: preadr,
				Amount:  -float64(prev) / aklib.ADK,
				Txid:    tr.Hash().String(),
				Time:    tr.Time.Unix(),
				Vout:    1,
			}
			tss = append(tss, ts)
			ac2ts[preac] = append(ac2ts[preac], ts)
			amount -= int64(prev)

			t.Log(tr.Hash(), -int64(prev))
		}
		preadr = adr
		prev = v
		h = tr.Hash()
		node.Resolve()
		time.Sleep(6 * time.Second)
		select {
		case str := <-debugNotify:
			str = strings.TrimSpace(str)
			t.Log("notified", tr.Hash())
			if str != tr.Hash().String() {
				t.Error("invalid walletnotify", str, tr.Hash())
			}
		case <-time.After(5 * time.Second):
			t.Fatal("failed to get notified")
		}
	}

	for _, ac := range acs {
		testlisttransactions(t, ac, ac2ts[ac], false)
	}

	confirmAll(t, cnotify, true)
	testgetaccount(t, adrr, adr2ac[adrr])
	testvalidateaddress1(t, "AKADRSD2smqjURgpGy3iYx67Vr3nGs7jqe444Hi7vkabhNSvnc58UDVNV", true)
	testvalidateaddress1(t, "AKADRSD2smqjURgpGy3iYx67Vr3nGs7jqe444Hi7vkabhNSvnc58UDVNa", false)
	testvalidateaddress2(t, adrr, adr2ac[adrr])
	testListAccounts(t, ac2val, acs...)
	testlistaddressgroupings(t, adr2ac, adr2val)

	for _, ac := range acs {
		testgetbalance(t, ac, ac2val)
		testlisttransactions(t, ac, ac2ts[ac], true)
	}
	testgetbalance2(t, ac2val)
	testlisttransactions2(t, true, adr2ac, tss)
	testgettransaction(t, adr2ac, tr, float64(amount)/aklib.ADK, true)
}

func testgetaccount(t *testing.T, adr, ac string) {
	req := &rpc.Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "getaccount",
	}
	params := []interface{}{adr}
	var err error
	req.Params, err = json.Marshal(params)
	if err != nil {
		t.Error(err)
	}
	var resp rpc.Response
	if err := getaccount(&s, req, &resp); err != nil {
		t.Error(err, adr, ac)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	result, ok := resp.Result.(string)
	if !ok {
		t.Error("result must be slice")
	}
	if ac != result {
		t.Error("invalid getaccount")
	}
}

func testlistaddressgroupings(t *testing.T, adr2ac map[string]string, adr2val map[string]uint64) {
	req := &rpc.Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "listaddressgroupings",
	}
	var resp rpc.Response
	if err := listaddressgroupings(&s, req, &resp); err != nil {
		t.Error(err)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	result, ok := resp.Result.([][][]interface{})
	if !ok {
		t.Error("result must be slice")
	}
	if len(result) != 1 {
		t.Error("result length must be 1, but", len(result))
	}
	if len(result[0]) != len(adr2val) {
		t.Error("result length must be ", len(adr2val), ",but", len(result[0]))
	}
	for i := range result[0] {
		adr, ok := result[0][i][0].(string)
		if !ok {
			t.Error("result[0][i][0] must be address")
		}
		acc, ok := result[0][i][2].(string)
		if !ok {
			t.Error("result[0][i][2] must be string")
		}
		v, ok := adr2val[adr]
		if !ok {
			t.Error("invalid adrress")
		}
		val, ok := result[0][i][1].(float64)
		if !ok {
			t.Error("result[0][i][1] must be float")
		}
		if float64(v)/aklib.ADK != val {
			t.Error("invalid value", v, val)
		}
		acc2, ok := adr2ac[adr]
		if !ok {
			t.Error("invalid address")
		}
		if acc2 != acc {
			t.Error("invalid account")
		}
	}
}
func testvalidateaddress2(t *testing.T, adr, ac string) {
	req := &rpc.Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "validateaddress",
	}
	params := []interface{}{adr}
	var err error
	req.Params, err = json.Marshal(params)
	if err != nil {
		t.Error(err)
	}
	var resp rpc.Response
	if err := validateaddress(&s, req, &resp); err != nil {
		t.Error(err)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	result, ok := resp.Result.(*rpc.Info)
	if !ok {
		t.Error("result must be info struct")
	}
	if *result.Account != ac || *result.IsCompressed ||
		*result.Pubkey != "" || *result.IsScript ||
		*result.IsWatchOnly {
		t.Error("params must be empty")
	}
	if !result.IsValid {
		t.Error("address must be valid")
	}
	if result.Address != string(adr) {
		t.Error("invalid address")
	}
	if result.ScriptPubKey != "" {
		t.Error("scriptpubkey must be empty")
	}
	if !result.IsMine {
		t.Error("address should be mine")
	}
}

func testvalidateaddress1(t *testing.T, adr string, isValid bool) {
	req := &rpc.Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "validateaddress",
	}
	params := []interface{}{adr}
	var err error
	req.Params, err = json.Marshal(params)
	if err != nil {
		t.Error(err)
	}
	var resp rpc.Response
	if err := validateaddress(&s, req, &resp); err != nil {
		t.Error(err)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	result, ok := resp.Result.(*rpc.Info)
	if !ok {
		t.Error("result must be info struct")
	}
	if result.Account != nil || result.IsCompressed != nil ||
		result.Pubkey != nil || result.IsScript != nil ||
		result.IsWatchOnly != nil {
		t.Error("params must be nil")
	}
	if result.IsValid != isValid {
		t.Fatal("validity of address must be ", isValid, adr)
	}
	if result.Address != adr {
		t.Error("invalid address")
	}
	if result.ScriptPubKey != "" {
		t.Error("scriptpubkey must be empty")
	}
	if result.IsMine {
		t.Error("address should not be mine")
	}
}

func testgettransaction(t *testing.T, adr2ac map[string]string, tr *tx.Transaction, amount float64, isConf bool) {
	req := &rpc.Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "gettransaction",
	}
	params := []interface{}{tr.Hash().String()}
	var err error
	req.Params, err = json.Marshal(params)
	if err != nil {
		t.Error(err)
	}
	var resp rpc.Response
	if err := gettransaction(&s, req, &resp); err != nil {
		t.Error(err)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	tx, ok := resp.Result.(*rpc.Gettx)
	if !ok {
		t.Error("result must be tx")
	}
	if tx.Amount != amount {
		t.Error("amount is incorrect", tx.Amount, "should be", amount)
	}
	if tx.Fee != 0 ||
		len(tx.WalletConflicts) != 0 || tx.BIP125Replaceable != "no" || tx.Hex != "" {
		t.Error("invalid dummy params")
	}
	if isConf {
		if tx.Confirmations != 100000 {
			t.Error("invalid confirmations")
		}
		if *tx.Blockhash != "" || *tx.Blockindex != 0 || *tx.Blocktime != tx.Time {
			t.Error("invalid block params", *tx.Blockhash, *tx.Blockindex, *tx.Blocktime)
		}
	} else {
		if tx.Confirmations != 0 {
			t.Error("invalid confirmations")
		}
		if tx.Blockhash != nil || tx.Blockindex != nil || tx.Blocktime != nil {
			t.Error("invalid block params")
		}
	}
	if tx.Txid != tr.Hash().String() {
		t.Error("invalid txid", tx.Txid, "should be", tr.Hash())
	}
	ok = false
	if tx.Time == tr.Time.Unix() {
		ok = true
	}
	if !ok {
		t.Error("invalid time", tx.Time)
	}
	if tx.TimeReceived-tx.Time > 1000 {
		t.Error("invalid timereceived")
	}

	j := 0
	for _, out := range tr.Body.Outputs {
		if _, ok := adr2ac[out.Address.String()]; !ok {
			continue
		}
		d := tx.Details[j]
		j++
		if d.Address != out.Address.String() {
			t.Error("invalid address", d.Address, out.Address.String())
		}
		adr := d.Address
		acc, ok := adr2ac[adr]
		if !ok || acc != d.Account {
			t.Error("invalid account")
		}
		if d.Amount < 0 || d.Category != "receive" {
			t.Error("invalid category")
		}
		if d.Amount != float64(out.Value)/aklib.ADK {
			t.Error("invalid amount", d.Amount, out.Value, adr)
		}
		if d.Fee != 0 {
			t.Error("invalid dummy params")
		}
		if d.Abandoned != nil {
			t.Error("invalid abandone")
		}
	}
	for _, in := range tr.Body.Inputs {
		txi, err := imesh.GetTxInfo(s.DB, in.PreviousTX)
		if err != nil {
			t.Error(err)
		}
		out := txi.Body.Outputs[in.Index]
		if _, ok := adr2ac[out.Address.String()]; !ok {
			continue
		}
		d := tx.Details[j]
		j++
		if d.Address != out.Address.String() {
			t.Error("invalid address", d.Address, out.Address.String())
		}
		adr := d.Address
		acc, ok := adr2ac[adr]
		if !ok || acc != d.Account {
			t.Error("invalid account")
		}
		if d.Amount > 0 || d.Category != "send" {
			t.Error("invalid category")
		}
		if d.Amount != -float64(out.Value)/100000000 {
			t.Error("invalid amount", d.Amount, out.Value, adr)
		}
		if d.Fee != 0 {
			t.Error("invalid dummy params")
		}
		if d.Abandoned == nil || *d.Abandoned {
			t.Error("invalid abandone")
		}
	}
	if j != len(tx.Details) {
		t.Error("invalid number of length ")
	}
}

func testgetbalance(t *testing.T, ac string, ac2val map[string]uint64) {
	req := &rpc.Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "getbalance",
	}
	params := []interface{}{ac}
	var err error
	req.Params, err = json.Marshal(params)
	if err != nil {
		t.Error(err)
	}

	var resp rpc.Response
	if err := getbalance(&s, req, &resp); err != nil {
		t.Error(err)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	result, ok := resp.Result.(float64)
	if !ok {
		t.Error("result must be float64")
	}
	if result != float64(ac2val[ac])/100000000 {
		t.Error("invalid balance", result, ac, ac2val[ac])
	}
}

func testgetbalance2(t *testing.T, ac2val map[string]uint64) {
	req := &rpc.Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "getbalance",
	}

	var resp rpc.Response
	if err := getbalance(&s, req, &resp); err != nil {
		t.Error(err)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	result, ok := resp.Result.(float64)
	if !ok {
		t.Error("result must be float64")
	}
	var total uint64
	for _, v := range ac2val {
		total += v
	}
	if result != float64(total)/100000000 {
		t.Error("invalid balance", result, total)
	}
}

func testlisttransactions(t *testing.T, ac string, hashes []*rpc.Transaction, isConf bool) {
	skip := 1
	count := 2

	req := &rpc.Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "listtransactions",
	}
	params := []interface{}{ac, float64(count), float64(skip)}
	var err error
	req.Params, err = json.Marshal(params)
	if err != nil {
		t.Error(err)
	}

	var resp rpc.Response
	if err := listtransactions(&s, req, &resp); err != nil {
		t.Error(err)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	result, ok := resp.Result.([]*rpc.Transaction)
	if !ok {
		t.Error("result must be transaction struct")
	}
	if len(result) != count {
		t.Error("invalid number of txs", len(result), ac)
	}
	// var last int64 = math.MaxInt64
	for i := range result {
		tx := result[i]
		otx := hashes[len(hashes)-skip-i-1]
		t.Log(tx.Txid, tx.Amount, otx.Txid, otx.Amount)
		if *tx.Account != ac {
			t.Error("invalid account")
		}
		if tx.Address != otx.Address {
			t.Error("invalid address", tx.Address, otx.Address)
		}
		if tx.Amount > 0 && tx.Category != "receive" {
			t.Error("invalid category")
		}
		if tx.Amount < 0 && tx.Category != "send" {
			t.Error("invalid category")
		}
		if tx.Amount == 0 {
			t.Error(" amount should not be 0")
		}
		if tx.Amount != otx.Amount {
			t.Error("invalid amount", tx.Amount, otx.Amount, ac)
		}
		if tx.Time != otx.Time {
			t.Error("invalid time")
		}
		if tx.Txid != otx.Txid {
			t.Error("invalid txid,", tx.Txid, "should be", otx.Txid)
		}
		if tx.TimeReceived-tx.Time > 60*60 {
			t.Error("time received is wrong")
		}
		conf := 100000
		if !isConf {
			conf = 0
		}
		if tx.Confirmations != conf {
			t.Error("invalid confirmations", tx.Confirmations, "should be", conf)
		}
		if tx.Vout != otx.Vout {
			t.Error("invalid vout")
		}
		if tx.Fee != 0 ||
			len(tx.Walletconflicts) != 0 || tx.BIP125Replaceable != "no" {
			t.Error("invalid dummy params")
		}
		if isConf {
			id := ledger.ID()
			if *tx.Blockhash != hex.EncodeToString(id[:]) || *tx.Blockindex != int64(ledger.Seq) ||
				*tx.Blocktime != ledger.CloseTime.Unix() {
				t.Error("invalid block params")
			}
			if tx.Trusted != nil {
				t.Error("invalid trusted")
			}
		} else {
			if tx.Blockhash != nil || tx.Blockindex != nil || tx.Blocktime != nil {
				t.Error("invalid block params")
			}
			if *tx.Trusted {
				t.Error("invalid trusted")
			}
		}
		if tx.Category == "receive" && tx.Abandoned != nil {
			t.Error("invalid abandone")
		}
		if tx.Category == "send" && (tx.Abandoned == nil || *tx.Abandoned) {
			t.Error("invalid abandone")
		}
	}
}

func testlisttransactions2(t *testing.T, isConf bool, adr2ac map[string]string, txs []*rpc.Transaction) {
	req := &rpc.Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "listtransactions",
	}
	params := []interface{}{"*", 100.0}
	var err error
	req.Params, err = json.Marshal(params)
	if err != nil {
		t.Error(err)
	}

	var resp rpc.Response
	if err := listtransactions(&s, req, &resp); err != nil {
		t.Error(err)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	result, ok := resp.Result.([]*rpc.Transaction)
	if !ok {
		t.Error("result must be transaction struct")
	}
	if len(result) != len(txs) {
		t.Error("invalid number of txs", len(result), len(txs))
	}
	// var last int64 = math.MaxInt64
	for i := range result {
		tx := result[i]
		otx := txs[len(txs)-i-1]
		if *tx.Account != adr2ac[tx.Address] {
			t.Error("invalid account")
		}
		if tx.Address != otx.Address {
			t.Error("invalid address", tx.Address, otx.Address)
		}
		if tx.Amount > 0 && tx.Category != "receive" {
			t.Error("invalid category")
		}
		if tx.Amount < 0 && tx.Category != "send" {
			t.Error("invalid category")
		}
		if tx.Amount == 0 {
			t.Error(" amount should not be 0")
		}
		if tx.Amount != otx.Amount {
			t.Error("invalid amount", tx.Amount)
		}
		if tx.Time != otx.Time {
			t.Error("invalid time")
		}
		if tx.TimeReceived-tx.Time > 60*60 {
			t.Error("time received is wrong")
		}
		// if tx.Time > last {
		// 	t.Error("invalid order")
		// }
		// last = tx.Time
		if tx.Txid != otx.Txid {
			t.Error("invalid txid")
		}
		conf := 100000
		if !isConf {
			conf = 0
		}
		if tx.Confirmations != conf {
			t.Error("invalid confirmations")
		}
		if tx.Fee != 0 ||
			len(tx.Walletconflicts) != 0 || tx.BIP125Replaceable != "no" {
			t.Error("invalid dummy params")
		}
		if tx.Vout != otx.Vout {
			t.Error("invalid vout")
		}
		if isConf {
			id := ledger.ID()
			if *tx.Blockhash != hex.EncodeToString(id[:]) || *tx.Blockindex != int64(ledger.Seq) ||
				*tx.Blocktime != ledger.CloseTime.Unix() {
				t.Error("invalid block params")
			}
			if tx.Trusted != nil {
				t.Error("invalid trusted")
			}
		} else {
			if tx.Blockhash != nil || tx.Blockindex != nil || tx.Blocktime != nil {
				t.Error("invalid block params")
			}
			if *tx.Trusted {
				t.Error("invalid trusted")
			}
		}
		if tx.Category == "receive" && tx.Abandoned != nil {
			t.Error("invalid abandone")
		}
		if tx.Category == "send" && (tx.Abandoned == nil || *tx.Abandoned) {
			t.Error("invalid abandone")
		}
	}
}

func testListAccounts(t *testing.T, ac2val map[string]uint64, acc ...string) {
	req := &rpc.Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "listaccounts",
	}
	var resp rpc.Response
	if err := listaccounts(&s, req, &resp); err != nil {
		t.Error(err)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	result, ok := resp.Result.(map[string]float64)
	if !ok {
		t.Error("result must be map")
	}
	if len(result) != len(acc) {
		t.Error("result length is incorrect")
	}
	for ac := range ac2val {
		if result[ac] != float64(ac2val[ac])/aklib.ADK {
			t.Error("invalid balance", ac, result[ac], "must be", ac2val[ac])
		}
	}
}

func newAddressT(t *testing.T, ac string) []string {
	adrs := make([]string, 3)
	req := &rpc.Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "getnewaddress",
		Params:  json.RawMessage{},
	}
	//test for default
	if ac != "" {
		params := []interface{}{ac}
		var err error
		req.Params, err = json.Marshal(params)
		if err != nil {
			t.Error(err)
		}
	}
	var resp rpc.Response
	for i := range adrs {
		if err := getnewaddress(&s, req, &resp); err != nil {
			t.Error(err)
		}
		if resp.Error != nil {
			t.Error("should not be error")
		}
		adrstr, ok := resp.Result.(string)
		if !ok {
			t.Error("result must be string")
		}
		if _, _, err := address.ParseAddress58(s.Config, adrstr); err != nil {
			t.Error(err)
		}
		adrs[i] = adrstr
	}
	return adrs
}
