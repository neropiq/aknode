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

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/AidosKuneen/aknode/explorer"
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
	var fname string
	flag.BoolVar(&verbose, "verbose", false, "outputs logs to stdout.")
	flag.StringVar(&fname, "fname", "~/.aknode/aknode.json", "setting file path")
	flag.Parse()

	s, err := ioutil.ReadFile(fname)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	setting, err := setting.Load(s)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if !verbose {
		l := &lumberjack.Logger{
			Filename:   filepath.Join(setting.BaseDir(), "aknode.log"),
			MaxSize:    10, // megabytes
			MaxBackups: 10,
			MaxAge:     30 * 3, //days
		}
		log.SetOutput(l)
	}

	onSigs(setting)

	if err := imesh.Init(setting); err != nil {
		fmt.Println(err)
		log.Fatal(err)
	}
	if err := leaves.Init(setting); err != nil {
		fmt.Println(err)
		log.Fatal(err)
	}
	if node.Start(setting); err != nil {
		fmt.Println(err)
		log.Fatal(err)
	}

	if setting.Debug {
		//for pprof
		go func() {
			runtime.SetBlockProfileRate(1)
			log.Println(http.ListenAndServe("127.0.0.1:6061", nil))
		}()
	}

	if setting.RunRPCServer {
		rpc.Run(setting)
	}
	if setting.RunExplorer {
		explorer.Run(setting)
	}
	if setting.RunFeeMiner || setting.RunTicketMiner {
		node.RunMiner(setting)
	}
	<-make(chan struct{})
}
