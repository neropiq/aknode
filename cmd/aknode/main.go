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
	"bufio"
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

	"github.com/AidosKuneen/aknode/consensus"

	"github.com/AidosKuneen/aklib/updater"
	"github.com/AidosKuneen/aknode/explorer"
	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/imesh/leaves"
	"github.com/AidosKuneen/aknode/node"
	"github.com/AidosKuneen/aknode/rpc"
	"github.com/AidosKuneen/aknode/setting"
	"github.com/blang/semver"
	"github.com/rhysd/go-github-selfupdate/selfupdate"

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
	var verbose, update bool
	var fname string
	flag.BoolVar(&verbose, "verbose", false, "outputs logs to stdout.")
	flag.BoolVar(&update, "update", false, "check for update")
	flag.StringVar(&fname, "config", defaultpath, "setting file path")
	flag.Parse()

	if update {
		if err := updater.Update("AidosKuneen/akwallet", setting.Version); err != nil {
			log.Fatal(err)
		}
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
	setting, err2 := setting.Load(f)
	if err2 != nil {
		fmt.Println(err2)
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
	if err := initialize(setting); err != nil {
		fmt.Println(err)
		log.Fatal(err)
	}

	<-setting.Stop
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
		if err := rpc.InitSecret(s, pwd); err != nil {
			return err
		}
	}
	return nil
}

func initialize(setting *setting.Setting) error {
	if err := imesh.Init(setting); err != nil {
		return err
	}
	if err := leaves.Init(setting); err != nil {
		return err
	}
	if _, err := node.Start(setting, false); err != nil {
		return err
	}

	if setting.Debug {
		//for pprof
		go func() {
			runtime.SetBlockProfileRate(1)
			log.Println(http.ListenAndServe("127.0.0.1:6061", nil))
		}()
	}

	if err := rpc.Init(setting); err != nil {
		return err
	}

	rpc.GoNotify(setting, node.RegisterTxNotifier, consensus.RegisterTxNotifier)

	if setting.RPCUser != "" {
		if err := checkWalletSeed(setting); err != nil {
			return err
		}
	}
	if setting.UsePublicRPC || setting.RPCUser != "" {
		rpc.Run(setting)
	}
	if setting.RunExplorer {
		explorer.Run(setting)
	}
	if setting.RunFeeMiner || setting.RunTicketMiner {
		node.RunMiner(setting)
	}
	return nil
}

func selfUpdate() error {
	latest, found, err := selfupdate.DetectLatest("AidosKuneen/aknode")
	if err != nil {
		return err
	}

	v := semver.MustParse(setting.Version)
	if !found || latest.Version.Equals(v) {
		log.Println("Current version is the latest")
		return nil
	}

	for ok := false; !ok; {
		fmt.Print("Do you want to update to", latest.Version, "? (y/n): ")
		input, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return err
		}
		switch input {
		case "y\n":
			ok = true
		case "n\n":
			return nil
		default:
			fmt.Println("Invalid input")
		}
	}

	if err := selfupdate.UpdateTo(latest.AssetURL, os.Args[0]); err != nil {
		return err
	}
	log.Println("Successfully updated to version", latest.Version)
	return nil
}
