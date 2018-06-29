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

package netadr

import (
	"bytes"
	"net"
	"strconv"
	"strings"
)

//NetAdr represents address with tcp/ip or tor type.
type NetAdr struct {
	IsTor   bool
	Address []byte
	Port    int
}

//New parses adr and returns NetAdr struct.
func New(adr string) ([]*NetAdr, error) {
	if strings.HasSuffix(adr, ".onion") {
		return []*NetAdr{
			&NetAdr{
				IsTor:   true,
				Address: []byte(adr),
			},
		}, nil
	}
	ips, err := ParseTCPAddrsses(adr)
	if err != nil {
		return nil, err
	}
	adrs := make([]*NetAdr, len(ips))
	for i, ip := range ips {
		adrs[i] = &NetAdr{
			IsTor:   false,
			Address: ip.IP,
			Port:    ip.Port,
		}
	}
	return adrs, nil
}

func (na *NetAdr) String() string {
	if na.IsTor {
		return string(na.Address)
	}
	ta := net.TCPAddr{
		IP:   na.Address,
		Port: na.Port,
	}
	return ta.String()
}

//Network returns a name of the network
func (na *NetAdr) Network() string {
	if !na.IsTor {
		return "tcp"
	}
	return "tor"
}

//Equal returns true if address of na == address of na2.
func (na *NetAdr) Equal(na2 *NetAdr) bool {
	if na.IsTor != na2.IsTor {
		return false
	}
	if na.IsTor {
		return bytes.Equal(na.Address, na2.Address)
	}
	return net.IP(na.Address).Equal(net.IP(na.Address))
}

//ParseIPs parses ip address string and returns slices of net.IP.
func ParseIPs(adrs ...string) ([]net.IP, error) {
	var ips []net.IP
	for _, b := range adrs {
		lips, err := net.LookupIP(b)
		if err != nil {
			return nil, err
		}
		ips = append(ips, lips...)
	}
	return ips, nil
}

//ParseTCPAddrsses  parses ip:port  string and returns slices of net.TCPAddr.
func ParseTCPAddrsses(adrs ...string) ([]*net.TCPAddr, error) {
	var tcps []*net.TCPAddr
	for _, b := range adrs {
		h, p, err := net.SplitHostPort(b)
		if err != nil {
			return nil, err
		}
		ips, err := ParseIPs(h)
		if err != nil {
			return nil, err
		}
		port, err := strconv.Atoi(p)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			tcps = append(tcps, &net.TCPAddr{
				IP:   ip,
				Port: port,
			})
		}
	}
	return tcps, nil
}
