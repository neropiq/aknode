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
	"encoding/hex"
	"log"
	"time"

	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/imesh/leaves"
	"github.com/AidosKuneen/aknode/msg"
	"github.com/AidosKuneen/aknode/setting"
)

var (
	ch     = make(chan struct{}, 1)
	notify chan []*imesh.HashWithType
)

//Resolve run resolve routine.
func Resolve() {
	if len(ch) == 0 {
		ch <- struct{}{}
	}
}

//RegisterNotifier registers a notifier for resolved txs.
func RegisterNotifier(n chan []*imesh.HashWithType) {
	notify = n
}

func goResolve(s *setting.Setting) {
	for range ch {
		log.Println("resolving unresolved transactions...")
		trs, err2 := imesh.Resolve(s)
		if err2 != nil {
			log.Println(err2)
			continue
		}
		for _, tr := range trs {
			log.Println("resolved txid:", hex.EncodeToString(tr.Hash))
		}
		if len(trs) != 0 {
			log.Println(" broadcasting resolved txs...")
			inv := make(msg.Inventories, 0, len(trs))
			for _, h := range trs {
				typ, err3 := msg.TxType2InvType(h.Type)
				if err3 != nil {
					log.Println(err3)
					continue
				}
				inv = append(inv, &msg.Inventory{
					Type: typ,
					Hash: h.Hash.Array(),
				})
				if (h.Type == tx.TxRewardFee && s.RunFeeMiner) ||
					(h.Type == tx.TxRewardTicket && s.RunTicketMiner) {
					addForMine(s, h)
				}
			}
			WriteAll(s, inv, msg.CmdInv)
		}

		ts, err2 := imesh.GetSearchingTx(s)
		if err2 != nil {
			log.Println(err2)
			continue
		}
		if len(ts) != 0 {
			log.Println("querying non-existent", len(ts), "transactions...")
			inv := make(msg.Inventories, 0, len(ts))
			for _, tr := range ts {
				typ, err2 := msg.TxType2InvType(tr.Type)
				if err2 != nil {
					log.Println(err2)
					continue
				}
				inv = append(inv, &msg.Inventory{
					Type: typ,
					Hash: tr.Hash.Array(),
				})
			}
			writeGetData(s, inv)
		}
		if notify != nil {
			notify <- trs
		}
		//wait to collect noexsistence txs
		time.Sleep(5 * time.Second)
	}
}

//GoCron starts cron jobs.
func goCron(s *setting.Setting) {
	go goResolve(s)

	go func() {
		for {
			time.Sleep(10 * time.Minute)
			log.Println("querying latest leaves and node addressses..")
			WriteAll(s, nil, msg.CmdGetLeaves)
			WriteAll(s, nil, msg.CmdGetAddr)
			peers.RLock()
			log.Println("#node", len(peers.Peers))
			peers.RUnlock()
			log.Println("#leaves", leaves.Size())
			log.Println("done")
		}
	}()
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			Resolve()
		}
	}()
}
