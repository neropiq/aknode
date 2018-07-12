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
	connectionTimeout = 10 * time.Minute
	rwTimeout         = 30 * time.Second
	//BanTime is time to be banned
	BanTime = time.Hour
)

//peers is a slice of connecting peers.
var peers = struct {
	Peers map[string]*peer
	sync.RWMutex
}{
	Peers: make(map[string]*peer),
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

//peer represetnts an opponent of a connection.
type peer struct {
	conn    *net.TCPConn
	remote  msg.Addr
	written []wdata
	sync.RWMutex
}

//GetBanned returns a banned list.
func GetBanned() map[string]time.Time {
	banned.RLock()
	defer banned.RUnlock()
	r := make(map[string]time.Time)
	for k, v := range banned.addr {
		r[k] = v
	}
	return r
}

//GetPeerlist returns a peer list.
func GetPeerlist() []msg.Addr {
	peers.RLock()
	defer peers.RUnlock()
	r := make([]msg.Addr, len(peers.Peers))
	i := 0
	for _, p := range peers.Peers {
		r[i] = p.remote
		i++
	}
	return r
}

//newPeer returns Peer struct.
func newPeer(v *msg.Version, conn *net.TCPConn, s *setting.Setting) (*peer, error) {
	remote := conn.RemoteAddr().(*net.TCPAddr).IP.String()
	if s.InBlacklist(remote) {
		return nil, errors.New("remote is in blacklist")
	}
	err2 := func() error {
		banned.Lock()
		defer banned.Unlock()
		if t, exist := banned.addr[remote]; exist {
			if t.Add(BanTime).Before(time.Now()) {
				delete(banned.addr, remote)
			} else {
				return errors.New("the remote node is banned now")
			}
		}
		return nil
	}()
	if err2 != nil {
		return nil, err2
	}
	if s.InBlacklist(v.AddrFrom.Address) {
		return nil, errors.New("remote is in blacklist")
	}
	if !s.UseTor {
		h, po, err2 := net.SplitHostPort(v.AddrFrom.Address)
		if err2 != nil {
			return nil, err2
		}
		if h == "" {
			v.AddrFrom.Address = net.JoinHostPort(remote, po)
		}
	}
	p := &peer{
		conn:   conn,
		remote: v.AddrFrom,
	}
	peers.RLock()
	defer peers.RUnlock()
	if len(peers.Peers) >= int(s.MaxConnections)*10 {
		return nil, errors.New("peers are too much")
	}
	if _, exist := peers.Peers[p.remote.Address]; exist {
		return nil, errors.New("already connected")
	}
	return p, nil
}

//Add adds to the Peer list.
func (p *peer) add(s *setting.Setting) error {
	peers.Lock()
	defer peers.Unlock()
	if len(peers.Peers) >= int(s.MaxConnections)*2 {
		return errors.New("peers is too big")
	}
	if _, exist := peers.Peers[p.remote.Address]; exist {
		return errors.New("already connected")
	}
	peers.Peers[p.remote.Address] = p

	return nil
}

func (p *peer) delete() {
	peers.Lock()
	defer peers.Unlock()
	delete(peers.Peers, p.remote.Address)
}

func isConnected(adr string) bool {
	_, exist := peers.Peers[adr]
	return exist
}

//ConnSize returns number of connection peers.
func ConnSize() int {
	peers.RLock()
	defer peers.RUnlock()
	return len(peers.Peers)
}

//WriteAll writes a command to all connected peers.
func WriteAll(s *setting.Setting, m interface{}, cmd byte) {
	peers.RLock()
	defer peers.RUnlock()
	for _, p := range peers.Peers {
		if err := p.write(s, m, cmd); err != nil {
			log.Println(err)
		}
	}
}

//WriteGetData writes a get_data command to all connected peers.
func writeGetData(s *setting.Setting, invs msg.Inventories) {
	peers.RLock()
	defer peers.RUnlock()
	for i := len(invs) - 1; i >= 0; i-- {
		j := akrand.R.Intn(i + 1)
		invs[i], invs[j] = invs[j], invs[i]
	}
	n := 2 * len(invs) / len(peers.Peers)
	if n*len(peers.Peers) != 2*len(invs) {
		n++
	}
	no := 0
	for _, p := range peers.Peers {
		winvs := make(msg.Inventories, 0, n)
		start := no
		for i := 0; i < n; i++ {
			winvs = append(winvs, invs[no])
			if no++; no >= len(invs) {
				no = 0
			}
			if start == no {
				break
			}
		}
		if err := p.write(s, winvs, msg.CmdGetData); err != nil {
			log.Println(err)
		}
	}
}

//Write writes a packet to peer p.
func (p *peer) write(s *setting.Setting, m interface{}, cmd byte) error {
	p.Lock()
	defer p.Unlock()
	w := wdata{
		cmd:  cmd,
		time: time.Now(),
	}
	switch cmd {
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
	return msg.Write(s, m, cmd, p.conn)
}

func (p *peer) isWritten(cmd byte, data []byte) int {
	p.RLock()
	defer p.RUnlock()
	for i, c := range p.written {
		if c.cmd == cmd && (data == nil || bytes.Equal(c.data, data)) {
			return i
		}
	}
	return -1
}

func (p *peer) received(cmd byte, data []byte) error {
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
func (p *peer) run(s *setting.Setting) {
	if err := p.runLoop(s); err != nil {
		log.Println(err)
		banned.Lock()
		h, _, err2 := net.SplitHostPort(p.remote.Address)
		if err2 != nil {
			banned.addr[p.remote.Address] = time.Now()
		} else {
			banned.addr[h] = time.Now()
		}
		banned.Unlock()
		if err3 := remove(s, p.remote); err3 != nil {
			log.Println(err3)
		}
	}
}

//for test
var setReadDeadline = func(p *peer, t time.Time) error {
	return p.conn.SetReadDeadline(t)
}

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
					fallthrough
				case msg.InvTxRewardTicket:
					typ := tx.TxRewardFee
					if inv.Type == msg.InvTxRewardTicket {
						typ = tx.TxRewardTicket
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
				if err := imesh.AddNoexistTxHash(s, h.Hash[:], tx.TxNormal); err != nil {
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

		default:
			return fmt.Errorf("invalid cmd %d", cmd)
		}
	}
}
