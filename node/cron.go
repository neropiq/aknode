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
	"time"

	"github.com/AidosKuneen/aknode/imesh"

	"github.com/AidosKuneen/aknode/msg"
	"github.com/AidosKuneen/aknode/setting"
)

func searchTx(s *setting.Setting) error {
	trs, err := imesh.GetSearchingTx(s)
	if err != nil {
		return err
	}
	inv := make(msg.Inventories, 0, len(trs))
	for _, tr := range trs {
		typ := msg.InvTx
		if tr.Minable {
			typ = msg.InvMinableTx
		}
		inv = append(inv, &msg.Inventory{
			Type: typ,
			Hash: tr.Hash.Array(),
		})
	}
	WriteAll(inv, msg.CmdGetData)
	return nil
}

func resolve(s *setting.Setting) error {
	txH, minableH, err := imesh.Resolve(s)
	if err != nil {
		return err
	}
	inv := make(msg.Inventories, 0, len(txH)+len(minableH))
	for _, h := range txH {
		inv = append(inv, &msg.Inventory{
			Type: msg.InvTx,
			Hash: h.Array(),
		})
	}
	for _, h := range minableH {
		inv = append(inv, &msg.Inventory{
			Type: msg.InvMinableTx,
			Hash: h.Array(),
		})
	}
	WriteAll(inv, msg.CmdInv)
	return nil
}

//GoCron starts cron jobs.
func GoCron(s *setting.Setting) {
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			WriteAll(nil, msg.CmdGetLeaves)
			WriteAll(nil, msg.CmdGetAddr)
		}
	}()
	go func() {
		for {
			time.Sleep(time.Minute)
			if err := resolve(s); err != nil {
				log.Println(err)
			}
			if err := searchTx(s); err != nil {
				log.Println(err)
			}
		}
	}()
}
