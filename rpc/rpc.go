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

package rpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/AidosKuneen/aknode/setting"
)

//Run runs RPC server.
func Run(setting *setting.Setting) {
	ipport := fmt.Sprintf("%s:%d", setting.RPCBind, setting.RPCPort)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handle(setting, w, r)
	})

	s := &http.Server{
		Addr:              ipport,
		Handler:           mux,
		ReadTimeout:       time.Minute,
		WriteTimeout:      time.Minute,
		ReadHeaderTimeout: time.Minute,
		MaxHeaderBytes:    1 << 20,
	}
	fmt.Println("Starting RPC Server on", ipport)
	go func() {
		log.Println(s.ListenAndServe())
	}()
}

//Request is for parsing request from client.
type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

//Err represents error struct for response.
type Err struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
}

//Response is for respoding to clinete in jsonrpc.
type Response struct {
	Result interface{} `json:"result"`
	Error  *Err        `json:"error"`
	ID     interface{} `json:"id"`
}

func isValidAuth(s *setting.Setting, r *http.Request) bool {
	username, password, ok := r.BasicAuth()
	if !ok {
		return false
	}
	return username == s.RPCUser && password == s.RPCPassword
}

//Handle handles api calls.
func handle(s *setting.Setting, w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Fatal(err)
		}
	}()
	if !isValidAuth(s, r) {
		log.Println("failed to auth")
		w.Header().Set("WWW-Authenticate", `Basic realm="MY REALM"`)
		w.WriteHeader(401)
		if _, err := w.Write([]byte("401 Unauthorized\n")); err != nil {
			log.Println(err)
		}
		return
	}
	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	res := Response{
		ID: req.ID,
	}
	log.Println(req.Method, " is requested")
	var err error
	switch req.Method {
	/*
		case "getnewaddress":
			err = getnewaddress(s, &req, &res)
		case "listaccounts":
			err = listaccounts(s, &req, &res)
		case "listaddressgroupings":
			err = listaddressgroupings(s, &req, &res)
		case "validateaddress":
			err = validateaddress(s, &req, &res)
		case "settxfee":
			err = settxfee(s, &req, &res)
		case "gettransaction":
			err = gettransaction(s, &req, &res)
		case "getbalance":
			err = getbalance(s, &req, &res)
		case "listtransactions":
			err = listtransactions(s, &req, &res)
		case "walletpassphrase":
			err = walletpassphrase(s, &req, &res)
		case "sendmany":
			err = sendmany(s, &req, &res)
		case "sendfrom":
			err = sendfrom(s, &req, &res)
		case "sendtoaddress":
			err = sendtoaddress(s, &req, &res)
		case "sendrawtransaction":
			err = sendrawtransaction(s, &req, &res)
		case "getnodeinfo":
			err = getnodeinfo(s, &req, &res)
		case "getpeerlist":
			err = getpeerlist(s, &req, &res)
		case "getleaves":
			err = getleaves(s, &req, &res)
		case "getlasthistory":
			err = getlasthistory(s, &req, &res)
		case "getrawtransaction":
			err = getrawtransaction(s, &req, &res)
	*/
	default:
		err = errors.New(req.Method + " not supperted")
	}
	if err != nil {
		res.Error = &Err{
			Code:    -1,
			Message: err.Error(),
		}
	}
	result, err := json.Marshal(&res)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if _, err := w.Write(result); err != nil {
		log.Fatal(err)
	}
}
