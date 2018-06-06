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

	"github.com/AidosKuneen/aknode/msg"
	"github.com/AidosKuneen/aknode/setting"
	"github.com/vmihailenco/msgpack"
)

var (
	dns = []string{"peer1.aidoskuneen.com"}
)

func readVersion(s *setting.Setting, conn *net.TCPConn) (*Peer, error) {
	cmd, buf, err := msg.ReadHeader(s, conn)
	if err != nil {
		return nil, err
	}
	if cmd != msg.CmdVersion {
		return nil, errors.New("cmd must be version for handshake")
	}
	v, err := msg.ReadVersion(buf)
	if err != nil {
		return nil, err
	}
	p, err := NewPeer(v, conn, s)
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
	enc := msgpack.NewEncoder(conn).StructAsArray(true)
	if err := enc.Encode(v); err != nil {
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

//Bootstrap  connects to peers.
func Bootstrap(s *setting.Setting) error {
	if Size() < int(s.MaxConnections) {
		var adrs msg.Addrs
		for _, d := range dns {
			ns, err := net.LookupHost(d)
			if err != nil {
				return err
			}
			for _, n := range ns {
				adr := msg.Addr{
					Address: n,
					Port:    s.Config.DefaultPort,
				}
				adrs = append(adrs, adr)
			}
			if err := Put(s, adrs); err != nil {
				return err
			}
		}
	}
	for i := 0; i < int(s.MaxConnections); i++ {
		go func() {
			for {
				ps := Get(1)
				if len(ps) == 0 {
					time.Sleep(time.Minute)
					continue
				}
				p := ps[0]
				to := p.Address + ":" + strconv.Itoa(int(p.Port))
				conn, err := net.Dial("tcp", to)
				if err != nil {
					log.Println(err)
					continue
				}
				tcpconn, ok := conn.(*net.TCPConn)
				if !ok {
					log.Println("invalid connection")
					continue
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
				if err := pr.Add(); err != nil {
					log.Println(err)
					continue
				}
				pr.Run(s)
			}
		}()
	}
	return nil
}

//Run runs a node server.
func Run(setting *setting.Setting) {
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
			if err := handle(setting, conn); err != nil {
				if err := conn.Close(); err != nil {
					log.Println(err)
				}
			}
		}
	}()
}

//Handle handles messages in tcp.
func handle(s *setting.Setting, conn *net.TCPConn) error {
	var err error
	if err := conn.SetDeadline(time.Now().Add(rwTimeout)); err != nil {
		log.Println(err)
		return err
	}
	p, err := readVersion(s, conn)
	if err != nil {
		log.Println(err)
		return err
	}
	if err := writeVersion(s, p.from, conn); err != nil {
		log.Println(err)
		return err
	}
	if err := p.Add(); err != nil {
		log.Println(err)
		return err
	}
	go p.Run(s)
	return nil
}
