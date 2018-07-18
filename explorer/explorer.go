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
	"path/filepath"
	"time"

	"github.com/AidosKuneen/aklib/address"
	"github.com/AidosKuneen/aknode/imesh/leaves"
	"github.com/AidosKuneen/aknode/node"

	"github.com/AidosKuneen/aknode/imesh"

	"github.com/AidosKuneen/aknode/setting"
	"github.com/alecthomas/template"
	"github.com/gobuffalo/packr"
)

const (
	wwwPath  = "../cmd/aknode/static"
	notFound = "we couldn't find what you are looking for."
)

var tmpl = template.New("")

//Run runs explorer server.
func Run(setting *setting.Setting) {
	box := packr.NewBox(filepath.Join(wwwPath, "templates"))
	for _, t := range box.List() {
		str, err := box.MustString(t)
		if err != nil {
			log.Fatal(err)
		}
		tmpl = template.Must(tmpl.Parse(str))
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
	for _, stat := range []string{"img", "css", "js"} {
		box := packr.NewBox(filepath.Join(wwwPath, stat))
		mux.HandleFunc("/"+stat+"/", func(w http.ResponseWriter, r *http.Request) {
			http.StripPrefix("/"+stat+"/", http.FileServer(box))
		})
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
		Time:    time.Now(),
		Txs:     imesh.GetTxNo(),
		Leaves:  leaves.Size(),
	}

	err := tmpl.ExecuteTemplate(w, "index.tpl", &info)
	if err != nil {
		log.Print(err)
	}
}
func txHandle(s *setting.Setting, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	id := q.Get("id")
	txid, err := hex.DecodeString(id)
	if err != nil {
		log.Println(err)
		renderError(w, err.Error())
	}
	ok, err := imesh.Has(s, txid)
	if err != nil {
		log.Println(err)
		renderError(w, err.Error())
	}
	if !ok {
		renderError(w, notFound)
	}
	info := struct {
		Created  time.Time
		Received time.Time
		Status   byte
		Inputs   struct {
			Address string
			Amount  float64
		}
		MInputs struct {
			N       int
			Address struct {
				HasSign bool
				Address string
				Amount  float64
			}
		}
	}{}
	err = tmpl.ExecuteTemplate(w, "tx.tpl", &info)
	if err != nil {
		log.Print(err)
	}
}

func addressHandle(s *setting.Setting, w http.ResponseWriter, r *http.Request) {
}

func renderError(w http.ResponseWriter, str string) {
	err := tmpl.ExecuteTemplate(w, "err.tpl", str)
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
