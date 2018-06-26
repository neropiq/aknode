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
	"errors"
	"net"
	"sync"

	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aklib/rand"
	"github.com/AidosKuneen/aknode/msg"
	"github.com/AidosKuneen/aknode/setting"
	"github.com/dgraph-io/badger"
)

const maxAddrs = 1000

type adrmap map[string]uint16

var nodesDB = struct {
	Addrs adrmap
	sync.RWMutex
}{
	Addrs: make(adrmap),
}

func (a adrmap) msgAddr(key string) msg.Addr {
	return msg.Addr{
		Address: []byte(key),
		Port:    a[key],
	}
}

func (a adrmap) add(ip net.IP, port uint16) {
	a[string(ip.To16())] = port
}
func (a adrmap) delete(ip net.IP) {
	delete(a, string(ip.To16()))
}

//Init loads node IP addresses from DB.
func initDB(s *setting.Setting) error {
	nodesDB.Addrs = nil
	err := s.DB.View(func(txn *badger.Txn) error {
		return db.Get(txn, nil, &nodesDB.Addrs, db.HeaderNodeIP)
	})
	if err != nil && err != badger.ErrKeyNotFound {
		return err
	}
	for _, n := range s.DefaultNodeIPs {
		nodesDB.Addrs.add(n.IP, uint16(n.Port))
	}
	for adr := range nodesDB.Addrs {
		if s.InBlacklist(net.IP(adr)) {
			nodesDB.Addrs.delete(net.IP(adr))
		}
	}
	return nil
}

//Get returns random n numbers of nodes.
func get(n int) []msg.Addr {
	nodesDB.Lock()
	defer nodesDB.Unlock()
	r := make([]msg.Addr, len(nodesDB.Addrs))
	i := 0
	for a := range nodesDB.Addrs {
		r[i] = nodesDB.Addrs.msgAddr(a)
		i++
	}

	for i := n - 1; i >= 0; i-- {
		j := rand.R.Intn(i + 1)
		r[i], r[j] = r[j], r[i]
	}
	if n <= 0 {
		return r
	}
	if n < len(nodesDB.Addrs) {
		return r
	}
	return r[:n]
}

//Remove removes address from list.
func remove(s *setting.Setting, addr msg.Addr) error {
	nodesDB.Lock()
	defer nodesDB.Unlock()
	if _, e := nodesDB.Addrs[addr.Key()]; !e {
		return errors.New("not found")
	}
	nodesDB.Addrs.delete(addr.Address)
	return put(s)
}

//Put put an address into db.
func putAddrs(s *setting.Setting, addrs msg.Addrs) error {
	nodesDB.Lock()
	defer nodesDB.Unlock()
	if len(nodesDB.Addrs) > msg.MaxAddrs {
		return nil
	}
	for _, addr := range addrs {
		if s.InBlacklist(addr.Address) {
			continue
		}
		if len(nodesDB.Addrs) > maxAddrs {
			continue
		}
		if _, e := nodesDB.Addrs[addr.Key()]; !e {
			nodesDB.Addrs.add(addr.Address, addr.Port)
		}
	}
	return put(s)
}

func put(s *setting.Setting) error {
	return s.DB.Update(func(txn *badger.Txn) error {
		return db.Put(txn, nil, nodesDB.Addrs, db.HeaderNodeIP)
	})
}
