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
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"net"
	"sort"
	"sync"
	"time"

	akrand "github.com/AidosKuneen/aklib/rand"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/imesh/leaves"

	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/msg"
	"github.com/AidosKuneen/aknode/setting"
)

const (
	connectionTimeout = 20 * time.Minute
	rwTimeout         = time.Minute
)

//peers is a slice of connecting peers.
var peers = struct {
	Peers map[*Peer]struct{}
	sync.RWMutex
}{
	Peers: make(map[*Peer]struct{}),
}

var banned = struct {
	addr map[string]time.Time
	sync.RWMutex
}{
	addr: make(map[string]time.Time),
}

type wdata struct {
	cmd  byte
	data []byte
	time time.Time
}

//Peer represetnts an opponent of a connection.
type Peer struct {
	*msg.Version
	conn    *net.TCPConn
	from    msg.Addr
	setting *setting.Setting
	written []wdata
	sync.RWMutex
}

//NewPeer returns Peer struct.
func NewPeer(v *msg.Version, conn *net.TCPConn, s *setting.Setting) (*Peer, error) {
	remote := conn.RemoteAddr().String()
	if s.InBlacklist(remote) {
		return nil, errors.New("remote is in blacklist")
	}
	err := func() error {
		banned.Lock()
		defer banned.Unlock()
		if t, exist := banned.addr[remote]; exist {
			if t.Add(time.Hour).After(time.Now()) {
				delete(banned.addr, remote)
			} else {
				return errors.New("the remote node is banned now")
			}
		}
		return nil
	}()
	if err != nil {
		return nil, err
	}
	if v.AddrFrom.Address != "" && s.InBlacklist(v.AddrFrom.Address) {
		return nil, errors.New("remote is in blacklist")
	}
	p := &Peer{
		Version: v,
		conn:    conn,
		setting: s,
		from: msg.Addr{
			Address: conn.RemoteAddr().String(),
			Port:    v.AddrFrom.Port,
		},
	}
	if v.AddrFrom.Address != "" {
		p.from.Address = v.AddrFrom.Address
	}
	peers.RLock()
	defer peers.RUnlock()
	if len(peers.Peers) >= int(s.MaxConnections)*10 {
		return nil, errors.New("peers is too big")
	}
	return p, nil
}

//Add adds to the Peer list.
func (p *Peer) Add() error {
	peers.Lock()
	defer peers.Unlock()
	if len(peers.Peers) >= int(p.setting.MaxConnections)*2 {
		return errors.New("peers is too big")
	}
	peers.Peers[p] = struct{}{}
	return nil
}

//PeerNum returns number of peers
func PeerNum() int {
	peers.RLock()
	defer peers.RUnlock()
	return len(peers.Peers)
}

//Close closes a connection to a peer.
func (p *Peer) Close() {
	if err := p.conn.Close(); err != nil {
		log.Println(err)
	}
	peers.Lock()
	defer peers.Unlock()
	delete(peers.Peers, p)
}

//WriteAll writes a command to all connected peers.
func WriteAll(m interface{}, cmd byte) {
	peers.RLock()
	defer peers.RUnlock()
	for p := range peers.Peers {
		if err := p.Write(m, cmd); err != nil {
			log.Println(err)
		}
	}
}

//WriteGetData writes a get_data command to all connected peers.
func WriteGetData(invs msg.Inventories) {
	peers.RLock()
	defer peers.RUnlock()
	for i := len(invs) - 1; i >= 0; i-- {
		j := akrand.R.Intn(i + 1)
		invs[i], invs[j] = invs[j], invs[i]
	}
	n := 2 * len(invs) / len(peers.Peers)
	if n == 0 {
		n = 1
	}
	i := 0
	for p := range peers.Peers {
		from := ((i / 2) * n) % len(invs)
		to := (((i / 2) + 1) * n) % (len(invs) + 1)
		if err := p.Write(invs[from:to], msg.CmdGetData); err != nil {
			log.Println(err)
		}
		i++
	}
}

//Write writes a packet to peer p.
func (p *Peer) Write(m interface{}, cmd byte) error {
	p.Lock()
	defer p.Unlock()
	w := wdata{
		cmd:  cmd,
		time: time.Now(),
	}
	switch cmd {
	case msg.CmdVersion:
		fallthrough
	case msg.CmdGetLeaves:
		fallthrough
	case msg.CmdGetAddr:
		p.written = append(p.written, w)
	case msg.CmdPing:
		n, ok := m.(*msg.Nonce)
		if !ok {
			return errors.New("invalid data")
		}
		w.data = n[:]
		p.written = append(p.written, w)
	}
	if err := p.conn.SetWriteDeadline(time.Now().Add(rwTimeout)); err != nil {
		return err
	}
	return msg.Write(p.setting, m, cmd, p.conn)
}

func (p *Peer) isWritten(cmd byte, data []byte) int {
	p.RLock()
	defer p.RUnlock()
	for i, c := range p.written {
		if c.cmd == cmd && (data == nil || bytes.Equal(c.data, data)) {
			return i
		}
	}
	return -1
}

func (p *Peer) received(cmd byte, data []byte) error {
	i := p.isWritten(cmd, data)
	if i < 0 {
		return fmt.Errorf("no command for %v", cmd)
	}
	p.Lock()
	defer p.Unlock()
	p.written = append(p.written[:i], p.written[i+1:]...)
	return nil
}

func nonce() msg.Nonce {
	var n [32]byte
	_, err := rand.Read(n[:])
	if err != nil {
		log.Fatal(err)
	}
	return n
}

//Run runs a rouintine for a peer.
func (p *Peer) Run(s *setting.Setting) {
	if err := p.run(s); err != nil {
		log.Println(err)
		banned.Lock()
		banned.addr[p.AddrFrom.Address] = time.Now()
		banned.Unlock()
		if err := Remove(s, p.AddrFrom); err != nil {
			log.Println(err)
		}
	}
}

func (p *Peer) run(s *setting.Setting) error {
	defer p.Close()
	for {
		var cmd byte
		var buf []byte
		var err error
		if err := p.conn.SetReadDeadline(time.Now().Add(connectionTimeout)); err != nil {
			return err
		}
		cmd, buf, err = msg.ReadHeader(p.setting, p.conn)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				if p.isWritten(msg.CmdPing, nil) >= 0 {
					err := fmt.Errorf("no response from %v", p.from)
					if err := Remove(s, p.from); err != nil {
						log.Println(err)
					}
					return err
				}
				n := nonce()
				if err := p.Write(&n, msg.CmdPing); err != nil {
					return err
				}
				continue
			}
			return err
		}

		switch cmd {
		case msg.CmdPing:
			v, err := msg.ReadNonce(buf)
			if err != nil {
				return err
			}
			if err := p.Write(v, msg.CmdPong); err != nil {
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
			adrs := Get(msg.MaxAddrs)
			if err := p.Write(adrs, msg.CmdAddr); err != nil {
				log.Println(err)
				return nil
			}

		case msg.CmdAddr:
			if err := p.received(msg.CmdGetAddr, nil); err != nil {
				return err
			}
			v, err := msg.ReadAddrs(buf)
			if err != nil {
				return err
			}
			if err := Put(s, *v); err != nil {
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
			resolve()

		case msg.CmdGetData:
			invs, err := msg.ReadInventories(buf)
			if err != nil {
				return err
			}
			trs := make(msg.Txs, 0, len(invs))
			for _, inv := range invs {
				switch inv.Type {
				case msg.InvTxNormal:
					tr, err := imesh.GetTx(s, inv.Hash[:])
					if err != nil {
						log.Println(err)
						continue
					}
					trs = append(trs, &msg.Tx{
						Type: inv.Type,
						Tx:   tr,
					})
				case msg.InvTxRewardFee:
					tr, err := imesh.GetMinableTx(s, inv.Hash[:], tx.TxRewardFee)
					if err != nil {
						log.Println(err)
						continue
					}
					trs = append(trs, &msg.Tx{
						Type: inv.Type,
						Tx:   tr,
					})
				case msg.InvTxRewardTicket:
					tr, err := imesh.GetMinableTx(s, inv.Hash[:], tx.TxRewardTicket)
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
			if err := p.Write(trs, msg.CmdTxs); err != nil {
				log.Println(err)
				return nil
			}
			resolve()

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
			resolve()

		case msg.CmdGetLeaves:
			v, err := msg.ReadGetLeaves(buf)
			if err != nil {
				return err
			}
			ls := leaves.GetAll()
			idx := sort.Search(len(ls), func(i int) bool {
				return bytes.Compare(ls[i], v.From[:]) >= 0
			})
			h := make(msg.Hashes, 0, len(ls)-idx)
			for i := idx; i < len(ls) && i < msg.MaxLeaves; i++ {
				h = append(h, ls[i].Array())
			}
			if err := p.Write(h, msg.CmdLeaves); err != nil {
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
				if err := imesh.AddNoexistTxHash(s, h.Hash[:], tx.TxNormal); err != nil {
					log.Println(err)
				}
			}
			if len(v) == msg.MaxLeaves {
				gl := &msg.GetLeaves{
					From: v[len(v)-1].Hash,
				}
				WriteAll(&gl, msg.CmdGetLeaves)
			}
			resolve()

		case msg.CmdClose:
			return nil

		default:
			return fmt.Errorf("invalid cmd %d", cmd)
		}
	}
}
