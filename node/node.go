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

package node

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	"golang.org/x/net/proxy"

	"github.com/AidosKuneen/aknode/akconsensus"
	"github.com/AidosKuneen/aknode/msg"
	"github.com/AidosKuneen/aknode/setting"
	"github.com/AidosKuneen/consensus"
)

func readVersion(s *setting.Setting, conn *net.TCPConn, nonce uint64) (*peer, error) {
	cmd, buf, err := msg.ReadHeader(s, conn)
	if err != nil {
		return nil, err
	}
	if cmd != msg.CmdVersion {
		return nil, errors.New("cmd must be version for handshake")
	}
	v, err := msg.ReadVersion(s, buf, nonce)
	if err != nil {
		return nil, err
	}
	p, err := newPeer(v, conn, s)
	if err != nil {
		return nil, err
	}
	return p, msg.Write(s, nil, msg.CmdVerack, conn)
}

func writeVersion(s *setting.Setting, to msg.Addr, conn *net.TCPConn, nonce uint64) error {
	v := msg.NewVersion(s, to, nonce)
	if err := msg.Write(s, v, msg.CmdVersion, conn); err != nil {
		log.Println(err)
		return err
	}

	cmd, _, err := msg.ReadHeader(s, conn)
	if err != nil {
		return err
	}
	if cmd != msg.CmdVerack {
		return errors.New("message must be verack after Version")
	}
	return nil
}

func lookup(s *setting.Setting) error {
	if len(nodesDB.Addrs) < int(s.MaxConnections) {
		log.Println("looking for DNS...")
		var adrs msg.Addrs
		log.Println("found:")
		for _, d := range s.Config.DNS {
			_, addrs, err := net.LookupSRV(d.Service, "tcp", d.Name)
			if err != nil {
				return err
			}
			for _, n := range addrs {
				adr := net.JoinHostPort(n.Target, strconv.Itoa(int(s.Config.DefaultPort)))
				log.Println(adr)
				adrs = append(adrs, *msg.NewAddr(adr, msg.ServiceFull))
			}
		}
		if err := putAddrs(s, adrs...); err != nil {
			return err
		}
	}
	log.Println("from DNS")
	return nil
}

func connect(ctx context.Context, s *setting.Setting) {
	dialer := net.Dial
	if s.Proxy != "" {
		p, err2 := proxy.SOCKS5("tcp", s.Proxy, nil, proxy.Direct)
		if err2 != nil {
			log.Fatal(err2)
		}
		dialer = p.Dial
	}
	var stop uint32
	go func() {
		ctx2, cancel2 := context.WithCancel(ctx)
		defer cancel2()
		<-ctx2.Done()
		atomic.StoreUint32(&stop, 1)
	}()
	for i := 0; i < int(s.MaxConnections); i++ {
		go func(i int) {
			for atomic.LoadUint32(&stop) == 0 {
				errr := func() error {
					var p msg.Addr
					found := false
					for _, p = range get(0) {
						if !isConnected(p.Address) {
							found = true
							break
						}
					}
					if !found {
						log.Println("no other peers found from peer", i, ", sleeping")
						time.Sleep(time.Minute)
						return nil
					}
					if err := remove(s, p); err != nil {
						log.Println(err)
					}

					conn, err3 := dialer("tcp", p.Address)
					if err3 != nil {
						return err3
					}
					tcpconn, ok := conn.(*net.TCPConn)
					if !ok {
						log.Fatal("invalid connection")
					}
					ctx2, cancel2 := context.WithCancel(ctx)
					defer cancel2()
					go func() {
						<-ctx2.Done()
						if err := tcpconn.Close(); err != nil {
							log.Println(err)
						}
					}()
					if err := tcpconn.SetDeadline(time.Now().Add(rwTimeout)); err != nil {
						return err
					}
					if err := writeVersion(s, p, tcpconn, verNonce); err != nil {
						return err
					}
					pr, err3 := readVersion(s, tcpconn, verNonce)
					if err3 != nil {
						return err3
					}
					if err := pr.add(s); err != nil {
						return err
					}
					if err := putAddrs(s, p); err != nil {
						log.Println(err)
					}
					log.Println("connected to", p.Address)
					pr.run(s)
					return nil
				}()
				if errr != nil {
					log.Println(errr)
				}
			}
		}(i)
	}
}

func start(ctx context.Context, setting *setting.Setting) (*net.TCPListener, error) {

	ipport := fmt.Sprintf("%s:%d", setting.Bind, setting.Port)
	tcpAddr, err2 := net.ResolveTCPAddr("tcp", ipport)
	if err2 != nil {
		return nil, err2
	}
	l, err2 := net.ListenTCP("tcp", tcpAddr)
	if err2 != nil {
		return nil, err2
	}
	go func() {
		ctx2, cancel2 := context.WithCancel(ctx)
		defer cancel2()
		<-ctx2.Done()
		if err := l.Close(); err != nil {
			log.Println(err)
		}
	}()
	log.Printf("Starting node Server on " + ipport + "\n")
	go func() {
		defer func() {
			if err := l.Close(); err != nil {
				log.Println(err)
			}
		}()
		for {
			conn, err3 := l.AcceptTCP()
			if err3 != nil {
				if ne, ok := err3.(net.Error); ok {
					if ne.Temporary() {
						log.Println("failed to accept:", err3)
						continue
					}
				}
				log.Println(err3)
				return
			}
			ctx2, cancel2 := context.WithCancel(ctx)
			go func() {
				defer cancel2()
				<-ctx2.Done()
				if err := conn.Close(); err != nil {
					log.Println(err)
				}
			}()
			go func() {
				defer cancel2()
				log.Println("connected from", conn.RemoteAddr())
				if err := handle(setting, conn); err != nil {
					log.Println(conn.RemoteAddr(), ":", err)
				}
			}()
		}
	}()

	goCron(ctx, setting)
	return l, nil
}

//Handle handles messages in tcp.
func handle(s *setting.Setting, conn *net.TCPConn) error {
	var err2 error
	if err := conn.SetDeadline(time.Now().Add(rwTimeout)); err != nil {
		return err
	}
	p, err2 := readVersion(s, conn, verNonce)
	if err2 != nil {
		log.Println(err2)
		return err2
	}

	if err := writeVersion(s, p.remote, conn, verNonce); err != nil {
		return err
	}

	if err := p.add(s); err != nil {
		return err
	}
	if err := putAddrs(s, p.remote); err != nil {
		return err
	}
	p.run(s)
	return nil
}

//Start starts a node server.
func Start(ctx context.Context, setting *setting.Setting, debug bool) (net.Listener, error) {
	if err := initDB(setting); err != nil {
		return nil, err
	}
	if !debug {
		if err := lookup(setting); err != nil {
			return nil, err
		}
		connect(ctx, setting)
		// if err := startConsensus(ctx, setting); err != nil {
		// 	return nil, err
		// }
	}
	l, err := start(ctx, setting)
	return l, err
}

func startConsensus(ctx context.Context, setting *setting.Setting) error {
	if err := akconsensus.Init(ctx, setting, &ConsensusPeer{}); err != nil {
		return err
	}
	var id consensus.NodeID
	if setting.RunValidator && setting.ValidatorSecret != "" {
		adr, err := setting.ValidatorAddress()
		if err != nil {
			return err
		}
		copy(id[:], adr.Address(setting.Config)[2:])
	}
	unl, err := setting.TrustedNodeIDs()
	if err != nil {
		return err
	}
	peers.cons = consensus.NewPeer(akconsensus.NewAdaptor(setting), id,
		unl, setting.RunValidator)
	peers.cons.Start(ctx)
	return nil
}
