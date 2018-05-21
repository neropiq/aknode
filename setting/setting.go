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

package setting

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/db"
	"github.com/dgraph-io/badger"
)

//Network types
const (
	NetworkIP byte = iota
	NetworkTor
)

//Setting is  a aknode setting.
type Setting struct {
	Debug        bool     `json:"debug"`
	Testnet      byte     `json:"testnet"`
	BlacklistIPs []string `json:"blacklist_ips"`
	NetworkType  byte     `json:"network_type"`

	MyHostName     string   `json:"my_ip"`
	DefaultNodes   []string `json:"default_nodes"`
	Bind           string   `json:"bind"`
	Port           uint16   `json:"port"`
	MaxConnections uint16   `json:"max_connections"`
	Proxy          string   `json:"proxy"`

	RunRPCServer      bool     `json:"run_rpc_server"`
	RPCBind           string   `json:"rpc_bind"`
	RPCPort           uint16   `json:"rpc_port"`
	RPCUser           string   `json:"rpc_user"`
	RPCPassword       string   `json:"rpc_password"`
	RPCAllowIPs       []string `json:"rpc_allow_ips"`
	RPCMaxConnections uint16   `json:"rpc_max_connections"`
	RPCProxy          string   `json:"rpc_proxy"`

	RunValidator bool     `json:"run_validator"`
	Validators   []string `json:"validtors"`

	DB     *badger.DB    `json:"-"`
	Config *aklib.Config `json:"-"`
}

//Init parse a json file fname , open DB and returns Settings struct .
func Init(fname string) *Setting {
	s, err := ioutil.ReadFile(fname)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	var se Setting
	if err := json.Unmarshal(s, &se); err != nil {
		log.Println(err)
		os.Exit(1)
	}
	if se.NetworkType != NetworkIP {
		log.Println("invalid network type")
		os.Exit(1)
	}

	if int(se.Testnet) > len(aklib.Configs) {
		log.Println("testnet must be 0(mainnet) or 1")
		os.Exit(1)
	}
	se.Config = aklib.Configs[se.Testnet]
	net := aklib.Configs[se.Testnet]
	if se.Bind == "" {
		se.Bind = "0.0.0.0"
	}
	if se.Port == 0 {
		se.Port = net.DefaultPort
	}
	if se.MaxConnections == 0 {
		se.MaxConnections = 10
	}

	if se.RPCBind == "" {
		se.RPCBind = "localhost"
	}
	if se.RPCPort == 0 {
		se.RPCPort = net.DefaultRPCPort
	}

	//need extra checks for quorum

	se.DB, err = db.Open("db")
	if err != nil {
		panic(err)
	}
	return &se
}
