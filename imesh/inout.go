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

//GetOutput returns an output related to InOutHash ih.
//If ih is output, returns the output specified by ih.
//If ih is input, return output refered by the input.
func GetOutput(s *setting.Setting, ih *tx.InoutHash) (*tx.Output, error) {
	tr, err := GetTxInfo(s.DB, ih.Hash)
	if err != nil {
		return nil, err
	}
	switch ih.Type {
	case tx.TypeOut:
		return tr.Body.Outputs[ih.Index], nil
	case tx.TypeIn:
		in := tr.Body.Inputs[ih.Index]
		prev, err := GetTxInfo(s.DB, in.PreviousTX)
		if err != nil {
			return nil, err
		}
		return prev.Body.Outputs[in.Index], nil
	default:
		return nil, errors.New("type must be Input or Output")
	}
}
