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

package msg

import (
	"errors"
	"net"
	"time"

	"github.com/AidosKuneen/aklib/arypack"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/setting"

	"github.com/vmihailenco/msgpack"
)

//Commands in Header.
const (
	CmdVersion    byte = iota //Header + Version  p2p
	CmdVerack                 // Header  p2p
	CmdPing                   // Header + Nonce ,p2p
	CmdPong                   // Header+ Nonce,p2p
	CmdGetAddr                // Header,p2p
	CmdAddr                   // Header * Addrs,p2p
	CmdInv                    // Header + Inventories,broadcast
	CmdGetData                // Header+ Inventories,p2p
	CmdNotfound               // Header+ Inventories,p2p
	CmdTxs                    // Header+ Txs,p2p
	CmdStatements             // Header + Statements,p2p
	CmdGetLeaves              // Header,p2p
	CmdLeaves                 // Header + Inventories,p2p
)

//Services in Version mesasge.
const (
	ServiceFull byte = iota
	ServiceSPV
)

//Invenory types
const (
	InvError byte = iota
	InvTx
	InvMinFeeTx
	InvMinTicketTx
	InvStatement
	InvQuorumSet
)

const (
	maxLength    = 2000000
	maxAddrs     = 1000
	maxInv       = 1000
	maxTx        = 500
	maxStatement = 1000
)

const userAgent = "AKnode Versin 1.0"

//MessageVersion is a version of the message.
const MessageVersion = 1

//Header  is a header of wire protocol.
type Header struct {
	Magic   uint32
	Length  uint32
	Command byte
}

//Version is a message when a node creates an outgoing connection.
type Version struct {
	Version   uint16
	Service   byte
	AddrFrom  Addr
	AddrTo    Addr
	UserAgent string
}

//Addr is an IP address and port.
type Addr []byte

//Addrs provides information on known nodes of the network.
type Addrs struct {
	Timestamp time.Time
	Addrs     []Addr
}

//Inventory vectors are used for notifying other nodes about objects they have or data which is being requested.
type Inventory struct {
	Type byte
	Hash [32]byte
}

//Inventories is a slice of Inventory, for inv, getdata, notfound messages.
type Inventories []*Inventory

//Txs describes a bitcoin transaction, in reply to getdata
type Txs []*tx.Transaction

//Nonce  is a  nonce for ping and pong.
type Nonce [32]byte

//GetLeaves is for getting leaves from From to To.
type GetLeaves struct {
	From [32]byte
	To   [32]byte
}

//Write write a message to con.
func Write(s *setting.Setting, m interface{}, cmd byte, conn *net.TCPConn) error {
	dat := arypack.Marshal(m)
	if len(dat) > maxLength {
		return errors.New("packet is too big")
	}
	h := &Header{
		Magic:   s.Config.MessageMagic,
		Length:  uint32(len(dat)),
		Command: cmd,
	}
	dat2 := arypack.Marshal(h)

	if _, err := conn.Write(dat); err != nil {
		return err
	}
	_, err := conn.Write(dat2)
	return err
}

//ReadHeader read a message from con and returns msg type and its contents in byte.
func ReadHeader(s *setting.Setting, conn *net.TCPConn) (byte, []byte, error) {
	var h Header
	var err error

	enc := msgpack.NewEncoder(conn).StructAsArray(true)
	if err := enc.Encode(&h); err != nil {
		return 0, nil, err
	}
	if h.Length > maxLength {
		return 0, nil, errors.New("message is too big")
	}
	if s.Config.MessageMagic != h.Magic {
		return 0, nil, errors.New("invalid magic")
	}
	buf := make([]byte, h.Length)
	n := 0
	for l := uint32(0); l < h.Length; l += uint32(n) {
		if n, err = conn.Read(buf[l:]); err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				continue
			}
			return 0, nil, err
		}
	}
	return h.Command, buf, nil
}

//ReadVersion read and make a Version struct.
func ReadVersion(buf []byte) (*Version, error) {
	var v Version
	err := arypack.Unmarshal(buf, &v)
	if v.Version != MessageVersion {
		return nil, errors.New("invalid version")
	}
	if v.Service != ServiceFull {
		return nil, errors.New("unknown service")
	}
	return &v, err
}

//ReadAddrs read and make a Addrs struct.
func ReadAddrs(buf []byte) (*Addrs, error) {
	var v Addrs
	err := arypack.Unmarshal(buf, &v)
	if err != nil {
		return nil, err
	}
	if len(v.Addrs) > maxAddrs {
		return nil, errors.New("addrs is too long")
	}
	return &v, nil
}

//ReadInventories read and make a Addrs struct.
func ReadInventories(buf []byte) (Inventories, error) {
	var v Inventories
	err := arypack.Unmarshal(buf, &v)
	if len(v) > maxInv {
		return nil, errors.New("Inventories is too long")
	}
	return v, err
}

//ReadTxs read and make a Addrs struct.
func ReadTxs(buf []byte) (Txs, error) {
	var v Txs
	err := arypack.Unmarshal(buf, &v)
	if len(v) > maxTx {
		return nil, errors.New("Txs is too long")
	}
	return v, err
}

//ReadNonce read and make a Addrs struct.
func ReadNonce(buf []byte) (Nonce, error) {
	var v Nonce
	err := arypack.Unmarshal(buf, &v)
	return v, err
}

//ReadGetLeaves read and make a GetLeaves struct.
func ReadGetLeaves(buf []byte) (*GetLeaves, error) {
	var v GetLeaves
	err := arypack.Unmarshal(buf, &v)
	return &v, err
}

//NewVersion returns Verstion struct.
func NewVersion(s *setting.Setting, to Addr) *Version {
	v := &Version{
		Version:   MessageVersion,
		Service:   ServiceFull,
		UserAgent: userAgent,
		AddrTo:    to,
		AddrFrom:  Addr(s.MyHostName),
	}
	return v
}
