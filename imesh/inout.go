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

package imesh

import (
	"errors"

	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/setting"
)

//InOutHashType is a type in InOutHash struct.
type InOutHashType byte

//Types for in/out txs.
const (
	TypeIn InOutHashType = iota
	TypeMulin
	TypeTicketin
	TypeOut
	TypeMulout
	TypeTicketout
)

func (t InOutHashType) String() string {
	switch t {
	case TypeIn:
		return "input"
	case TypeMulin:
		return "multisig_input"
	case TypeTicketin:
		return "ticket_input"
	case TypeOut:
		return "output"
	case TypeMulout:
		return "multisig_output"
	case TypeTicketout:
		return "ticket_output"
	default:
		return ""
	}
}

//InoutHash represents in/out tx hashess.
type InoutHash struct {
	Hash  tx.Hash       `json:"hash"`
	Type  InOutHashType `json:"type"`
	Index byte          `json:"index"`
}

func newInoutHash(dat []byte) (*InoutHash, error) {
	if len(dat) != 34 {
		return nil, errors.New("invalid dat length")
	}
	ih := &InoutHash{
		Hash:  make(tx.Hash, 32),
		Type:  InOutHashType(dat[32]),
		Index: dat[33],
	}
	copy(ih.Hash, dat[:32])
	return ih, nil
}
func (ih *InoutHash) bytes() []byte {
	ary := inout2key(ih.Hash, ih.Type, ih.Index)
	return ary[:]
}

//Serialize returns serialized a 34 bytes array.
func (ih *InoutHash) Serialize() [34]byte {
	return inout2keyArray(ih.Hash, ih.Type, ih.Index)
}

func inout2keyArray(h tx.Hash, typ InOutHashType, no byte) [34]byte {
	var r [34]byte
	copy(r[:], h)
	r[32] = byte(typ)
	r[33] = no
	return r
}

func inout2key(h tx.Hash, typ InOutHashType, no byte) []byte {
	r := inout2keyArray(h, typ, no)
	return r[:]
}

func inputHashes(tr *tx.Body) []*InoutHash {
	prevs := make([]*InoutHash, 0, 1+
		len(tr.Inputs)+len(tr.MultiSigIns))
	if tr.TicketInput != nil {
		prevs = append(prevs, &InoutHash{
			Type: TypeTicketin,
			Hash: tr.TicketInput,
		})
	}
	for _, prev := range tr.Inputs {
		prevs = append(prevs, &InoutHash{
			Type: TypeIn,
			Hash: prev.PreviousTX,
		})
	}
	for _, prev := range tr.MultiSigIns {
		prevs = append(prevs, &InoutHash{
			Type: TypeMulin,
			Hash: prev.PreviousTX,
		})
	}
	return prevs
}

//GetOutput returns an output related to InOutHash ih.
//If ih is output, returns the output specified by ih.
//If ih is input, return output refered by the input.
func (ih *InoutHash) GetOutput(s *setting.Setting) (*tx.Output, error) {
	tr, err := GetTxInfo(s, ih.Hash)
	if err != nil {
		return nil, err
	}
	switch ih.Type {
	case TypeOut:
		return tr.Body.Outputs[ih.Index], nil
	case TypeIn:
		in := tr.Body.Inputs[ih.Index]
		prev, err := GetTxInfo(s, in.PreviousTX)
		if err != nil {
			return nil, err
		}
		return prev.Body.Outputs[in.Index], nil
	default:
		return nil, errors.New("type must be Input or Output")
	}
}
