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

package node

import (
	"context"
	"encoding/hex"
	"errors"
	"log"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/address"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/imesh/leaves"
	"github.com/AidosKuneen/aknode/setting"
)

var mineCh chan *tx.HashWithType

//AddForMine adds a minable tx for mine.
func addForMine(tr *tx.HashWithType) {
	if len(mineCh) != 0 {
		<-mineCh
	}
	mineCh <- tr
}

func mine(s *setting.Setting, mtx *tx.HashWithType) error {
	tr, err := imesh.GetMinableTx(s, mtx.Hash, mtx.Type)
	if err != nil {
		return err
	}
	if mtx.Type == tx.TypeRewardFee && uint64(s.MinimumFee*aklib.ADK) > tr.Outputs[len(tr.Outputs)-1].Value {
		return nil
	}
	madr, _, err := address.ParseAddress58(s.Config, s.MinerAddress)
	if err != nil {
		return err
	}
	switch mtx.Type {
	case tx.TypeRewardFee:
		tr.Outputs[len(tr.Outputs)-1].Address = madr
	case tx.TypeRewardTicket:
		tr.TicketOutput = madr
	default:
		return errors.New("invalid type")
	}
	log.Println("mining", hex.EncodeToString(tr.Hash()))
	if err := tr.PoW(); err != nil {
		return err
	}
	if err := imesh.CheckAddTx(s, tr, tx.TypeNormal); err != nil {
		return err
	}
	Resolve()
	log.Println("succeeded to mine, txid=", hex.EncodeToString(tr.Hash()))
	return nil
}

func issueTicket(ctx context.Context, s *setting.Setting) error {
	madr, _, err := address.ParseAddress58(s.Config, s.MinerAddress)
	if err != nil {
		log.Fatal(err)
	}
	tr, err := tx.IssueTicket(ctx, s.Config, madr, leaves.Get(0)...)
	if err != nil {
		log.Println(err)
	}
	if err := imesh.CheckAddTx(s, tr, tx.TypeNormal); err != nil {
		log.Println(err)
	}
	Resolve()
	log.Println("ticket issued,", tr.Hash())
	return nil
}

//RunMiner runs a miner
func RunMiner(ctx context.Context, s *setting.Setting) {
	mineCh = make(chan *tx.HashWithType, 1)

	if s.RunTicketIssuer {
		go func() {
			ctx2, cancel2 := context.WithCancel(ctx)
			defer cancel2()
			for {
				if err := issueTicket(ctx, s); err != nil {
					log.Println(err)
				}
				select {
				case <-ctx2.Done():
					return
				default:
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
			case h := <-mineCh:
				if err := mine(s, h); err != nil {
					log.Println(err)
				}
			}
		}
	}()

}
