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
	"strings"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/address"
	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/consensus"
	"github.com/dgraph-io/badger"
)

//DefaultMinimumFee is the minimum fee to receive minable tx.
const DefaultMinimumFee = 0.05

//Version is the version of aknode
const Version = "1.0.0"

//Address represents an address.
type Address struct {
	Address string `json:"address"`
	Port    uint16 `json:"port"`
}

//DBConfig is a set of config for db.
type DBConfig struct {
	DB     *badger.DB    `json:"-"`
	Config *aklib.Config `json:"-"`
}

//Setting is  a aknode setting.
type Setting struct {
	Version    string
	Debug      bool     `json:"debug"`
	Testnet    byte     `json:"testnet"`
	Blacklists []string `json:"blacklists"`
	RootDir    string   `json:"root_dir"`
	UseTor     bool     `json:"-"` //disabled

	MyHostPort   string   `json:"my_host_port"`
	DefaultNodes []string `json:"default_nodes"`

	Bind           string `json:"bind"`
	Port           uint16 `json:"port"`
	MaxConnections uint16 `json:"max_connections"`
	Proxy          string `json:"proxy"`

	UsePublicRPC      bool   `json:"use_public_rpc"`
	RPCBind           string `json:"rpc_bind"`
	RPCPort           uint16 `json:"rpc_port"`
	RPCUser           string `json:"rpc_user"`
	RPCPassword       string `json:"rpc_password"`
	RPCMaxConnections uint16 `json:"rpc_max_connections"`
	RPCTxTag          string `json:"rpc_tx_tag"`
	RPCAllowPublicPoW bool   `json:"rpc_allow_public_pow"`
	WalletNotify      string `json:"wallet_notify"`

	RunValidator    bool     `json:"run_validator"`
	ValidatorSecret string   `json:"validator_secret"`
	TrustedNodes    []string `json:"trusted_nodes"`

	RunExplorer            bool   `json:"run_explore"`
	ExplorerBind           string `json:"explorer_bind"`
	ExplorerPort           uint16 `json:"explorer_port"`
	ExplorerMaxConnections uint16 `json:"explorer_max_connections"`

	MinimumFee      float64 `json:"minimum_fee"`
	RunFeeMiner     bool    `json:"run_fee_miner"`
	RunTicketMiner  bool    `json:"run_ticket_miner"`
	RunTicketIssuer bool    `json:"run_ticket_issuer"`
	MinerAddress    string  `json:"miner_address"`

	DBConfig
	Stop chan struct{} `json:"-"`
}

//Load parse a json file fname , open DB and returns Settings struct .
func Load(s []byte) (*Setting, error) {
	var se Setting
	if err := json.Unmarshal(s, &se); err != nil {
		return nil, err
	}
	if int(se.Testnet) >= len(aklib.Configs) {
		return nil, errors.New("testnet must be 0(mainnet) or 1")
	}
	se.Config = aklib.Configs[se.Testnet]

	for _, a := range se.Blacklists {
		if err := se.CheckAddress(a, false, false); err != nil {
			return nil, err
		}
	}

	usr, err2 := user.Current()
	if err2 != nil {
		return nil, err2
	}
	if se.RootDir == "" {
		se.RootDir = filepath.Join(usr.HomeDir, ".aknode")
	}
	if se.Port == 0 {
		se.Port = se.Config.DefaultPort
	}
	if se.MyHostPort == "" {
		if !se.UseTor {
			se.MyHostPort = ":" + strconv.Itoa(int(se.Port))
		} else {
			return nil, errors.New("my_host_port is empty")
		}
	}
	if err := se.CheckAddress(se.MyHostPort, true, true); err != nil {
		return nil, err
	}

	if se.Bind == "" {
		se.Bind = "0.0.0.0"
	}

	if se.UseTor && se.Proxy == "" {
		return nil, errors.New("should be proxied for using Tor")
	}
	if se.MaxConnections == 0 {
		se.MaxConnections = 5
	}
	if se.RPCBind == "" {
		se.RPCBind = "127.0.0.1"
	}
	if se.RPCPort == 0 {
		se.RPCPort = se.Config.DefaultRPCPort
	}
	if (se.RPCUser != "" && se.RPCPassword == "") || (se.RPCUser == "" && se.RPCPassword != "") {
		return nil, errors.New("You must specify rpc_user and rpc_password")
	}
	if se.RPCMaxConnections == 0 {
		se.MaxConnections = 1
	}

	if se.ExplorerBind == "" {
		se.RPCBind = "127.0.0.1"
	}
	if se.ExplorerPort == 0 {
		se.RPCPort = se.Config.DefaultExplorerPort
	}
	if se.ExplorerMaxConnections == 0 {
		se.ExplorerMaxConnections = 1
	}
	if (se.RunTicketIssuer || se.RunFeeMiner || se.RunTicketMiner) && se.MinerAddress == "" {
		return nil, errors.New("You must specify mining address")
	}
	if se.RunFeeMiner && se.MinimumFee == 0 {
		se.MinimumFee = DefaultMinimumFee
	}
	if se.MinerAddress != "" {
		if _, _, err := address.ParseAddress58(se.Config, se.MinerAddress); err != nil {
			return nil, err
		}
	}

	if se.RunValidator && se.ValidatorSecret == "" {
		return nil, errors.New("You must spcrify validator_secret")
	}
	if len(se.TrustedNodes) == 0 {
		return nil, errors.New("You must spcrify trusted_nodes")
	}
	if _, err := se.TrustedNodeIDs(); err != nil {
		return nil, err
	}
	if se.ValidatorSecret != "" {
		if _, err := se.ValidatorAddress(); err != nil {
			return nil, err
		}
	}

	dbDir := filepath.Join(se.BaseDir(), "db")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, err
	}
	se.DB, err2 = db.Open(dbDir)
	if err2 != nil {
		return nil, err2
	}
	if err := os.MkdirAll(se.BaseDir(), 0755); err != nil {
		return nil, err
	}
	se.Stop = make(chan struct{})
	return &se, nil
}

//BaseDir returns a dir which contains a setting file and db dir.
func (s *Setting) BaseDir() string {
	return filepath.Join(s.RootDir, s.Config.Name)
}

//ValidatorAddress returns the validator address  from ValidatorSecret
func (s *Setting) ValidatorAddress() (*address.Address, error) {
	if s.ValidatorSecret == "" {
		return nil, errors.New("validorSecret is nil")
	}
	b, isNode, err := address.HDFrom58(s.Config, s.ValidatorSecret, []byte(""))
	if err != nil {
		return nil, err
	}
	if !isNode {
		return nil, errors.New("invalid validator_seed")
	}
	return address.NewNode(s.Config, b)
}

//TrustedNodeIDs returns the nodeids of trusted nodes.
func (s *Setting) TrustedNodeIDs() ([]consensus.NodeID, error) {
	trustedNodeIDs := make([]consensus.NodeID, len(s.TrustedNodes))
	for i, a := range s.TrustedNodes {
		ta, isNode, err := address.ParseAddress58(s.Config, a)
		if err != nil {
			return nil, err
		}
		if !isNode {
			return nil, errors.New("invalid trusted_nodes")
		}
		copy(trustedNodeIDs[i][:], ta[2:])
	}
	return trustedNodeIDs, nil
}

//InBlacklist returns true if remote is in blacklist.
func (s *Setting) InBlacklist(remote string) bool {
	h, _, err := net.SplitHostPort(remote)
	if err == nil {
		remote = h
	}
	for _, ip := range s.Blacklists {
		if ip == remote {
			return true
		}
	}
	return false
}

//ErrTorAddress represents an error  tor address is used.
var ErrTorAddress = errors.New("cannot use tor address")

//CheckAddress checks an address adr.
func (s *Setting) CheckAddress(adr string, hasPort, isEmptyHost bool) error {
	if strings.HasSuffix(adr, ".onion") {
		if s.UseTor {
			return nil
		}
		return ErrTorAddress
	}
	h, p, err2 := net.SplitHostPort(adr)
	if err2 != nil && hasPort {
		return err2
	}
	if err2 == nil && !hasPort {
		return errors.New("should not have port number")
	}
	if hasPort {
		po, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		if po > 0xffff || po <= 0 {
			return errors.New("illegal port number")
		}
		adr = h
	}
	if adr == "" {
		if !isEmptyHost {
			return errors.New("empty host name")
		}
		return nil
	}
	_, err2 = net.LookupIP(adr)
	return err2
}

//IsTrusted returns true if adr is trusted.
func (s *Setting) IsTrusted(adr string) bool {
	ok := false
	for _, t := range s.TrustedNodes {
		if t == adr {
			ok = true
		}
	}
	return ok
}
