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

	akrand "github.com/AidosKuneen/aklib/rand"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/msg"
	"github.com/AidosKuneen/aknode/setting"
	"github.com/AidosKuneen/consensus"
)

const (
	connectionTimeout = 10 * time.Minute
	rwTimeout         = 30 * time.Second
	//BanTime is time to be banned
	BanTime = time.Hour
)

//peers is a slice of connecting peers.
var peers = struct {
	Peers  map[string]*peer
	banned map[string]time.Time
	cons   *consensus.Peer
	sync.RWMutex
}{
	Peers:  make(map[string]*peer),
	banned: make(map[string]time.Time),
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
	peers.RLock()
	defer peers.RUnlock()
	r := make(map[string]time.Time)
	for k, v := range peers.banned {
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

//ConsensusPeer is for consensus to communicate with peers.
type ConsensusPeer struct{}

//GetLedger get a ledger with id.
func (cp *ConsensusPeer) GetLedger(s *setting.Setting, id consensus.LedgerID) {
	WriteAll(s, &id, msg.CmdGetLedger)
}

//BroadcastProposal broadcast our proposal.
func (cp *ConsensusPeer) BroadcastProposal(s *setting.Setting, p *consensus.Proposal) {
	WriteAll(s, p, msg.CmdProposal)
}

//BroadcastValidatoin broadcast our validation.
func (cp *ConsensusPeer) BroadcastValidatoin(s *setting.Setting, v *consensus.Validation) {
	WriteAll(s, v, msg.CmdValidation)
}

//GetTx get a tx with hash h.
func (cp *ConsensusPeer) GetTx(s *setting.Setting, h tx.Hash) {
	if err := imesh.AddNoexistTxHash(s, h, tx.TypeNormal); err != nil {
		log.Println(err)
	}
}

//newPeer returns Peer struct.
//locked
func newPeer(v *msg.Version, conn *net.TCPConn, s *setting.Setting) (*peer, error) {
	remote := conn.RemoteAddr().(*net.TCPAddr).IP.String()
	if s.InBlacklist(remote) {
		return nil, errors.New("remote is in blacklist")
	}
	err2 := func() error {
		peers.Lock()
		defer peers.Unlock()
		if t, exist := peers.banned[remote]; exist {
			if t.Add(BanTime).Before(time.Now()) {
				delete(peers.banned, remote)
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
	if len(peers.Peers) == 0 {
		log.Println("no peers to writegetdata")
		return
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
	log.Println("writing packet cmd", cmd)

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
	case msg.CmdGetLedger:
		n, ok := m.(*consensus.LedgerID)
		if !ok {
			return errors.New("invalid data")
		}
		w.data = n[:]
		p.written = append(p.written, w)
	}
	if err := p.conn.SetWriteDeadline(time.Now().Add(rwTimeout)); err != nil {
		return err
	}
	log.Println("writing", cmd, p.remote)
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
//悪・即・バン
func (p *peer) run(s *setting.Setting) {
	if err := p.runLoop(s); err != nil {
		log.Println(err)
		peers.Lock()
		h, _, err2 := net.SplitHostPort(p.remote.Address)
		if err2 != nil {
			peers.banned[p.remote.Address] = time.Now()
		} else {
			peers.banned[h] = time.Now()
		}
		peers.Unlock()
		if err3 := remove(s, p.remote); err3 != nil {
			log.Println(err3)
		}
	}
}

//for test
var setReadDeadline = func(p *peer, t time.Time) error {
	return p.conn.SetReadDeadline(t)
}
