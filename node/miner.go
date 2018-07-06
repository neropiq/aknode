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
	"log"

	"github.com/AidosKuneen/aklib/address"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/setting"
)

var mineCh = make(chan *imesh.HashWithType, 1)

//AddForMine adds a minable tx for mine.
func addForMine(s *setting.Setting, h tx.Hash, typ tx.Type) {
	if (typ == tx.TxRewardFee && s.RunFeeMiner) ||
		(typ == tx.TxRewardTicket && s.RunTicketMiner) {
		if len(mineCh) != 0 {
			<-mineCh
		}
		mineCh <- &imesh.HashWithType{
			Hash: h,
			Type: typ,
		}
	}
}

func mine(s *setting.Setting, mtx *imesh.HashWithType) error {
	tr, err := imesh.GetMinableTx(s, mtx.Hash, mtx.Type)
	if err != nil {
		return err
	}
	madr, err := address.FromAddress58(s.MinerAddress)
	if err != nil {
		return err
	}
	switch mtx.Type {
	case tx.TxRewardFee:
		tr.Outputs[len(tr.Outputs)-1].Address = madr
	case tx.TxRewardTicket:
		tr.TicketOutput = madr
	}
	if err := tr.PoW(); err != nil {
		return err
	}
	if err := imesh.CheckAddTx(s, tr, tx.TxNormal); err != nil {
		return err
	}
	Resolve()
	log.Println("succeeded to mine, txid=", tr.Hash())
	return nil
}

//RunMiner runs a miner
func RunMiner(s *setting.Setting) {
	go func() {
		for h := range mineCh {
			if err := mine(s, h); err != nil {
				log.Println(err)
			}
		}
	}()

}
