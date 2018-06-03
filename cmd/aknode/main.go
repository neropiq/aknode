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

package aknode

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"runtime"
	"syscall"
	"time"

	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/imesh/leaves"
	"github.com/AidosKuneen/aknode/node"
	"github.com/AidosKuneen/aknode/rpc"
	"github.com/AidosKuneen/aknode/setting"

	"github.com/natefinch/lumberjack"
)

func onSigs(se *setting.Setting) {
	sig := make(chan os.Signal)
	signal.Notify(sig,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	go func() {
		s := <-sig
		switch s {
		case syscall.SIGHUP:
			if err := se.DB.Close(); err != nil {
				log.Println(err)
			}
			os.Exit(0)

		// kill -SIGINT XXXX or Ctrl+c
		case syscall.SIGINT:
			if err := se.DB.Close(); err != nil {
				log.Println(err)
			}
			os.Exit(0)

		case syscall.SIGTERM:
			if err := se.DB.Close(); err != nil {
				log.Println(err)
			}
			os.Exit(0)

		case syscall.SIGQUIT:
			if err := se.DB.Close(); err != nil {
				log.Println(err)
			}
			os.Exit(0)

		default:
			log.Println("Unknown signal.")
			if err := se.DB.Close(); err != nil {
				log.Println(err)
			}
			os.Exit(0)
		}
	}()
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	var verbose bool
	flag.BoolVar(&verbose, "verbose", false, "outputs logs to stdout.")
	flag.Parse()
	if !verbose {
		l := &lumberjack.Logger{
			Filename:   path.Join("aknode.log"),
			MaxSize:    10, // megabytes
			MaxBackups: 10,
			MaxAge:     28, //days
		}
		log.SetOutput(l)
	}

	setting := setting.Init("aknode.json")
	onSigs(setting)
	node.Init(setting)
	imesh.Init(setting)
	leaves.Init(setting)
	if err := node.Bootstrap(setting); err != nil {
		fmt.Println(err)
		log.Fatal(err)
	}
	runNode(setting)
	node.GoCron(setting)

	if setting.Debug {
		//for pprof
		go func() {
			runtime.SetBlockProfileRate(1)
			log.Println(http.ListenAndServe("127.0.0.1:6061", nil))
		}()
	}

	if setting.RunRPCServer {
		runRPC(setting)
	}
}

func runRPC(setting *setting.Setting) {
	ipport := fmt.Sprintf("%s:%d", setting.RPCBind, setting.RPCPort)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		rpc.Handle(setting, w, r)
	})

	s := &http.Server{
		Addr:              ipport,
		Handler:           mux,
		ReadTimeout:       time.Minute,
		WriteTimeout:      time.Minute,
		ReadHeaderTimeout: time.Minute,
		MaxHeaderBytes:    1 << 20,
	}
	fmt.Printf("Starting RPC Server on " + ipport + "\n")
	go func() {
		log.Println(s.ListenAndServe())
	}()
}

func runNode(setting *setting.Setting) {
	ipport := fmt.Sprintf("%s:%d", setting.Bind, setting.Port)
	tcpAddr, err := net.ResolveTCPAddr("tcp", ipport)
	if err != nil {
		log.Println(err)
		return
	}
	l, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		log.Println(err)
		return
	}
	fmt.Printf("Starting node Server on " + ipport + "\n")
	go func() {
		defer l.Close()
		for {
			conn, err := l.AcceptTCP()
			if err != nil {
				if ne, ok := err.(net.Error); ok {
					if ne.Temporary() {
						log.Println("AcceptTCP", err)
						continue
					}
				}
				log.Fatal(err)
			}
			if err := node.Handle(setting, conn); err != nil {
				if err := conn.Close(); err != nil {
					log.Println(err)
				}
			}
		}
	}()
}
