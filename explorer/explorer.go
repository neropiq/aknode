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

package explorer

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/address"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/imesh/leaves"
	"github.com/AidosKuneen/aknode/node"
	"github.com/AidosKuneen/aknode/setting"
	"github.com/alecthomas/template"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/gobuffalo/packr"
	qrcode "github.com/skip2/go-qrcode"
)

const (
	notFound = "we couldn't find what you are looking for."
)

var tmpl = template.New("")

//Run runs explorer server.
func Run(setting *setting.Setting) {
	p, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	wwwPath := filepath.Join(p, "static")
	if _, err := os.Stat(wwwPath); err != nil {
		wwwPath = filepath.Join(p, "../cmd/aknode/static")
	}
	funcMap := template.FuncMap{
		"toADK": func(amount uint64) string {
			p := message.NewPrinter(language.English)
			return p.Sprintf("%.8f", float64(amount)/aklib.ADK)
		},
		"tformat": func(t time.Time) string {
			return t.Format("2006-01-02 15:04:05 MST")
		},
		"add": func(a, b int) int {
			return a + b
		},
		"duration": func(a time.Time) string {
			dur := time.Now().Sub(a)
			switch {
			case dur < time.Minute:
				return strconv.Itoa(int(dur.Seconds())) + " second(s)"
			case dur < time.Hour:
				return strconv.Itoa(int(dur.Minutes())) + " minute(s)"
			case dur < 24*time.Hour:
				return strconv.Itoa(int(dur.Hours())) + " hour(s)"
			default:
				return strconv.Itoa(int(dur.Hours()/24)) + " day(s)"
			}
		},
	}
	tmpl.Funcs(funcMap)
	box := packr.NewBox(filepath.Join(wwwPath, "templates"))
	for _, t := range box.List() {
		str, err := box.MustString(t)
		if err != nil {
			log.Fatal(t, err)
		}
		tmpl, err = tmpl.Parse(str)
		if err != nil {
			log.Fatal(t, " ", err)
		}
	}

	ipport := fmt.Sprintf("%s:%d", setting.ExplorerBind, setting.ExplorerPort)
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		indexHandle(setting, w, r)
	})
	mux.HandleFunc("/search/", func(w http.ResponseWriter, r *http.Request) {
		searchHandle(setting, w, r)
	})
	mux.HandleFunc("/tx/", func(w http.ResponseWriter, r *http.Request) {
		txHandle(setting, w, r)
	})
	mux.HandleFunc("/address/", func(w http.ResponseWriter, r *http.Request) {
		addressHandle(setting, w, r)
	})
	mux.HandleFunc("/maddress/", func(w http.ResponseWriter, r *http.Request) {
		maddressHandle(setting, w, r)
	})
	mux.HandleFunc("/qrcode/", func(w http.ResponseWriter, r *http.Request) {
		qrHandle(setting, w, r)
	})
	for _, stat := range []string{"images", "css", "js", "icofont"} {
		box := packr.NewBox(filepath.Join(wwwPath, stat))
		mux.Handle("/"+stat+"/", http.StripPrefix("/"+stat+"/", http.FileServer(box)))
	}
	ibox := packr.NewBox(filepath.Join(wwwPath, "images"))
	mux.Handle("/favicon.ico", http.StripPrefix("/", http.FileServer(ibox)))

	s := &http.Server{
		Addr:              ipport,
		Handler:           mux,
		ReadTimeout:       time.Minute,
		WriteTimeout:      time.Minute,
		ReadHeaderTimeout: time.Minute,
		MaxHeaderBytes:    1 << 20,
	}
	fmt.Println("Starting Explorer Server on", ipport)
	go func() {
		log.Println(s.ListenAndServe())
	}()
}

func qrHandle(s *setting.Setting, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	id := q.Get("id")
	_, _, err1 := address.ParseAddress58(s.Config, id)
	_, err2 := address.ParseMultisigAddress(s.Config, id)
	if err1 != nil && err2 != nil {
		renderError(w, "invalid address")
		return
	}
	qr, err := qrcode.New(id, qrcode.High)
	if err != nil {
		log.Fatal(err)
	}
	if err := qr.Write(256, w); err != nil {
		log.Fatal(err)
	}
}

//Handle handles api calls.
func indexHandle(s *setting.Setting, w http.ResponseWriter, r *http.Request) {
	type Statement struct {
		Index        int
		Time         time.Time
		Transactions int
		ADK          uint64
	}
	type Transaction struct {
		ID   string
		ADK  uint64
		Time time.Time
	}
	info := struct {
		Net          string
		Version      string
		Peers        int
		Time         time.Time
		Txs          uint64
		Leaves       int
		Statements   []Statement
		Transactions []Transaction
	}{
		Net:     s.Config.Name,
		Version: setting.Version,
		Peers:   node.ConnSize(),
		Time:    time.Now().Truncate(time.Second),
		Txs:     imesh.GetTxNo(),
		Leaves:  leaves.Size(),
	}

	err := tmpl.ExecuteTemplate(w, "index", &info)
	if err != nil {
		renderError(w, err.Error())
	}
}

func txHandle(s *setting.Setting, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	id := q.Get("id")
	txid, err := hex.DecodeString(id)
	if err != nil {
		renderError(w, err.Error())
		return
	}
	ok, err := imesh.Has(s, txid)
	if err != nil {
		renderError(w, err.Error())
		return
	}
	if !ok {
		renderError(w, notFound)
		return
	}
	ti, err := imesh.GetTxInfo(s.DB, txid)
	if !ok {
		renderError(w, err.Error())
		return
	}

	info := struct {
		Net                string
		TXID               string
		Created            time.Time
		Received           time.Time
		Status             imesh.TxStatus
		Inputs             []*tx.Output
		MInputs            []*tx.MultiSigOut
		Signs              map[string]bool
		Outputs            []*tx.Output
		MOutputs           []*tx.MultiSigOut
		TicketInput        string
		TicketOutput       string
		Message            string
		MessageStr         string
		Nonce              []uint32
		GNonce             uint32
		LockTime           time.Time
		Parents            []tx.Hash
		GetMultisigAddress func(*tx.MultiSigOut) string
	}{
		Net:        s.Config.Name,
		TXID:       id,
		Created:    ti.Body.Time,
		Received:   ti.Received,
		Status:     ti.Status,
		Inputs:     make([]*tx.Output, len(ti.Body.Inputs)),
		MInputs:    make([]*tx.MultiSigOut, len(ti.Body.MultiSigIns)),
		Signs:      make(map[string]bool),
		Outputs:    ti.Body.Outputs,
		MOutputs:   ti.Body.MultiSigOuts,
		Message:    hex.EncodeToString(ti.Body.Message),
		MessageStr: string(ti.Body.Message),
		Nonce:      ti.Body.Nonce,
		GNonce:     ti.Body.Gnonce,
		LockTime:   ti.Body.LockTime,
		Parents:    ti.Body.Parent,
		GetMultisigAddress: func(out *tx.MultiSigOut) string {
			return out.Address(s.Config)
		},
	}
	if ti.Body.TicketOutput != nil {
		info.TicketOutput = ti.Body.TicketOutput.String()
	}
	if ti.Body.TicketInput != nil {
		ti, err := imesh.GetTxInfo(s.DB, ti.Body.TicketInput)
		if err != nil {
			renderError(w, err.Error())
			return
		}
		info.TicketInput = ti.Body.TicketOutput.String()
	}
	for i, inp := range ti.Body.Inputs {
		info.Inputs[i], err = imesh.PreviousOutput(s, inp)
		if err != nil {
			renderError(w, err.Error())
			return
		}
	}
	for i, inp := range ti.Body.MultiSigIns {
		ti, err := imesh.GetTxInfo(s.DB, inp.PreviousTX)
		if err != nil {
			renderError(w, err.Error())
			return
		}
		info.MInputs[i] = ti.Body.MultiSigOuts[inp.Index]
	}
	if len(ti.Body.MultiSigIns) != 0 {
		tr, err := imesh.GetTx(s.DB, txid)
		if err != nil {
			renderError(w, err.Error())
			return
		}
		for _, sig := range tr.Signatures {
			sigadr := sig.Address(s.Config, false)
			adr, err := address.Address58(s.Config, sigadr)
			if err != nil {
				renderError(w, err.Error())
			}
			info.Signs[adr] = true
		}
	}
	err = tmpl.ExecuteTemplate(w, "tx", &info)
	if err != nil {
		renderError(w, err.Error())
	}
}

func addressHandle(s *setting.Setting, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	id := q.Get("id")
	_, _, err := address.ParseAddress58(s.Config, id)
	if err != nil {
		renderError(w, err.Error())
		return
	}
	hist, err := imesh.GetHisoty(s, id, true)
	if err != nil {
		renderError(w, notFound)
		return
	}
	info := struct {
		Net                 string
		Address             string
		Balance             uint64
		Received            uint64
		ReceivedUnconfirmed uint64
		Send                uint64
		UTXOs               []tx.Hash
		MUTXOs              []tx.Hash
		Inputs              []tx.Hash
		MInputs             []tx.Hash
		Ticketins           []tx.Hash
		Ticketouts          []tx.Hash
	}{
		Net:     s.Config.Name,
		Address: id,
	}
	for _, h := range hist {
		switch h.Type {
		case tx.TypeIn:
			info.Inputs = append(info.Inputs, h.Hash)
		case tx.TypeMulin:
			info.MInputs = append(info.MInputs, h.Hash)
		case tx.TypeOut:
			ti, err := imesh.GetTxInfo(s.DB, h.Hash)
			if err != nil {
				renderError(w, err.Error())
				return
			}
			info.Balance += ti.Body.Outputs[h.Index].Value
			info.UTXOs = append(info.UTXOs, h.Hash)
		case tx.TypeMulout:
			info.MUTXOs = append(info.MUTXOs, h.Hash)
		case tx.TypeTicketin:
			info.Ticketins = append(info.Ticketins, h.Hash)
		case tx.TypeTicketout:
			info.Ticketouts = append(info.Ticketouts, h.Hash)
		}
	}
	err = tmpl.ExecuteTemplate(w, "address", &info)
	if err != nil {
		renderError(w, err.Error())
	}
}

func maddressHandle(s *setting.Setting, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	id := q.Get("id")
	madr, err := address.ParseMultisigAddress(s.Config, id)
	if err != nil {
		renderError(w, err.Error())
		return
	}
	msig, err := imesh.GetMultisig(s.DB, madr)
	if err != nil {
		renderError(w, notFound)
		return
	}
	hist, err := imesh.GetHisoty(s, msig.Addresses[0].String(), true)
	if err != nil {
		renderError(w, notFound)
		return
	}
	info := struct {
		Net                 string
		Struct              *tx.MultisigStruct
		Address             string
		Balance             uint64
		Received            uint64
		Send                uint64
		ReceivedUnconfirmed uint64
		UTXOs               []tx.Hash
		Inputs              []tx.Hash
	}{
		Net:     s.Config.Name,
		Struct:  msig,
		Address: id,
	}
	for _, h := range hist {
		switch h.Type {
		case tx.TypeMulin:
			tr, err := imesh.GetTx(s.DB, h.Hash)
			if err != nil {
				renderError(w, err.Error())
				return
			}
			mout, err := imesh.PreviousMultisigOutput(s, tr.MultiSigIns[h.Index])
			if err != nil {
				renderError(w, err.Error())
				return
			}
			if !bytes.Equal(madr, mout.AddressByte(s.Config)) {
				continue
			}
			info.Inputs = append(info.Inputs, h.Hash)
		case tx.TypeMulout:
			tr, err := imesh.GetTx(s.DB, h.Hash)
			if err != nil {
				renderError(w, err.Error())
				return
			}
			mout := tr.Body.MultiSigOuts[h.Index]
			if !bytes.Equal(madr, mout.AddressByte(s.Config)) {
				continue
			}
			info.Balance += mout.Value
			info.UTXOs = append(info.UTXOs, h.Hash)
		}
	}
	err = tmpl.ExecuteTemplate(w, "maddress", &info)
	if err != nil {
		renderError(w, err.Error())
	}
}
func renderError(w http.ResponseWriter, str string) {
	log.Println(str)
	w.WriteHeader(http.StatusNotFound)
	err := tmpl.ExecuteTemplate(w, "404", str)
	if err != nil {
		log.Print(err)
	}
}

func searchHandle(s *setting.Setting, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	id := q.Get("id")
	_, err1 := hex.DecodeString(id)
	_, _, err2 := address.ParseAddress58(s.Config, id)
	_, err3 := address.ParseMultisigAddress(s.Config, id)
	switch {
	case err1 == nil:
		http.Redirect(w, r, "/tx?id="+id, http.StatusFound)
		return
	case err2 == nil:
		http.Redirect(w, r, "/address?id="+id, http.StatusFound)
		return
	case err3 == nil:
		http.Redirect(w, r, "/maddress?id="+id, http.StatusFound)
		return
	default:
		renderError(w, notFound)
		return
	}
}
