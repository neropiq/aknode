[![GoDoc](https://godoc.org/github.com/AidosKuneen/aknode?status.svg)](https://godoc.org/github.com/AidosKuneen/aknode)
[![Build Status](https://travis-ci.org/AidosKuneen/aknode.svg?branch=master)](https://travis-ci.org/AidosKuneen/aknode)
[![GitHub license](https://img.shields.io/badge/license-MIT-blue.svg)](https://raw.githubusercontent.com/AidosKuneen/aknode/LICENSE)
[![Coverage Status](https://coveralls.io/repos/github/AidosKuneen/aknode/badge.svg?branch=master)](https://coveralls.io/github/AidosKuneen/aknode?branch=master)
[![GolangCI](https://golangci.com/badges/github.com/AidosKuneen/aknode.svg)](https://golangci.com/r/github.com/AidosKuneen/aknode) 


# aknode

## Overview

This is the node server for Aidos Kuneen for step2, 
including:

* Messaging between nodes (Testing)
* Consensus / Validator (Testing)
* Aidos Explorer (Testing)
* RPC APIs with Wallet Service (Testing)
* One time address (Not Yet)
* One time ring signature / Confidential transaction (Not planned for Step2)

UNDER CONSTRUCTION. DON'T TOUCH ME.

![Plan](https://i.imgur.com/63Uv8FA.png)

## Requirements

* git
* go 1.9+

are required to compile.

## Installation

     $ go get github.com/AidosKuneen/aknode


## How to Run

1. Setup aknode.json
2. $ cd github.com/AidosKuneen/aknode/cmd/aknode
2. $ go run main.go -config aknode.json

## aknode.json

| key | default | description |
| --- | ------- | ------------|
|debug| false |setup for debug info (memory usage etc)|
|    testnet| 0|0:mainnet  1:testnet 2:debugnet|
|    blacklists|[] |node IPs which should be banned|
|    root_dir| $HOME/.aknode |root directory data will be stored|
|    my_host_port|remote address:port in TCP/IP packet |hostname and port repoted when connected from (connects to) remote node. required if your node is behind firewall.|
|    default_nodes|[] |nodes which are connected from start|
|   bind|"0.0.0.0"|bind address for listening node|
|    port|mainnet:14270, testnet:14370|port number for listening node|
|    max_connections|5 |umber of max connections for node|
|    proxy|""|proxy ussed when connecting nodes|
 |   use_public_rpc |false |open public RPCs|
 |   rpc_bind| "localhost" |bind address for listening RPC|
 |   rpc_port| mainnet:14271, testnet: 14371|port number for listening RPC|
 |   rpc_user|"" |rpc user name, required if you use wallet|
|    rpc_password|""|rpc password, required if you use wallet|
 |   rpc_max_connections|1|umber of max connections for RPC|
 |   rpc_tx_tag|"" |Tag of transactions sent from aknode|
 |   rpc_allow_public_pow`|false, |if allow PoW from remote|
|    wallet_notify|"" |the comand when a tx comes into wallet|
|    run_validator| false, |run validator node|
 |   validator_secret |"" | secret key for validator, required if run_validator:true|
|    trusted_nodes |[],| trusted node for consensus|
|    run_explorer |false |run explorer|
 |   explorer_bind|"localhost"|bind address for listening explorer|
|    explorer_port|mainnet:8080, testnet:8081|port number for listening explorer|
 |   explorer_max_connections|1 |number of max connections for explorer|
 |   run_fee_miner|false|run miner node for fee|
 |   run_ticket_miner|false|run miner node for ticket|
 |   run_ticket_issuer|false|run miner node for issuing miner|
 |   miner_address| ""|address of miner, required if run_*_miner :true |






example:
```json
{
    "debug": true,
    "testnet": 2,
    "use_public_rpc": true,
    "rpc_user":"tester",
    "rpc_password":"test",
    "wallet_notify":"echo %s",
    "trusted_nodes": [
        "AKNODED2dKTb5ZX2tvhtPXEX11EbEAN8TXtDnPB1ZFmNthXERPRuhtrJ6",
        "AKNODED3q22XMC1t75NbPBpqYxoVejTE5vdmXtCWH3zPzMz2B2DbbuZCt",
        "AKNODED3PDQB8zLbui446oG9qKHYA54jpxKXV6AaFVpY2gvoukoNJ9T71",
        "AKNODED3EZKtf7KE7EjnvQUEVSAcdcZpK93j35WJSMTJAL2JKFMdP6mwR",
        "AKNODED3hWKVimUPTAbBGQBviLUqzS8d3CpznTihzwtgo3zVjKxTWuKSa",
        "AKNODED2S1mE6Vx54SKmEKy3ZLca13u8CNipVMthi42MZ2VnXrTqjBKQP",
        "AKNODED42BxxYvFvCsabih3xCCAFWftB7oMdMZkPWWx6X5wMb3ubTAHRJ"
    ],
    "run_explorer": true,
    "run_fee_miner":true,
    "run_ticket_miner":true,
    "run_ticket_issuer":true,
    "miner_address":"AKADRSD4HSext48uT6cibWYoQQj6pvtVgLkQo61wbiToSKfeSi2inrMUU"
}
```


## Contribution
Improvements to the codebase and pull requests are encouraged.



## Dependencies and Licenses

This software includes the work that is distributed in the Apache License 2.0.

```
github.com/AidosKuneen/aklib                              MIT License
github.com/AidosKuneen/aknode                             MIT License
github.com/AidosKuneen/cuckoo                             MIT License
github.com/AidosKuneen/glyph                              MIT License
github.com/AidosKuneen/numcpu                             MIT License
github.com/AndreasBriese/bbloom                           MIT License, Public Domain
github.com/alecthomas/template/parse                      BSD 3-clause "New" or "Revised" License 
github.com/blang/semver                                   MIT License
github.com/dgraph-io/badger                               Apache License 2.0 
github.com/dgryski/go-farm                                MIT License
github.com/gobuffalo/packr                                MIT License
github.com/golang/protobuf/proto                          BSD 3-clause "New" or "Revised" License 
github.com/google/go-github/github                        BSD 3-clause "New" or "Revised" License 
github.com/google/go-querystring/query                    BSD 3-clause "New" or "Revised" License 
github.com/inconshreveable/go-update                      Apache License 2.0
github.com/inconshreveable/go-update/internal/binarydist  MIT License
github.com/inconshreveable/go-update/internal/osext       BSD 3-clause "New" or "Revised" License 
github.com/mattn/go-shellwords                            MIT License
github.com/natefinch/lumberjack                           MIT License
github.com/pkg/errors                                     BSD 2-clause "Simplified" License
github.com/rhysd/go-github-selfupdate/selfupdate          MIT License
github.com/skip2/go-qrcode                                MIT License
github.com/tcnksm/go-gitconfig                            MIT License
github.com/ulikunitz/xz                                   BSD 3-clause "New" or "Revised" License 
github.com/vmihailenco/msgpack/codes                      BSD 2-clause "Simplified" License
golang.org/x/crypto/ssh/terminal                          BSD 3-clause "New" or "Revised" License 
golang.org/x/net                                          BSD 3-clause "New" or "Revised" License 
golang.org/x/oauth2/internal                              BSD 3-clause "New" or "Revised" License 
golang.org/x/sys/unix                                     BSD 3-clause "New" or "Revised" License
Golang Standard Library                                   BSD 3-clause License
```