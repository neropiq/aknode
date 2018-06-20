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
	"fmt"
	"log"
	"sync"

	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aklib/rand"
	"github.com/AidosKuneen/aknode/msg"
	"github.com/AidosKuneen/aknode/setting"
	"github.com/dgraph-io/badger"
)

var nodesDB = struct {
	Addrs map[msg.Addr]struct{}
	sync.RWMutex
}{
	Addrs: make(map[msg.Addr]struct{}),
}

//Init loads node IP addresses from DB.
func Init(s *setting.Setting) {
	nodesDB.Addrs = nil
	err := s.DB.View(func(txn *badger.Txn) error {
		return db.Get(txn, nil, &nodesDB.Addrs, db.HeaderNodeIP)
	})
	if err != nil && err != badger.ErrKeyNotFound {
		fmt.Println(err)
		log.Fatal(err)
	}
}

//Size returns size of node addresses.
func Size() int {
	nodesDB.RLock()
	defer nodesDB.RUnlock()
	return len(nodesDB.Addrs)
}

//Get returns random n numbers of nodes.
func Get(n int) []msg.Addr {
	nodesDB.Lock()
	defer nodesDB.Unlock()
	r := make([]msg.Addr, len(nodesDB.Addrs))
	i := 0
	for a := range nodesDB.Addrs {
		r[i] = a
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
func Remove(s *setting.Setting, addr msg.Addr) error {
	nodesDB.Lock()
	defer nodesDB.Unlock()
	if _, e := nodesDB.Addrs[addr]; !e {
		return errors.New("not found")
	}
	delete(nodesDB.Addrs, addr)
	return put(s)
}

//Put put an address into db.
func Put(s *setting.Setting, addrs msg.Addrs) error {
	nodesDB.Lock()
	defer nodesDB.Unlock()
	if len(nodesDB.Addrs) > msg.MaxAddrs {
		return nil
	}
	for _, addr := range addrs {
		if s.InBlacklist(addr.Address) {
			continue
		}
		if len(nodesDB.Addrs) > msg.MaxAddrs {
			continue
		}
		if _, e := nodesDB.Addrs[addr]; !e {
			nodesDB.Addrs[addr] = struct{}{}
		}
	}
	return put(s)
}

func put(s *setting.Setting) error {
	return s.DB.Update(func(txn *badger.Txn) error {
		return db.Put(txn, nil, nodesDB.Addrs, db.HeaderNodeIP)
	})
}
