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
	"io/ioutil"
	"log"

	"github.com/AidosKuneen/aklib/address"
	"github.com/AidosKuneen/aknode/node"
	"github.com/AidosKuneen/aknode/setting"
)

func listpeer(conf *setting.Setting, req *Request, res *Response) error {
	res.Result = node.GetPeerlist()
	return nil
}
func dumpprivkey(conf *setting.Setting, req *Request, res *Response) error {
	mutex.RLock()
	defer mutex.RUnlock()
	if wallet.Secret.seed == nil || wallet.Secret.pwd == nil {
		return errors.New("call walletpassphrase first")
	}
	res.Result = address.HDSeed58(conf.Config, wallet.Secret.seed, wallet.Secret.pwd, false)
	return nil
}

//Bans is a struct for listbanned RPC.
type Bans struct {
	Address string `json:"address"`
	Created int64  `json:"ban_created"`
	Until   int64  `json:"banned_until"`
	Reason  string `json:"ban_reason"`
}

func listbanned(conf *setting.Setting, req *Request, res *Response) error {
	bs := node.GetBanned()
	banned := make([]*Bans, 0, len(bs))
	for k, v := range bs {
		banned = append(banned, &Bans{
			Address: k,
			Created: v.Unix(),
			Until:   v.Add(node.BanTime).Unix(),
			Reason:  "node misbehaving",
		})
	}
	res.Result = banned
	return nil
}

func stop(conf *setting.Setting, req *Request, res *Response) error {
	res.Result = "aknode servere stopping"
	conf.Stop <- struct{}{}
	return nil
}

//Dump is a struct for dumpwallet RPC.
type Dump struct {
	Wallet  *Wallet             `json:"wallet"`
	Hist    []*History          `json:"history"`
	Address map[string]*Address `json:"address"`
}

func dumpwallet(conf *setting.Setting, req *Request, res *Response) error {
	log.Println(req.Params)
	fname := ""
	n, err := req.parseParam(&fname)
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("invalid #params")
	}
	log.Println(fname)
	mutex.RLock()
	defer mutex.RUnlock()
	h, err := getHistory(conf)
	if err != nil {
		return err
	}
	adrs, err := getAllAddress(conf)
	if err != nil {
		return err
	}
	d := &Dump{
		Wallet:  &wallet,
		Hist:    h,
		Address: adrs,
	}
	dat, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(fname, dat, 0644)
}

func importwallet(conf *setting.Setting, req *Request, res *Response) error {
	fname := ""
	n, err := req.parseParam(&fname)
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("invalid #params")
	}
	dat, err := ioutil.ReadFile(fname)
	if err != nil {
		return err
	}
	var d Dump
	if err := json.Unmarshal(dat, &d); err != nil {
		return err
	}
	mutex.Lock()
	defer mutex.Unlock()
	pwd := wallet.Secret.pwd
	wallet = *d.Wallet
	wallet.Secret.pwd = pwd
	wallet.conf = conf
	if err := putHistory(conf, d.Hist); err != nil {
		return err
	}
	for _, adr := range d.Address {
		if err := putAddress(conf, adr, false); err != nil {
			return err
		}
	}
	return nil
}
