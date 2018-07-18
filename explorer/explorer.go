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
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/AidosKuneen/aknode/setting"
	"github.com/alecthomas/template"
)

const wwwPath = "../cmd/aknode/"

var tmpl = template.New("")

//Run runs explorer server.
func Run(setting *setting.Setting) {
	if _, err := tmpl.ParseGlob(wwwPath + "public/*.tpl"); err != nil {
		log.Fatal(err)
	}

	ipport := fmt.Sprintf("%s:%d", setting.ExplorerBind, setting.ExplorerPort)
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		indexHandle(setting, w, r)
	})
	mux.HandleFunc("/search/", func(w http.ResponseWriter, r *http.Request) {
		searchHandle(setting, w, r)
	})
	for _, stat := range []string{"img", "css", "js"} {
		mux.HandleFunc("/"+stat+"/", func(w http.ResponseWriter, r *http.Request) {
			http.FileServer(http.Dir(stat + "/"))
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
	// var ni *giota.GetNodeInfoResponse
	// var txs *giota.GetTransactionsToApproveResponse
	// var err1 error
	// var err2 error
	// var server string = "http://localhost:14266"
	// for i := 0; i < 5; i++ {
	// 	//		server = giota.RandomNode()
	// 	api := giota.NewAPI(server, client)
	// 	ni, err1 = api.GetNodeInfo()
	// 	txs, err2 = api.GetTransactionsToApprove(27)
	// 	if err1 == nil && err2 == nil {
	// 		break
	// 	}
	// }
	// if renderIfError(w, err1, err2) {
	// 	return
	// }

	// err := tmpl.ExecuteTemplate(w, "index.tpl", struct {
	// 	Server   string
	// 	NodeInfo *giota.GetNodeInfoResponse
	// 	Tx       *giota.GetTransactionsToApproveResponse
	// }{
	// 	server,
	// 	ni,
	// 	txs,
	// })
	// if err != nil {
	// 	log.Print(err)
	// }
}

func searchHandle(s *setting.Setting, w http.ResponseWriter, r *http.Request) {
}
