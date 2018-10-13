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
	"net"
	"net/http"
	"time"

	"github.com/AidosKuneen/aklib/rpc"
	"github.com/AidosKuneen/aknode/setting"
	"golang.org/x/net/netutil"
)

type rpcfunc func(*setting.Setting, *rpc.Request, *rpc.Response) error

var publicRPCs = map[string]rpcfunc{
	"sendrawtx":       sendrawtx,
	"getnodeinfo":     getnodeinfo,
	"getleaves":       getleaves,
	"getlasthistory":  getlasthistory,
	"getrawtx":        getrawtx,
	"getminabletx":    getminabletx,
	"gettxsstatus":    gettxsstatus,
	"getmultisiginfo": getmultisiginfo,
	"getledger":       getledger,
}

var rpcs = map[string]rpcfunc{
	//control
	"listpeer":     listpeer,
	"listbanned":   listbanned,
	"stop":         stop,
	"dumpwallet":   dumpwallet,
	"importwallet": importwallet,
	"dumpprivkey":  dumpprivkey,

	//wallet
	"gettransaction":       gettransaction,
	"validateaddress":      validateaddress,
	"getnewaddress":        getnewaddress,
	"listaccounts":         listaccounts,
	"listaddressgroupings": listaddressgroupings,
	"settxfee":             settxfee,
	"getbalance":           getbalance,
	"listtransactions":     listtransactions,
	"getaccount":           getaccount,

	//send
	"sendmany":         sendmany,
	"sendfrom":         sendfrom,
	"sendtoaddress":    sendtoaddress,
	"walletpassphrase": walletpassphrase,
	"walletlock":       walletlock,
}

//Run runs RPC server.
func Run(setting *setting.Setting) net.Listener {
	ipport := fmt.Sprintf("%s:%d", setting.RPCBind, setting.RPCPort)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handle(setting, w, r)
	})

	s := &http.Server{
		Addr:              ipport,
		Handler:           mux,
		ReadTimeout:       30 * time.Minute,
		WriteTimeout:      30 * time.Minute,
		ReadHeaderTimeout: 30 * time.Minute,
		MaxHeaderBytes:    1 << 20,
	}
	fmt.Println("Starting RPC Server on", ipport)
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		log.Fatal(err)
	}
	var l net.Listener
	if setting.RPCMaxConnections == 0 {
		l = ln
	} else {
		l = netutil.LimitListener(ln, int(setting.RPCMaxConnections))
	}
	go func() {
		log.Println(s.Serve(l))
	}()
	return l
}

func parseParam(req *rpc.Request, data ...interface{}) (int, error) {
	if len(req.Params) == 0 {
		return 0, nil
	}
	if err := json.Unmarshal(req.Params, &data); err != nil {
		return 0, err
	}
	return len(data), nil
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
	http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
	return errors.New("failed to auth")
}

//Handle handles api calls.
func handle(s *setting.Setting, w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Println(err)
		}
	}()
	var req rpc.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	res := rpc.Response{
		ID: req.ID,
	}
	log.Println(req.Method, " is rpc.Requested")
	exist := false
	var err error
	if f, ok := publicRPCs[req.Method]; ok {
		exist = true
		if !s.UsePublicRPC {
			if err2 := isValidAuth(s, w, r); err2 != nil {
				log.Println(err2)
				return
			}
		}
		err = f(s, &req, &res)
	}
	if f, ok := rpcs[req.Method]; ok {
		exist = true
		if s.RPCUser == "" {
			err = errors.New("non public RPCS are not allowed")
		} else {
			if err2 := isValidAuth(s, w, r); err2 != nil {
				log.Println(err2)
				return
			}
			err = f(s, &req, &res)
		}
	}
	if !exist {
		err = errors.New(req.Method + " is not supported")
	}
	if err != nil {
		res.Error = &rpc.Err{
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
