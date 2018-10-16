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
	"context"
	"encoding/hex"
	"log"
	"os/exec"
	"sort"
	"strings"

	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/setting"
	"github.com/AidosKuneen/aknode/walletImpl"

	shellwords "github.com/mattn/go-shellwords"
)

const walletVersion = 1

var pwd []byte

var wallet *walletImpl.Wallet

//Init initialize wallet struct.
func Init(s *setting.Setting) error {
	var err error
	wallet, err = walletImpl.Load(&s.DBConfig, nil, "")
	return err
}

//IsSecretEmpty returns true if wallet is empty.
func IsSecretEmpty() bool {
	return wallet.EncSeed == nil
}

//New initialize the wallet.
func New(s *setting.Setting, pwdd []byte) error {
	if err := wallet.InitSeed(&s.DBConfig, pwdd); err != nil {
		return err
	}
	wallet.Pool = &walletImpl.Pool{}
	return wallet.FillPool(&s.DBConfig, pwdd)
}

//GetOutput returns an output related to InOutHash ih.
//If ih is output, returns the output specified by ih.
//If ih is input, return output refered by the input.
func GetOutput(s *setting.Setting, h *walletImpl.History) (*tx.Output, error) {
	return imesh.GetOutput(s, h.InoutHash)
}

//GoNotify runs gorouitine to get history of addresses in wallet.
//This func needs to run even if RPC is stopped for collecting history.
func GoNotify(ctx context.Context, s *setting.Setting, nreg, creg func(chan []tx.Hash)) {
	nnotify := make(chan []tx.Hash, 10)
	nreg(nnotify)
	cnotify := make(chan []tx.Hash, 10)

	if s.WalletNotify != "" {
		creg(cnotify)
		go func() {
			ctx2, cancel2 := context.WithCancel(ctx)
			defer cancel2()
			for {
				select {
				case <-ctx2.Done():
					return
				case noti := <-cnotify:
					if err := walletnotifyRunCommand(s, noti); err != nil {
						log.Println(err)
					}
				}
			}
		}()
	}
	go func() {
		ctx2, cancel2 := context.WithCancel(ctx)
		defer cancel2()
		for {
			select {
			case <-ctx2.Done():
				return
			case noti := <-nnotify:
				trs := make([]*imesh.TxInfo, 0, len(noti))
				for _, h := range noti {
					tr, err := imesh.GetTxInfo(s.DB, h)
					if err != nil {
						log.Println(err)
					}
					trs = append(trs, tr)
				}
				sort.Slice(trs, func(i, j int) bool {
					return trs[i].Received.Before(trs[j].Received)
				})
				if err := walletnotifyUpdate(s, trs); err != nil {
					log.Println(err)
				}
				if s.WalletNotify != "" {
					cnotify <- noti
				}
			}
		}
	}()
}

var debugNotify chan string

func walletnotifyRunCommand(s *setting.Setting, noti []tx.Hash) error {
start:
	for _, h := range noti {
		tr, err := imesh.GetTxInfo(s.DB, h)
		if err != nil {
			return err
		}
		for _, out := range tr.Body.Outputs {
			if !wallet.FindAddress(out.Address.String()) {
				continue
			}
			str, err := runCommand(s, h)
			if err != nil {
				return err
			}
			if debugNotify != nil {
				debugNotify <- str
			}
			continue start
		}
		for _, in := range tr.Body.Inputs {
			out, err := imesh.PreviousOutput(s, in)
			if err != nil {
				return err
			}
			if !wallet.FindAddress(out.Address.String()) {
				continue
			}
			str, err := runCommand(s, h)
			if err != nil {
				return err
			}
			if debugNotify != nil {
				debugNotify <- str
			}
			continue start
		}
		log.Println("didn't run cmd", h)
	}
	return nil
}
func walletnotifyUpdate(s *setting.Setting, trs []*imesh.TxInfo) error {
	mutex.Lock()
	defer mutex.Unlock()
	hist, err := walletImpl.GetHistory(&s.DBConfig)
	if err != nil {
		return err
	}
	for _, tr := range trs {
		for i, out := range tr.Body.Outputs {
			if !wallet.FindAddress(out.Address.String()) {
				continue
			}
			hist = append(hist, &walletImpl.History{
				InoutHash: &tx.InoutHash{
					Type:  tx.TypeOut,
					Hash:  tr.Hash,
					Index: byte(i),
				},
				Received: tr.Received,
			})
		}
		for i, in := range tr.Body.Inputs {
			out, err := imesh.PreviousOutput(s, in)
			if err != nil {
				return err
			}
			if !wallet.FindAddress(out.Address.String()) {
				continue
			}
			hist = append(hist, &walletImpl.History{
				InoutHash: &tx.InoutHash{
					Type:  tx.TypeIn,
					Hash:  tr.Hash,
					Index: byte(i),
				},
				Received: tr.Received,
			})
		}
		if err := walletImpl.PutHistory(&s.DBConfig, hist); err != nil {
			return err
		}
	}
	return nil
}

func runCommand(conf *setting.Setting, h tx.Hash) (string, error) {
	if conf.WalletNotify == "" {
		return "", nil
	}
	cmd := strings.Replace(conf.WalletNotify, "%s", hex.EncodeToString(h), -1)
	args, err := shellwords.Parse(cmd)
	if err != nil {
		return "", err
	}
	var out []byte
	if len(args) == 1 {
		out, err = exec.Command(args[0]).Output()
	} else {
		out, err = exec.Command(args[0], args[1:]...).Output()
	}
	if err != nil {
		return "", err
	}
	log.Println("executed ", cmd, ",output:", string(out))
	return string(out), nil
}
