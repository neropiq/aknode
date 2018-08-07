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
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
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
	mux.HandleFunc("/qrcode/", func(w http.ResponseWriter, r *http.Request) {
		qrHandle(setting, w, r)
	})
	for _, stat := range []string{"image", "css", "js"} {
		box := packr.NewBox(filepath.Join(wwwPath, stat))
		mux.Handle("/"+stat+"/", http.StripPrefix("/"+stat+"/", http.FileServer(box)))
	}

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
	_, _, err := address.ParseAddress58(id, s.Config)
	if err != nil {
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
	info := struct {
		Net     string
		Version string
		Peers   int
		Time    time.Time
		Txs     uint64
		Leaves  int
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
	ti, err := imesh.GetTxInfo(s, txid)
	if !ok {
		renderError(w, err.Error())
		return
	}

	info := struct {
		TXID         string
		Created      time.Time
		Received     time.Time
		Status       imesh.TxStatus
		Inputs       []*tx.Output
		MInputs      []*tx.MultiSigOut
		Signs        map[string]bool
		Outputs      []*tx.Output
		MOutputs     []*tx.MultiSigOut
		TicketInput  string
		TicketOutput string
		Message      string
		MessageStr   string
		Nonce        []uint32
		GNonce       uint32
		LockTime     time.Time
		Parents      []tx.Hash
	}{
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
	}
	for i, inp := range ti.Body.Inputs {
		info.Inputs[i], err = imesh.PreviousOutput(s, inp)
		if err != nil {
			renderError(w, err.Error())
			return
		}
	}
	for i, inp := range ti.Body.MultiSigIns {
		ti, err := imesh.GetTxInfo(s, inp.PreviousTX)
		if err != nil {
			renderError(w, err.Error())
			return
		}
		info.MInputs[i] = ti.Body.MultiSigOuts[inp.Index]
	}
	if len(ti.Body.MultiSigIns) != 0 {
		tr, err := imesh.GetTx(s, txid)
		if err != nil {
			renderError(w, err.Error())
			return
		}
		for _, sig := range tr.Signatures {
			sigadr, err := sig.Address(s.Config)
			if err != nil {
				renderError(w, err.Error())
				return
			}
			info.Signs[address.To58(sigadr)] = true
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
	_, _, err := address.ParseAddress58(id, s.Config)
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
		Address    string
		Balance    uint64
		UTXOs      []tx.Hash
		MUTXOs     []tx.Hash
		Inputs     []tx.Hash
		MInputs    []tx.Hash
		Ticketins  []tx.Hash
		Ticketouts []tx.Hash
	}{
		Address: id,
	}
	for _, h := range hist {
		switch h.Type {
		case tx.TypeIn:
			info.Inputs = append(info.Inputs, h.Hash)
		case tx.TypeMulin:
			info.MInputs = append(info.MInputs, h.Hash)
		case tx.TypeOut:
			ti, err := imesh.GetTxInfo(s, h.Hash)
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

func renderError(w http.ResponseWriter, str string) {
	log.Println(str)
	w.WriteHeader(http.StatusNotFound)
	err := tmpl.ExecuteTemplate(w, "err", str)
	if err != nil {
		log.Print(err)
	}
}

func searchHandle(s *setting.Setting, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	id := q.Get("id")
	_, err1 := hex.DecodeString(id)
	_, _, err2 := address.ParseAddress58(id, s.Config)
	switch {
	case err1 == nil:
		http.Redirect(w, r, "/tx?id="+id, http.StatusFound)
		return
	case err2 == nil:
		http.Redirect(w, r, "/address?id="+id, http.StatusFound)
		return
	default:
		renderError(w, notFound)
		return
	}
}
