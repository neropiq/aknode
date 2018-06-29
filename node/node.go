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

func readVersion(s *setting.Setting, conn *net.TCPConn) (*peer, error) {
	cmd, buf, err := msg.ReadHeader(s, conn)
	if err != nil {
		return nil, err
	}
	if cmd != msg.CmdVersion {
		return nil, errors.New("cmd must be version for handshake")
	}
	v, err := msg.ReadVersion(s, buf)
	if err != nil {
		return nil, err
	}
	p, err := newPeer(v, conn, s)
	if err != nil {
		return nil, err
	}
	return p, msg.Write(s, nil, msg.CmdVerack, conn)
}

func writeVersion(s *setting.Setting, to msg.Addr, conn *net.TCPConn) error {
	v := msg.NewVersion(s, to)
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
		var adrs msg.Addrs
		for _, d := range s.Config.DNS {
			_, addrs, err := net.LookupSRV(d.Service, "tcp", d.Name)
			if err != nil {
				return err
			}
			for _, n := range addrs {
				adr := net.JoinHostPort(n.Target, strconv.Itoa(int(s.Config.DefaultPort)))
				adrs = append(adrs, *msg.NewAddr(adr, msg.ServiceFull))
			}
			if err := putAddrs(s, adrs...); err != nil {
				return err
			}
		}
	}
	return nil
}

func connect(s *setting.Setting) {
	dialer := net.Dial
	if s.Proxy != "" {
		p, err := proxy.SOCKS5("tcp", s.Proxy, nil, proxy.Direct)
		if err != nil {
			log.Fatal(err)
		}
		dialer = p.Dial
	}
	for i := 0; i < int(s.MaxConnections); i++ {
		go func() {
			for {
				ps := get(1)
				if len(ps) == 0 {
					time.Sleep(time.Minute)
					continue
				}
				p := ps[0]
				if err := remove(s, p); err != nil {
					log.Println(err)
				}

				conn, err := dialer("tcp", p.Address)
				if err != nil {
					log.Println(err)
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
				if err := writeVersion(s, p, tcpconn); err != nil {
					log.Println(err)
					continue
				}
				pr, err := readVersion(s, tcpconn)
				if err != nil {
					log.Println(err)
					continue
				}
				if err := pr.add(); err != nil {
					log.Println(err)
					continue
				}
				if err := putAddrs(s, p); err != nil {
					log.Println(err)
				}
				pr.run(s)
			}
		}()
	}
}

func start(setting *setting.Setting) (*net.TCPListener, error) {
	ipport := fmt.Sprintf("%s:%d", setting.Bind, setting.Port)
	tcpAddr, err := net.ResolveTCPAddr("tcp", ipport)
	if err != nil {
		return nil, err
	}
	l, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return nil, err
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
				log.Println(err)
				return
			}
			go func() {
				log.Println("handling")
				if err := handle(setting, conn); err != nil {
					log.Println(err)
				}
				log.Println("closing")
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
	var err error
	if err := conn.SetDeadline(time.Now().Add(rwTimeout)); err != nil {
		return err
	}
	p, err := readVersion(s, conn)
	if err != nil {
		log.Println(err)
		return err
	}

	if err := writeVersion(s, p.remote, conn); err != nil {
		return err
	}

	if err := p.add(); err != nil {
		return err
	}
	if err := putAddrs(s, p.remote); err != nil {
		return err
	}
	p.run(s)
	return nil
}

//Start starts a node server.
func Start(setting *setting.Setting) error {
	if err := initDB(setting); err != nil {
		return err
	}
	if err := lookup(setting); err != nil {
		return err
	}
	connect(setting)
	_, err := start(setting)
	return err
}
