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
	"bytes"
	"fmt"
	"log"
	"net"
	"sort"
	"time"

	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/akconsensus"
	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/imesh/leaves"
	"github.com/AidosKuneen/aknode/msg"
	"github.com/AidosKuneen/aknode/setting"
)

func (p *peer) runLoop(s *setting.Setting) error {
	defer p.delete()
	for {
		var cmd byte
		var buf []byte
		var err2 error
		if err := setReadDeadline(p, time.Now().Add(connectionTimeout)); err != nil {
			return err
		}
		cmd, buf, err2 = msg.ReadHeader(s, p.conn)
		if err2 != nil {
			if ne, ok := err2.(net.Error); ok && ne.Timeout() {
				if i := p.isWritten(msg.CmdPing, nil); i >= 0 {
					if err3 := remove(s, p.remote); err3 != nil {
						log.Println(err3)
					}
					return fmt.Errorf("no response from %v", p.remote)
				}
				n := nonce()
				if err := p.write(s, &n, msg.CmdPing); err != nil {
					return err
				}
				continue
			}
			return err2
		}
		log.Println("read packet cmd", cmd)
		switch cmd {
		case msg.CmdPing:
			v, err := msg.ReadNonce(buf)
			if err != nil {
				return err
			}
			if err := p.write(s, v, msg.CmdPong); err != nil {
				log.Println(err)
				continue
			}

		case msg.CmdPong:
			v, err := msg.ReadNonce(buf)
			if err != nil {
				return err
			}
			if err := p.received(msg.CmdPing, v[:]); err != nil {
				return err
			}

		case msg.CmdGetAddr:
			adrs := get(msg.MaxAddrs)
			if err := p.write(s, adrs, msg.CmdAddr); err != nil {
				log.Println(err)
				return nil
			}

		case msg.CmdAddr:
			if err := p.received(msg.CmdGetAddr, nil); err != nil {
				return err
			}
			v, err := msg.ReadAddrs(s, buf)
			if err != nil {
				return err
			}
			if err := putAddrs(s, *v...); err != nil {
				log.Println(err)
				continue
			}

		case msg.CmdInv:
			invs, err := msg.ReadInventories(buf)
			if err != nil {
				return err
			}
			for _, inv := range invs {
				typ, err := inv.Type.ToTxType()
				if err != nil {
					log.Println(err)
					continue
				}
				if err := imesh.AddNoexistTxHash(s, inv.Hash[:], typ); err != nil {
					log.Println(err)
					continue
				}
			}
			Resolve()

		case msg.CmdGetData:
			invs, err := msg.ReadInventories(buf)
			if err != nil {
				return err
			}
			trs := make(msg.Txs, 0, len(invs))
			for _, inv := range invs {
				switch inv.Type {
				case msg.InvTxNormal:
					tr, err := imesh.GetTx(s.DB, inv.Hash[:])
					if err != nil {
						log.Println(err)
						continue
					}
					trs = append(trs, &msg.Tx{
						Type: inv.Type,
						Tx:   tr,
					})
				case msg.InvTxRewardFee:
					fallthrough
				case msg.InvTxRewardTicket:
					typ := tx.TypeRewardFee
					if inv.Type == msg.InvTxRewardTicket {
						typ = tx.TypeRewardTicket
					}
					tr, err := imesh.GetMinableTx(s, inv.Hash[:], typ)
					if err != nil {
						log.Println(err)
						continue
					}
					trs = append(trs, &msg.Tx{
						Type: inv.Type,
						Tx:   tr,
					})
				default:
					return fmt.Errorf("unknown inv type %v", inv.Type)
				}
			}
			if len(trs) == 0 {
				continue
			}
			if err := p.write(s, trs, msg.CmdTxs); err != nil {
				log.Println(err)
				return nil
			}
			Resolve()

		case msg.CmdTxs:
			vs, err := msg.ReadTxs(buf)
			if err != nil {
				return err
			}
			for _, v := range vs {
				typ, err := v.Type.ToTxType()
				if err != nil {
					return err
				}
				if err := v.Tx.Check(s.Config, typ); err != nil {
					return err
				}
				if err := imesh.CheckAddTx(s, v.Tx, typ); err != nil {
					log.Println(err)
				}
			}
			Resolve()

		case msg.CmdGetLeaves:
			v, err := msg.ReadLeavesFrom(buf)
			if err != nil {
				return err
			}
			ls := leaves.GetAll()
			idx := sort.Search(len(ls), func(i int) bool {
				return bytes.Compare(ls[i], v[:]) >= 0
			})
			h := make(msg.Inventories, 0, len(ls)-idx)
			for i := idx; i < len(ls) && i < msg.MaxLeaves; i++ {
				h = append(h, &msg.Inventory{
					Type: msg.InvTxNormal,
					Hash: ls[i].Array(),
				})
			}
			if err := p.write(s, h, msg.CmdLeaves); err != nil {
				log.Println(err)
				return nil
			}

		case msg.CmdLeaves:
			v, err := msg.ReadInventories(buf)
			if err != nil {
				return err
			}
			if err := p.received(msg.CmdGetLeaves, nil); err != nil {
				return err
			}
			for _, h := range v {
				if h.Type != msg.InvTxNormal {
					return fmt.Errorf("invalid inventory type %v", h.Type)
				}
				if err := imesh.AddNoexistTxHash(s, h.Hash[:], tx.TypeNormal); err != nil {
					log.Println(err)
				}
			}
			if len(v) == msg.MaxLeaves {
				gl := v[len(v)-1].Hash
				WriteAll(s, &gl, msg.CmdGetLeaves)
			}
			Resolve()

		case msg.CmdClose:
			return nil

		case msg.CmdGetLedger:
			v, err := akconsensus.ReadGetLeadger(buf)
			if err != nil {
				return err
			}
			l, err := akconsensus.GetLedger(s, v)
			if err != nil {
				log.Println(err)
				continue
			}
			if err := p.write(s, l, msg.CmdLedger); err != nil {
				log.Println(err)
				continue
			}
		case msg.CmdLedger:
			v, err := akconsensus.ReadLeadger(s, peers.cons, buf)
			if err != nil {
				return err
			}
			id := v.ID()
			if err := p.received(msg.CmdGetLedger, id[:]); err != nil {
				return err
			}
			if err := akconsensus.PutLedger(s, v); err != nil {
				return err
			}
		case msg.CmdValidation:
			v, noexist, err := akconsensus.ReadValidation(s, peers.cons, buf)
			if err != nil {
				return err
			}
			if noexist {
				WriteAll(s, v, msg.CmdValidation)
			}

		case msg.CmdProposal:
			v, noexist, err := akconsensus.ReadProposal(s, peers.cons, buf)
			if err != nil {
				log.Println(err)
				return err
			}
			if noexist {
				WriteAll(s, v, msg.CmdProposal)
			}

		default:
			return fmt.Errorf("invalid cmd %d", cmd)
		}
	}
}
