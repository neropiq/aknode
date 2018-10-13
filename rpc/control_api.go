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

	"github.com/AidosKuneen/aklib/address"
	"github.com/AidosKuneen/aklib/rpc"
	"github.com/AidosKuneen/aknode/node"
	"github.com/AidosKuneen/aknode/setting"
	"github.com/AidosKuneen/aknode/walletImpl"
)

func listpeer(conf *setting.Setting, req *rpc.Request, res *rpc.Response) error {
	res.Result = node.GetPeerlist()
	return nil
}
func dumpprivkey(conf *setting.Setting, req *rpc.Request, res *rpc.Response) error {
	mutex.RLock()
	defer mutex.RUnlock()
	if pwd == nil {
		return errors.New("call walletpassphrase first")
	}
	seed, err := wallet.DecryptSeed(pwd)
	if err != nil {
		return err
	}
	res.Result = address.HDSeed58(conf.Config, seed, pwd, false)
	return nil
}

func listbanned(conf *setting.Setting, req *rpc.Request, res *rpc.Response) error {
	bs := node.GetBanned()
	banned := make([]*rpc.Bans, 0, len(bs))
	for k, v := range bs {
		banned = append(banned, &rpc.Bans{
			Address: k,
			Created: v.Unix(),
			Until:   v.Add(node.BanTime).Unix(),
			Reason:  "node misbehaving",
		})
	}
	res.Result = banned
	return nil
}

func stop(conf *setting.Setting, req *rpc.Request, res *rpc.Response) error {
	res.Result = "aknode servere stopping"
	conf.Stop <- struct{}{}
	return nil
}

//Dump is a struct for dumpwallet RPC.
type dump struct {
	Wallet  *walletImpl.Wallet             `json:"wallet"`
	Hist    []*walletImpl.History          `json:"history"`
	Address map[string]*walletImpl.Address `json:"address"`
}

func dumpwallet(conf *setting.Setting, req *rpc.Request, res *rpc.Response) error {
	fname := ""
	n, err := parseParam(req, &fname)
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("invalid #params")
	}
	mutex.RLock()
	defer mutex.RUnlock()
	h, err := walletImpl.GetHistory(&conf.DBConfig)
	if err != nil {
		return err
	}
	adrs, err := walletImpl.GetAllAddress(&conf.DBConfig)
	if err != nil {
		return err
	}
	d := &dump{
		Wallet:  wallet,
		Hist:    h,
		Address: adrs,
	}
	dat, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(fname, dat, 0644)
}

func importwallet(conf *setting.Setting, req *rpc.Request, res *rpc.Response) error {
	fname := ""
	n, err := parseParam(req, &fname)
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
	var d dump
	if err := json.Unmarshal(dat, &d); err != nil {
		return err
	}
	mutex.Lock()
	defer mutex.Unlock()
	*wallet = *d.Wallet
	if err := walletImpl.PutHistory(&conf.DBConfig, d.Hist); err != nil {
		return err
	}
	for _, adr := range d.Address {
		if err := wallet.PutAddress(&conf.DBConfig, pwd, adr, false); err != nil {
			return err
		}
	}
	return nil
}
