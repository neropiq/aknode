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
	"errors"
	"time"

	"github.com/AidosKuneen/aklib/tx"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aknode/setting"
)

func walletpassphrase(conf *setting.Setting, req *Request, res *Response) error {
	var pwd string
	var sec uint
	n, err := req.parseParam(&pwd, &sec)
	if err != nil {
		return err
	}
	if n != 2 {
		return errors.New("invalid #params")
	}
	mutex.Lock()
	defer mutex.Unlock()
	if wallet.Secret.pwd != nil {
		return errors.New("wallet is already unlocked")
	}
	if err := decryptSecret(conf, []byte(pwd)); err != nil {
		return err
	}
	go func() {
		time.Sleep(time.Second * time.Duration(sec))
		mutex.Lock()
		clearSecret()
		mutex.Unlock()
	}()
	return nil
}

func walletlock(conf *setting.Setting, req *Request, res *Response) error {
	mutex.Lock()
	defer mutex.Unlock()
	clearSecret()
	return nil
}

func sendmany(conf *setting.Setting, req *Request, res *Response) error {
	var acc string
	target := map[string]float64{}
	n, err := req.parseParam(&acc, &target)
	if err != nil {
		return err
	}
	if n < 2 || n > 5 {
		return errors.New("invalid param length")
	}
	mutex.Lock()
	defer mutex.Unlock()
	if wallet.Secret.pwd == nil {
		return errors.New("not priviledged")
	}
	trs := make([]*tx.RawOutput, len(target))
	i := 0
	for k, v := range target {
		trs[i] = &tx.RawOutput{
			Address: k,
			Value:   uint64(v * aklib.ADK),
		}
		i++
	}
	res.Result, err = Send(conf, acc, []byte(conf.RPCTxTag), trs...)
	return err
}

func sendfrom(conf *setting.Setting, req *Request, res *Response) error {
	var acc, adrstr string
	var value float64
	n, err := req.parseParam(&acc, &adrstr, &value)
	if err != nil {
		return err
	}
	if n < 3 || n > 6 {
		return errors.New("invalid param length")
	}
	mutex.Lock()
	defer mutex.Unlock()
	if wallet.Secret.pwd == nil {
		return errors.New("not priviledged")
	}
	res.Result, err = Send(conf, acc, []byte(conf.RPCTxTag), &tx.RawOutput{
		Address: adrstr,
		Value:   uint64(value * aklib.ADK),
	})
	return err
}

func sendtoaddress(conf *setting.Setting, req *Request, res *Response) error {
	var adrstr string
	var value float64
	n, err := req.parseParam(&adrstr, &value)
	if err != nil {
		return err
	}
	if n > 5 || n < 2 {
		return errors.New("invalid param length")
	}

	mutex.Lock()
	defer mutex.Unlock()
	if wallet.Secret.pwd == nil {
		return errors.New("not priviledged")
	}
	res.Result, err = Send(conf, "*", []byte(conf.RPCTxTag), &tx.RawOutput{
		Address: adrstr,
		Value:   uint64(value * aklib.ADK),
	})
	return err
}
