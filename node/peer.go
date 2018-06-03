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
	"sync"
	"time"

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
	Peers []*Peer
	sync.RWMutex
}{}

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
	if s.InBlacklist(conn.RemoteAddr().String()) {
		return nil, errors.New("remote is in blacklist")
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
	peers.Peers = append(peers.Peers, p)
	return nil
}

//Close closes a connection to a peer.
func (p *Peer) Close() {
	if err := p.conn.Close(); err != nil {
		log.Println(err)
	}
	peers.Lock()
	defer peers.Unlock()
	for i, ps := range peers.Peers {
		if ps == p {
			copy(peers.Peers[i:], peers.Peers[i+1:])
			peers.Peers[len(peers.Peers)-1] = nil
			peers.Peers = peers.Peers[:len(peers.Peers)-1]
			break
		}
	}
}

//WriteAll writes a command to all connected peers.
func WriteAll(m interface{}, cmd byte) {
	peers.RLock()
	defer peers.RUnlock()
	for _, p := range peers.Peers {
		if err := p.Write(m, cmd); err != nil {
			log.Println(err)
		}
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
				switch inv.Type {
				case msg.InvTx:
					if err := imesh.AddTxHash(s, inv.Hash[:], false); err != nil {
						log.Println(err)
						continue
					}
				case msg.InvMinableTx:
					if err := imesh.AddTxHash(s, inv.Hash[:], true); err != nil {
						log.Println(err)
						continue
					}
				default:
					return fmt.Errorf("unknown inv type %v", inv.Type)
				}
			}
		case msg.CmdGetData:
			invs, err := msg.ReadInventories(buf)
			if err != nil {
				return err
			}
			trs := make(msg.Txs, 0, len(invs))
			for _, inv := range invs {
				switch inv.Type {
				case msg.InvTx:
					tx, err := imesh.GetTx(s, inv.Hash[:])
					if err != nil {
						log.Println(err)
						continue
					}
					trs = append(trs, tx)
				case msg.InvMinableTx:
					tx, err := imesh.GetMinableTx(s, inv.Hash[:])
					if err != nil {
						log.Println(err)
						continue
					}
					trs = append(trs, tx)
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
		case msg.CmdTxs:
			v, err := msg.ReadTxs(buf)
			if err != nil {
				return err
			}
			if err := imesh.CheckAddTx(s, v); err != nil {
				log.Println(err)
			}
		case msg.CmdGetLeaves:
			ls, err := leaves.Get(0)
			if err != nil {
				log.Println(err)
				return nil
			}
			h := make(msg.Hashes, 0, len(ls))
			for _, l := range ls {
				h = append(h, l.Array())
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
				if h.Type != msg.InvTx {
					return fmt.Errorf("invalid inventory type %v", h.Type)
				}
				if err := imesh.AddTxHash(s, h.Hash[:], false); err != nil {
					log.Println(err)
				}
			}
		case msg.CmdClose:
			return nil
		default:
			return fmt.Errorf("invalid cmd %d", cmd)
		}
	}
}
