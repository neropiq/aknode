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
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"golang.org/x/net/proxy"

	"github.com/AidosKuneen/aknode/msg"
	"github.com/AidosKuneen/aknode/setting"
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
		i := 0
		var adrs msg.Addrs
		for _, d := range s.Config.DNS {
			_, addrs, err := net.LookupSRV(d.Service, "tcp", d.Name)
			if err != nil {
				return err
			}
			for _, n := range addrs {
				adr := net.JoinHostPort(n.Target, strconv.Itoa(int(s.Config.DefaultPort)))
				adrs = append(adrs, *msg.NewAddr(adr, msg.ServiceFull))
				i++
			}
		}
		if err := putAddrs(s, adrs...); err != nil {
			return err
		}
		log.Println("found", i, "nodes from DNS")
	}
	return nil
}

func connect(s *setting.Setting) {
	dialer := net.Dial
	if s.Proxy != "" {
		p, err2 := proxy.SOCKS5("tcp", s.Proxy, nil, proxy.Direct)
		if err2 != nil {
			log.Fatal(err2)
		}
		dialer = p.Dial
	}
	for i := 0; i < int(s.MaxConnections); i++ {
		go func(i int) {
			for {
				var p msg.Addr
				found := false
				for _, p = range get(0) {
					if !isConnected(p.Address) {
						found = true
						break
					}
				}
				if !found {
					log.Println("#", i, "no peers found, sleeping")
					time.Sleep(time.Minute)
					continue
				}
				if err := remove(s, p); err != nil {
					log.Println(err)
				}

				conn, err3 := dialer("tcp", p.Address)
				if err3 != nil {
					log.Println(err3)
					continue
				}
				tcpconn, ok := conn.(*net.TCPConn)
				if !ok {
					log.Fatal("invalid connection")
				}
				if err := tcpconn.SetDeadline(time.Now().Add(rwTimeout)); err != nil {
					log.Println(err)
					continue
				}
				if err := writeVersion(s, p, tcpconn, verNonce); err != nil {
					log.Println(err)
					continue
				}
				pr, err3 := readVersion(s, tcpconn, verNonce)
				if err3 != nil {
					log.Println(err3)
					continue
				}
				if err := pr.add(s); err != nil {
					log.Println(err)
					continue
				}
				if err := putAddrs(s, p); err != nil {
					log.Println(err)
				}
				log.Println("#", i, "connected to", p.Address)
				pr.run(s)
			}
		}(i)
	}
}

func start(setting *setting.Setting) (*net.TCPListener, error) {
	ipport := fmt.Sprintf("%s:%d", setting.Bind, setting.Port)
	tcpAddr, err2 := net.ResolveTCPAddr("tcp", ipport)
	if err2 != nil {
		return nil, err2
	}
	l, err2 := net.ListenTCP("tcp", tcpAddr)
	if err2 != nil {
		return nil, err2
	}
	fmt.Printf("Starting node Server on " + ipport + "\n")
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
			go func() {
				log.Println("connected from", conn.RemoteAddr())
				if err := handle(setting, conn); err != nil {
					log.Println(conn.RemoteAddr(), ":", err)
				}
				if err := conn.Close(); err != nil {
					log.Println(err)
				}
			}()
		}
	}()
	goCron(setting)
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
func Start(setting *setting.Setting, debug bool) (net.Listener, error) {
	if err := initDB(setting); err != nil {
		return nil, err
	}
	if !debug {
		if err := lookup(setting); err != nil {
			return nil, err
		}
		connect(setting)
	}
	l, err := start(setting)
	return l, err
}
