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
	"os"
	"testing"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/address"
	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aknode/imesh/leaves"
	"github.com/AidosKuneen/aknode/setting"
)

var s setting.Setting
var a *address.Address

func setup(t *testing.T) {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	var err error
	if err := os.RemoveAll("./test_db"); err != nil {
		log.Println(err)
	}
	s.DB, err = db.Open("./test_db")
	if err != nil {
		panic(err)
	}
	s.Config = aklib.DebugConfig
	seed := address.GenerateSeed()
	a, err = address.New(address.Height10, seed, s.Config)
	if err != nil {
		panic(err)
	}
	s.Config.Genesis = map[string]uint64{
		a.Address58(): aklib.ADKSupply,
	}
	leaves.Init(&s)
}

func teardown(t *testing.T) {
	if err := os.RemoveAll("./test_db"); err != nil {
		t.Error(err)
	}
}
func TestNode(t *testing.T) {
}
