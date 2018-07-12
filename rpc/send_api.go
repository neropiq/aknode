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
	"encoding/json"
	"errors"
	"time"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aknode/setting"
)

func walletpassphrase(conf *setting.Setting, req *Request, res *Response) error {
	data, ok := req.Params.([]interface{})
	if !ok {
		return errors.New("invalid params")
	}
	if len(data) != 2 {
		return errors.New("invalid param length")
	}
	pwd, ok := data[0].(string)
	if !ok {
		return errors.New("invalid password")
	}
	sec, ok := data[1].(time.Duration)
	if !ok {
		return errors.New("invalid time")
	}
	mutex.Lock()
	defer mutex.Lock()
	if wallet.Secret.pwd != nil {
		return errors.New("wallet is already unlocked")
	}
	if err := decryptSecret(conf, []byte(pwd)); err != nil {
		return err
	}
	if err := fillPool(conf); err != nil {
		return err
	}
	go func() {
		time.Sleep(time.Second * sec)
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
	mutex.Lock()
	defer mutex.Unlock()
	if wallet.Secret.pwd == nil {
		return errors.New("not priviledged")
	}
	data, ok := req.Params.([]interface{})
	if !ok {
		return errors.New("invalid params")
	}
	if len(data) < 2 || len(data) > 5 {
		return errors.New("invalid param length")
	}
	acc, ok := data[0].(string)
	if !ok {
		return errors.New("invalid account")
	}
	target := make(map[string]float64)
	switch data[1].(type) {
	case string:
		t := data[1].(string)
		if err := json.Unmarshal([]byte(t), &target); err != nil {
			return err
		}
	case map[string]interface{}:
		t := data[1].(map[string]interface{})
		for k, v := range t {
			f, ok := v.(float64)
			if !ok {
				return errors.New("param must be a  map string")
			}
			target[k] = f
		}
	default:
		return errors.New("param must be a  map string")
	}
	trs := make([]output, len(target))
	i := 0
	var err error
	for k, v := range target {
		trs[i].address = k
		trs[i].value = uint64(v * aklib.ADK)
		i++
	}
	res.Result, err = Send(conf, acc, []byte(conf.RPCTxTag), trs...)
	return err
}

func sendfrom(conf *setting.Setting, req *Request, res *Response) error {
	var err error
	mutex.Lock()
	defer mutex.Unlock()
	if wallet.Secret.pwd == nil {
		return errors.New("not priviledged")
	}
	data, ok := req.Params.([]interface{})
	if !ok {
		return errors.New("invalid params")
	}
	if len(data) < 3 || len(data) > 6 {
		return errors.New("invalid params")
	}
	acc, ok := data[0].(string)
	if !ok {
		return errors.New("invalid account")
	}
	adrstr, ok := data[1].(string)
	if !ok {
		return errors.New("invalid address")
	}
	value, ok := data[2].(float64)
	if !ok {
		return errors.New("invalid value")
	}
	res.Result, err = Send(conf, acc, []byte(conf.RPCTxTag), output{
		address: adrstr,
		value:   uint64(value * aklib.ADK),
	})
	return err
}

func sendtoaddress(conf *setting.Setting, req *Request, res *Response) error {
	var err error
	mutex.Lock()
	defer mutex.Unlock()
	if wallet.Secret.pwd == nil {
		return errors.New("not priviledged")
	}
	data, ok := req.Params.([]interface{})
	if !ok {
		return errors.New("invalid params")
	}
	if len(data) > 5 || len(data) < 2 {
		return errors.New("invalid params")
	}
	adrstr, ok := data[0].(string)
	if !ok {
		return errors.New("invalid address")
	}
	value, ok := data[1].(float64)
	if !ok {
		return errors.New("invalid value")
	}
	res.Result, err = Send(conf, "*", []byte(conf.RPCTxTag), output{
		address: adrstr,
		value:   uint64(value * aklib.ADK),
	})
	return err
}
