// Copyright (c) 2015, Air Computing Inc. <oss@aerofs.com>
// All rights reserved.

package main // github.com/aerofs/lipwig

import (
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/aerofs/lipwig/cfg"
	"github.com/aerofs/lipwig/server"
	"io/ioutil"
	"net"
)

func main() {
	var address string
	var insecure bool
	var openLogin bool

	cfg.InitConfig()

	flag.StringVar(&address, "listen", "0.0.0.0:8787", "Listening address")
	flag.BoolVar(&insecure, "insecure", false, "Disable TLS")
	flag.BoolVar(&openLogin, "open", false, "Enable open login")
	flag.Parse()

	auth := &server.MultiSchemeAuthenticator{
		Schemes: map[string]server.AuthenticatorFunc{},
	}

	if openLogin {
		fmt.Println("WARN: open login is enabled")
		auth.Schemes["open"] = func(_ net.Conn, _, _, _ []byte) bool { return true }
	}

	if len(cfg.Secret) > 0 {
		b, err := ioutil.ReadFile(cfg.Secret)
		if err != nil {
			panic(err)
		}
		auth.Schemes["secret"] = server.SecretAuth(bytes.TrimSpace(b))
	}

	l, err := net.Listen("tcp", address)
	if err != nil {
		panic(err)
	}
	var tlsCfg *tls.Config = nil
	if insecure {
		fmt.Println("WARN: TLS is disabled")
	} else {
		tlsCfg = cfg.TLSConfig()
		auth.Schemes["cert"] = server.CertAuth
	}
	s := server.NewServer(l, auth, tlsCfg)
	SetupSignalHandler(s)
	fmt.Println("lipwig serving at", s.ListeningPort())
	err = s.Serve()
	if err != nil {
		panic(err)
	}
	fmt.Println("exit.")
}
