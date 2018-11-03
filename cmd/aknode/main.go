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
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/AidosKuneen/aklib/db"

	"github.com/AidosKuneen/aklib/address"

	"github.com/AidosKuneen/aklib/updater"
	"github.com/AidosKuneen/aknode/akconsensus"
	"github.com/AidosKuneen/aknode/explorer"
	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/imesh/leaves"
	"github.com/AidosKuneen/aknode/node"
	"github.com/AidosKuneen/aknode/rpc"
	"github.com/AidosKuneen/aknode/setting"

	"golang.org/x/crypto/ssh/terminal"

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
			log.Println("received SIGHUP")
			se.Stop <- struct{}{}

		// kill -SIGINT XXXX or Ctrl+c
		case syscall.SIGINT:
			log.Println("received SIGINIT")
			se.Stop <- struct{}{}

		case syscall.SIGTERM:
			log.Println("received SIGTERM")
			se.Stop <- struct{}{}

		case syscall.SIGQUIT:
			log.Println("received SIGQIOT")
			se.Stop <- struct{}{}

		default:
			log.Println("Unknown signal.")
			se.Stop <- struct{}{}
		}
	}()
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	usr, err2 := user.Current()
	if err2 != nil {
		fmt.Println(err2)
		os.Exit(1)
	}
	defaultpath := filepath.Join(usr.HomeDir, ".aknode", "aknode.json")
	var verbose, update, genkey, genaddress bool
	var fname string
	flag.BoolVar(&verbose, "verbose", false, "outputs logs to stdout.")
	flag.BoolVar(&update, "update", false, "check for update")
	flag.BoolVar(&genkey, "genkey", false, "generate a validator key")
	flag.BoolVar(&genaddress, "genaddress", false, "generate a random address")
	flag.StringVar(&fname, "config", defaultpath, "setting file path")
	flag.Parse()

	if update {
		if err := updater.Update("AidosKuneen/aknode", setting.Version); err != nil {
			log.Fatal(err)
		}
		return
	}
	info, err := os.Stat(fname)
	if err != nil {
		log.Fatal(err)
	}
	m := info.Mode()
	if m&0177 != 0 {
		fmt.Println("setting file must not be readable by others (must be permission 0600) for security")
		os.Exit(1)
	}

	f, err2 := ioutil.ReadFile(fname)
	if err2 != nil {
		fmt.Println(err2)
		f = []byte("{}")
	}
	if genaddress {
		setting, err2 := setting.Load(f, true)
		if err2 != nil {
			fmt.Println(err2, "in setting file")
			os.Exit(1)
		}
		seed := address.GenerateSeed32()
		akseed := address.HDSeed58(setting.Config, seed, []byte(""), false)
		seed2 := address.HDseed(seed, 0, 0)
		pub, err := address.New(setting.Config, seed2)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println("privatekey is", akseed)
		fmt.Println("address is", pub.Address58(setting.Config))
		fmt.Println("Keep the privatekey secret.")
		return
	}
	if genkey {
		setting, err2 := setting.Load(f, true)
		if err2 != nil {
			fmt.Println(err2, "in setting file")
			os.Exit(1)
		}
		seed := address.GenerateSeed32()
		akseed := address.HDSeed58(setting.Config, seed, []byte(""), true)
		pub, err := address.NewNode(setting.Config, seed)
		if err != nil {
			log.Println(err)
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println("Your validator secret key is", akseed)
		fmt.Println("Your validator pulibic key is", pub.Address58(setting.Config))
		fmt.Println("Keep the secret key secret.")
		return
	}
	setting, err2 := setting.Load(f, false)
	if err2 != nil {
		fmt.Println(err2, "in setting file")
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := initialize(ctx, setting); err != nil {
		fmt.Println(err)
		log.Fatal(err)
	}

	<-setting.Stop
	fmt.Println("stopping aknode...")
	time.Sleep(3 * time.Second)
	cancel()
	time.Sleep(3 * time.Second)
	if err := setting.DB.Close(); err != nil {
		log.Println(err)
	}
	log.Println("aknode was stopped")
}

func checkWalletSeed(s *setting.Setting) error {
	if rpc.IsSecretEmpty() {
		fmt.Print("This is the first time you run RPC. Enter walletpassphrase...")
		pwd, err := terminal.ReadPassword(int(syscall.Stdin)) //int conversion is needed for win
		fmt.Println("")
		if err != nil {
			return err
		}
		if err := rpc.New(s, pwd); err != nil {
			return err
		}
	}
	return nil
}

func initialize(ctx context.Context, setting *setting.Setting) error {
	db.GoGC(ctx, setting.DB)
	if err := imesh.Init(setting); err != nil {
		return err
	}
	if err := leaves.Init(setting); err != nil {
		return err
	}
	if _, err := node.Start(ctx, setting, false); err != nil {
		return err
	}

	if setting.Debug {
		//for pprof
		srv := &http.Server{Addr: "127.0.0.1:6061"}
		go func() {
			runtime.SetBlockProfileRate(1)
			log.Println(srv.ListenAndServe())
		}()
		go func() {
			ctx2, cancel2 := context.WithCancel(ctx)
			defer cancel2()
			<-ctx2.Done()
			if err := srv.Shutdown(ctx2); err != nil {
				log.Print(err)
			}
		}()
	}
	if err := rpc.Init(setting); err != nil {
		return err
	}

	rpc.GoNotify(ctx, setting, node.RegisterTxNotifier, akconsensus.RegisterTxNotifier)

	if setting.RPCUser != "" {
		if err := checkWalletSeed(setting); err != nil {
			return err
		}
	}
	if setting.UsePublicRPC || setting.RPCUser != "" {
		rpc.Run(ctx, setting)
	}
	if setting.RunExplorer {
		explorer.Run(ctx, setting)
	}
	if setting.RunFeeMiner || setting.RunTicketMiner {
		node.RunMiner(ctx, setting)
	}
	return nil
}
