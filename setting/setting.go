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
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"

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
	Debug       bool     `json:"debug"`
	Testnet     byte     `json:"testnet"`
	Blacklists  []string `json:"blacklist_ips"`
	NetworkType byte     `json:"network_type"`
	RootDir     string   `json:"root_dir"`

	MyHostName     string   `json:"my_ip"`
	DefaultNodes   []string `json:"default_nodes"`
	Bind           string   `json:"bind"`
	Port           uint16   `json:"port"`
	MaxConnections uint16   `json:"max_connections"`
	Proxy          string   `json:"proxy"`

	RunRPCServer      bool   `json:"run_rpc"`
	RPCType           byte   `json:"rpc_type"`
	RPCBind           string `json:"rpc_bind"`
	RPCPort           uint16 `json:"rpc_port"`
	RPCUser           string `json:"rpc_user"`
	RPCPassword       string `json:"rpc_password"`
	RPCMaxConnections uint16 `json:"rpc_max_connections"`
	RPCProxy          string `json:"rpc_proxy"`

	RunValidator bool `json:"run_validator"`
	//Validators   []string `json:"validtors"`

	RunExplorer            bool   `json:"run_explore"`
	ExplorerBind           string `json:"explorer_bind"`
	ExplorerPort           uint16 `json:"explorer_port"`
	ExplorerMaxConnections uint16 `json:"explorer_max_connections"`
	ExplorerProxy          string `json:"explorer_proxy"`

	DB     *badger.DB    `json:"-"`
	Config *aklib.Config `json:"-"`
}

//Load parse a json file fname , open DB and returns Settings struct .
func Load(fname string) *Setting {
	var se Setting
	var err error
	if fname != "" {
		s, err := ioutil.ReadFile(fname)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		if err := json.Unmarshal(s, &se); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
	if int(se.Testnet) > len(aklib.Configs) {
		fmt.Println("testnet must be 0(mainnet) or 1")
		os.Exit(1)
	}
	net := aklib.Configs[se.Testnet]
	if se.NetworkType != NetworkIP {
		fmt.Println("invalid network type")
		os.Exit(1)
	}
	usr, err := user.Current()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if se.RootDir == "" {
		se.RootDir = filepath.Join(usr.HomeDir, ".aknode")
	}

	if se.Bind == "" {
		se.Bind = "0.0.0.0"
	}
	if se.Port == 0 {
		se.Port = net.DefaultPort
	}
	if se.MaxConnections == 0 {
		se.MaxConnections = 10
	}
	if se.RPCType != NetworkIP && se.RPCType != NetworkTor {
		fmt.Println("invalid RPC type")
		os.Exit(1)
	}
	if se.RPCBind == "" {
		se.RPCBind = "localhost"
	}
	if se.RPCPort == 0 {
		se.RPCPort = net.DefaultRPCPort
	}
	if se.RunRPCServer && (se.RPCUser == "" || se.RPCPassword == "") {
		fmt.Println("You must specify rpc_user and rpc_password")
		os.Exit(1)
	}
	if se.RPCMaxConnections == 0 {
		se.MaxConnections = 1
	}

	if se.ExplorerBind == "" {
		se.RPCBind = "localhost"
	}
	if se.ExplorerPort == 0 {
		se.RPCPort = net.DefaultExplorerPort
	}
	if se.ExplorerMaxConnections == 0 {
		se.ExplorerMaxConnections = 1
	}

	dbDir := filepath.Join(se.BaseDir(), "db")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	se.DB, err = db.Open(dbDir)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if err := os.MkdirAll(se.BaseDir(), 0755); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	se.Config = aklib.Configs[se.Testnet]
	return &se
}

//BaseDir returns a dir which contains a setting file and db dir.
func (s *Setting) BaseDir() string {
	return filepath.Join(s.RootDir, s.Config.Name)
}

//InBlacklist returns true if remote is in blacklist.
func (s *Setting) InBlacklist(remote string) bool {
	for _, b := range s.Blacklists {
		if remote == b {
			return true
		}
	}
	return false
}
