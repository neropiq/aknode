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
	"io"
	"log"

	"github.com/AidosKuneen/aklib/arypack"
	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/setting"

	"github.com/vmihailenco/msgpack"
)

//Commands in Header.
const (
	CmdVersion   byte = iota + 1 //Header + Version  p2p
	CmdVerack                    // Header  p2p
	CmdPing                      // Header + Nonce ,p2p
	CmdPong                      // Header+ Nonce,p2p
	CmdGetAddr                   // Header,p2p
	CmdAddr                      // Header * Addrs,p2p
	CmdInv                       // Header + Inventories,broadcast
	CmdGetData                   // Header+ Inventories,p2p
	CmdTxs                       // Header+ Txs,p2p
	CmdGetLeaves                 // Header + LeavesFrom,p2p
	CmdLeaves                    // Header + Inventories,p2p
	CmdClose                     //Header,p2p
)

//Services in Version mesasge.
const (
	ServiceFull byte = iota
)

//InvType is a tx type of Inv.
type InvType byte

//Invenory types
const (
	InvTxNormal InvType = iota
	InvTxRewardTicket
	InvTxRewardFee
)

//TxType2DBHeader return db beader from tx type.
func TxType2DBHeader(typ tx.Type) (db.Header, error) {
	switch typ {
	case tx.TxNormal:
		return db.HeaderTxInfo, nil
	case tx.TxRewardTicket:
		return db.HeaderTxRewardTicket, nil
	case tx.TxRewardFee:
		return db.HeaderTxRewardFee, nil
	default:
		return 255, errors.New("invalid type")
	}
}

//TxType2InvType return inventory type from tx type.
func TxType2InvType(typ tx.Type) (InvType, error) {
	switch typ {
	case tx.TxNormal:
		return InvTxNormal, nil
	case tx.TxRewardTicket:
		return InvTxRewardTicket, nil
	case tx.TxRewardFee:
		return InvTxRewardFee, nil
	default:
		return 255, errors.New("invalid type")
	}
}

//ToTxType return a tx type from inventory type.
func (it InvType) ToTxType() (tx.Type, error) {
	switch it {
	case InvTxNormal:
		return tx.TxNormal, nil
	case InvTxRewardTicket:
		return tx.TxRewardTicket, nil
	case InvTxRewardFee:
		return tx.TxRewardFee, nil
	default:
		return 255, errors.New("invalid type")
	}
}

//Maxs for messages.
const (
	MaxLength = 1000000
	MaxAddrs  = 100
	MaxInv    = 100
	MaxTx     = 500
	MaxLeaves = 10000
)

const userAgent = "AKnode Versin 0.01"

//messageVersion is a version of the message.
const messageVersion = 1

//Header  is a header of wire protocol.
type Header struct {
	Magic   uint32
	Length  uint32
	Command byte
}

//Version is a message when a node creates an outgoing connection.
type Version struct {
	Version   uint16
	AddrFrom  Addr
	AddrTo    Addr
	UserAgent string
}

//Addr is an IP address and port.
type Addr struct {
	Address string
	Service byte
}

//NewAddr returns a Addr struct.
func NewAddr(adr string, service byte) *Addr {
	return &Addr{
		Address: adr,
		Service: service,
	}
}

//Addrs provides information on known nodes of the network.
type Addrs []Addr

//Inventory vectors are used for notifying other nodes about objects they have or data which is being requested.
type Inventory struct {
	Type InvType
	Hash [32]byte
}

//Inventories is a slice of Inventory, for inv, getdata, notfound messages.
type Inventories []*Inventory

//Tx describes a  transaction with Invenroty type.
type Tx struct {
	Type InvType
	Tx   *tx.Transaction
}

//Txs describes a transaction slice in reply to getTxs
type Txs []*Tx

//Nonce  is a  nonce for ping and pong.
type Nonce [32]byte

//LeavesFrom is for getting leaves from from.
type LeavesFrom [32]byte

//Write write a message to con.
func Write(s *setting.Setting, m interface{}, cmd byte, conn io.ReadWriter) error {
	var dat []byte
	if m != nil {
		dat = arypack.Marshal(m)
		if len(dat) > MaxLength {
			return errors.New("packet is too big")
		}
	}
	h := &Header{
		Magic:   s.Config.MessageMagic,
		Length:  uint32(len(dat)),
		Command: cmd,
	}
	if _, err := conn.Write(arypack.Marshal(h)); err != nil {
		return err
	}
	if m == nil {
		return nil
	}
	_, err := conn.Write(dat)
	return err
}

type unbuf struct {
	reader io.Reader
	last   byte
	unread bool
}

func (bs *unbuf) ReadByte() (byte, error) {
	if bs.unread {
		bs.unread = false
		return bs.last, nil
	}
	b := make([]byte, 1)
	_, err := bs.reader.Read(b)
	bs.last = b[0]
	return b[0], err
}
func (bs *unbuf) UnreadByte() error {
	if bs.unread {
		return errors.New("called UnreadByte twice")
	}
	bs.unread = true
	return nil
}
func (bs *unbuf) Read(p []byte) (n int, err error) {
	return bs.reader.Read(p)
}

//ReadHeader read a message from con and returns msg type.
func ReadHeader(s *setting.Setting, conn io.ReadWriter) (byte, []byte, error) {
	var h Header
	var err error
	bs := &unbuf{
		reader: conn,
	}
	dec := msgpack.NewDecoder(bs)
	if err := dec.Decode(&h); err != nil {
		log.Println(err)
		return 0, nil, err
	}
	if h.Length > MaxLength {
		return 0, nil, errors.New("message is too big")
	}
	if s.Config.MessageMagic != h.Magic {
		return 0, nil, errors.New("invalid magic")
	}
	buf := make([]byte, h.Length)
	n := 0
	for l := uint32(0); l < h.Length; l += uint32(n) {
		if n, err = conn.Read(buf[l:]); err != nil {
			return 0, nil, err
		}
	}
	return h.Command, buf, nil
}

//ReadVersion read and make a Version struct.
func ReadVersion(s *setting.Setting, buf []byte) (*Version, error) {
	var v Version
	if err := arypack.Unmarshal(buf, &v); err != nil {
		return nil, err
	}
	if v.Version != messageVersion {
		return nil, errors.New("invalid version")
	}
	if v.AddrFrom.Service != ServiceFull {
		return nil, errors.New("unknown service")
	}
	if v.AddrTo.Service != ServiceFull {
		return nil, errors.New("unknown service")
	}
	if err := s.CheckAddress(string(v.AddrFrom.Address), true, true); err != nil {
		return nil, err
	}
	if err := s.CheckAddress(string(v.AddrTo.Address), true, false); err != nil {
		return nil, err
	}
	return &v, nil
}

//ReadAddrs read and make a Addrs struct.
func ReadAddrs(s *setting.Setting, buf []byte) (*Addrs, error) {
	var v Addrs
	if err := arypack.Unmarshal(buf, &v); err != nil {
		return nil, err
	}
	if len(v) > MaxAddrs {
		return nil, errors.New("addrs is too long")
	}
	for i := len(v) - 1; i >= 0; i-- {
		adr := v[i]
		if err := s.CheckAddress(string(adr.Address), true, false); err != nil {
			if err != setting.ErrTorAddress {
				return nil, err
			}
			v = append(v[:i], v[i+1:]...)
		}
	}
	return &v, nil
}

//ReadInventories read and make a Inventories struct.
func ReadInventories(buf []byte) (Inventories, error) {
	var v Inventories
	if err := arypack.Unmarshal(buf, &v); err != nil {
		return nil, err
	}
	if len(v) > MaxInv {
		return nil, errors.New("Inventories is too long")
	}
	return v, nil
}

//ReadTxs read and make a Txs struct.
func ReadTxs(buf []byte) (Txs, error) {
	var v Txs
	if err := arypack.Unmarshal(buf, &v); err != nil {
		return nil, err
	}
	if len(v) > MaxTx {
		return nil, errors.New("Txs is too long")
	}
	return v, nil
}

//ReadNonce read and make a Nonce struct.
func ReadNonce(buf []byte) (*Nonce, error) {
	var v Nonce
	if err := arypack.Unmarshal(buf, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

//ReadLeavesFrom read and make a LeavesFrom struct.
func ReadLeavesFrom(buf []byte) (*LeavesFrom, error) {
	var v LeavesFrom
	if err := arypack.Unmarshal(buf, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

//NewVersion returns Verstion struct.
func NewVersion(s *setting.Setting, to Addr) *Version {
	return &Version{
		Version:   messageVersion,
		UserAgent: userAgent,
		AddrTo:    to,
		AddrFrom:  *NewAddr(s.MyHostPort, ServiceFull),
	}
}
