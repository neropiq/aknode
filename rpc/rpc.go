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

type rpcfunc func(*setting.Setting, *Request, *Response) error

var publicRPCs = map[string]rpcfunc{
	// "sendrawtransaction": sendrawtransaction,
	// "getnodeinfo":        getnodeinfo,
	// "getleaves":          getleaves,
	// "getlasthistory":     getlasthistory,
	// "getrawtransaction":  getrawtransaction,
	// "getminabletx":       getminabletx,
}

var rpcs = map[string]rpcfunc{
	// "gettransaction":  gettransaction,
	// "validateaddress": validateaddress,
	// "getpeerlist":     getpeerlist,
	// "addnode":         addnode,
	// "clearbanned":     clearbanned,
	// "stop":            dumpwallet,
	// "setban":          setban,
	// "listbanned":          listbanned,

	// "getnewaddress":         getnewaddress,
	// "listaccounts":          listaccounts,
	// "listaddressgroupings":  listaddressgroupings,
	// "settxfee":              settxfee,
	// "walletpassphrase":      walletpassphrase,
	// "sendmany":              sendmany,
	// "sendfrom":              sendfrom,
	// "sendtoaddress":         sendtoaddress,
	// "getbalance":            getbalance,
	// "listtransactions":      listtransactions,
	// "getaddressesbyaccount": getaddressesbyaccount,
	// "getaccount":            getaccount,
	// "dumpwallet":            dumpwallet,
	// "importwallet":            importwallet,
	// "dumpprivseed":          dumpprivseed,
	// "importprivseed":          importprivseed,
	// "keypoolrefill":          keypoolrefill,
	//"walletlock":walletlock,

}

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

func isValidAuth(s *setting.Setting, w http.ResponseWriter, r *http.Request) error {
	username, password, ok := r.BasicAuth()
	if !ok {
		return errors.New("user and password are not supplied")
	}
	if username == s.RPCUser && password == s.RPCPassword {
		return nil
	}
	w.Header().Set("WWW-Authenticate", `Basic realm="MY REALM"`)
	w.WriteHeader(401)
	if _, err := w.Write([]byte("401 Unauthorized\n")); err != nil {
		log.Println(err)
	}
	return errors.New("failed to auth")
}

//Handle handles api calls.
func handle(s *setting.Setting, w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	res := Response{
		ID: req.ID,
	}
	log.Println(req.Method, " is requested")
	exist := false
	var err error
	if f, ok := publicRPCs[req.Method]; ok {
		if !s.UsePublicRPC {
			if err2 := isValidAuth(s, w, r); err2 != nil {
				log.Println(err2)
				return
			}
		}
		exist = true
		err = f(s, &req, &res)
	}
	if f, ok := rpcs[req.Method]; ok && s.RPCUser != "" {
		if err2 := isValidAuth(s, w, r); err2 != nil {
			log.Println(err2)
			return
		}
		exist = true
		err = f(s, &req, &res)
	}
	if !exist {
		err = errors.New(req.Method + " is nots supported")
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
