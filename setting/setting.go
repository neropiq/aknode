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
	"errors"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/db"
	"github.com/dgraph-io/badger"
)

//Network types
const (
	NetworkIP byte = iota
	NetworkTor
)

//Address represents an address.
type Address struct {
	Address string `json:"address"`
	Port    uint16 `json:"port"`
}

//Setting is  a aknode setting.
type Setting struct {
	Debug        bool     `json:"debug"`
	Testnet      byte     `json:"testnet"`
	Blacklists   []string `json:"blacklists"`
	BlacklistIPs []net.IP `json:"-"`
	RootDir      string   `json:"root_dir"`

	MyHostName     string         `json:"my_host"`
	MyHostIP       net.IP         `json:"-"`
	DefaultNodes   []string       `json:"default_nodes"`
	DefaultNodeIPs []*net.TCPAddr `json:"-"`

	Bind           string `json:"bind"`
	Port           uint16 `json:"port"`
	MaxConnections uint16 `json:"max_connections"`
	Proxy          string `json:"proxy"`

	RunRPCServer      bool   `json:"run_rpc"`
	RPCType           byte   `json:"rpc_type"`
	RPCBind           string `json:"rpc_bind"`
	RPCPort           uint16 `json:"rpc_port"`
	RPCUser           string `json:"rpc_user"`
	RPCPassword       string `json:"rpc_password"`
	RPCMaxConnections uint16 `json:"rpc_max_connections"`

	RunValidator bool `json:"run_validator"`

	RunExplorer            bool   `json:"run_explore"`
	ExplorerBind           string `json:"explorer_bind"`
	ExplorerPort           uint16 `json:"explorer_port"`
	ExplorerMaxConnections uint16 `json:"explorer_max_connections"`

	RunFeeMiner    bool `json:"run_fee_miner"`
	RunTicketMiner bool `json:"run_ticket_miner"`

	DB     *badger.DB    `json:"-"`
	Config *aklib.Config `json:"-"`
}

//Load parse a json file fname , open DB and returns Settings struct .
func Load(s []byte) (*Setting, error) {
	var se Setting
	var err error

	if err := json.Unmarshal(s, &se); err != nil {
		return nil, err
	}
	if int(se.Testnet) > len(aklib.Configs) {
		return nil, errors.New("testnet must be 0(mainnet) or 1")
	}
	se.Config = aklib.Configs[se.Testnet]
	netconf := aklib.Configs[se.Testnet]

	se.BlacklistIPs, err = parseIPs(se.Blacklists...)
	if err != nil {
		return nil, err
	}
	se.DefaultNodeIPs, err = parseTCPAddrsses(se.DefaultNodes...)
	if err != nil {
		return nil, err
	}
	ips, err := parseIPs(se.MyHostName)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, errors.New("invalid my_host_name")
	}
	se.MyHostIP = ips[0]

	usr, err := user.Current()
	if err != nil {
		return nil, errors.New("testnet must be 0(mainnet) or 1")
	}
	if se.RootDir == "" {
		se.RootDir = filepath.Join(usr.HomeDir, ".aknode")
	}

	if se.Bind == "" {
		se.Bind = "0.0.0.0"
	}
	if se.Port == 0 {
		se.Port = netconf.DefaultPort
	}
	if se.MaxConnections == 0 {
		se.MaxConnections = 10
	}
	if se.RPCType != NetworkIP && se.RPCType != NetworkTor {
		return nil, errors.New("invalid RPC type")
	}
	if se.RPCBind == "" {
		se.RPCBind = "localhost"
	}
	if se.RPCPort == 0 {
		se.RPCPort = netconf.DefaultRPCPort
	}
	if se.RunRPCServer && (se.RPCUser == "" || se.RPCPassword == "") {
		return nil, errors.New("You must specify rpc_user and rpc_password")
	}
	if se.RPCMaxConnections == 0 {
		se.MaxConnections = 1
	}

	if se.ExplorerBind == "" {
		se.RPCBind = "localhost"
	}
	if se.ExplorerPort == 0 {
		se.RPCPort = netconf.DefaultExplorerPort
	}
	if se.ExplorerMaxConnections == 0 {
		se.ExplorerMaxConnections = 1
	}

	dbDir := filepath.Join(se.BaseDir(), "db")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, err
	}
	se.DB, err = db.Open(dbDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(se.BaseDir(), 0755); err != nil {
		return nil, err

	}
	return &se, nil
}

func parseIPs(adrs ...string) ([]net.IP, error) {
	var ips []net.IP
	for _, b := range adrs {
		lips, err := net.LookupIP(b)
		if err != nil {
			return nil, err
		}
		ips = append(ips, lips...)
	}
	return ips, nil
}

func parseTCPAddrsses(adrs ...string) ([]*net.TCPAddr, error) {
	var tcps []*net.TCPAddr
	for _, b := range adrs {
		h, p, err := net.SplitHostPort(b)
		if err != nil {
			return nil, err
		}
		ips, err := parseIPs(h)
		if err != nil {
			return nil, err
		}
		port, err := strconv.Atoi(p)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			tcps = append(tcps, &net.TCPAddr{
				IP:   ip,
				Port: port,
			})
		}
	}
	return tcps, nil
}

//BaseDir returns a dir which contains a setting file and db dir.
func (s *Setting) BaseDir() string {
	return filepath.Join(s.RootDir, s.Config.Name)
}

//InBlacklist returns true if remote is in blacklist.
func (s *Setting) InBlacklist(remote net.IP) bool {
	for _, ip := range s.BlacklistIPs {
		if ip.Equal(remote) {
			return true
		}
	}
	return false
}
