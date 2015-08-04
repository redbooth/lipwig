// Copyright (c) 2015, Air Computing Inc. <oss@aerofs.com>
// All rights reserved.

// +build !aero

package cfg

import (
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
)

var hostname string
var cacertFile string
var certFile string
var keyFile string

var Secret string

func InitConfig() {
	flag.StringVar(&Secret, "secret", "", "Path to shared secret")
	flag.StringVar(&hostname, "host", "", "TLS hostname")
	flag.StringVar(&cacertFile, "cacert", "", "Path to CA certificate")
	flag.StringVar(&certFile, "cert", "", "Path to server certificate")
	flag.StringVar(&keyFile, "key", "", "Path to server private key")
}

var errInvalidCert = fmt.Errorf("invalid cert")

func certFromFile(file string) (*x509.Certificate, error) {
	d, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	b, _ := pem.Decode(d)
	if b == nil || b.Type != "CERTIFICATE" {
		return nil, errInvalidCert
	}
	return x509.ParseCertificate(b.Bytes)
}

// NB: uses global variables initialized from command line flags
func TLSConfig() *tls.Config {
	if len(hostname) == 0 || len(cacertFile) == 0 ||
		len(certFile) == 0 || len(keyFile) == 0 {
		flag.Usage()
		os.Exit(1)
	}
	tls, err := LoadTLSConfig(keyFile, certFile, cacertFile)
	if err != nil {
		flag.Usage()
		os.Exit(1)
	}
	tls.ServerName = hostname
	return tls
}

func LoadTLSConfig(keyFile, certFile, cacertFile string) (*tls.Config, error) {
	cacert, err := certFromFile(cacertFile)
	if err != nil {
		return nil, err
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	x509, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, err
	}
	return NewTLSConfig(cert.PrivateKey, x509, cacert), nil
}

func NewTLSConfig(key crypto.PrivateKey, cert *x509.Certificate, cacert *x509.Certificate) *tls.Config {
	roots := x509.NewCertPool()
	roots.AddCert(cacert)
	// lock down TLS config to 1.2 or higher and safest available ciphersuites
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		},
		RootCAs: roots,
		Certificates: []tls.Certificate{
			tls.Certificate{
				Certificate: [][]byte{
					cert.Raw,
					cacert.Raw,
				},
				PrivateKey: key,
				Leaf:       cert,
			},
		},
		ClientAuth: tls.VerifyClientCertIfGiven,
		ClientCAs:  roots,
	}
}
