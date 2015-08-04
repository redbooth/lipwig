// Copyright (c) 2015, Air Computing Inc. <oss@aerofs.com>
// All rights reserved.

package server

import (
	"bytes"
	"crypto/subtle"
	"crypto/tls"
	"github.com/aerofs/lipwig/ssmp"
	"net"
)

// The Authenticator interface is used to accpet or reject LOGIN attempts.
type Authenticator interface {
	// Auth determines whether cred is a valid credential for user in the given
	// authentication scheme.
	// The underlying network connection is provided to allow TLS state to be
	// extracted or challenge/response approaches.
	Auth(c net.Conn, user []byte, scheme []byte, cred []byte) bool

	// Unauthorized return a 401 response with a space-separated list of
	// supported authentication schemes
	Unauthorized() []byte
}

type AuthenticatorFunc func(net.Conn, []byte, []byte, []byte) bool

// MultiSchemeAuthenticator maps authentication schems to corresponding AuthenticatorFunc
type MultiSchemeAuthenticator struct {
	Schemes map[string]AuthenticatorFunc

	unauthorized []byte
}

func (a *MultiSchemeAuthenticator) Auth(c net.Conn, user, scheme, cred []byte) bool {
	f := a.Schemes[string(scheme)]
	return f != nil && f(c, user, scheme, cred)
}

func (a *MultiSchemeAuthenticator) Unauthorized() []byte {
	if a.unauthorized == nil {
		var b bytes.Buffer
		b.WriteString("401")
		for k := range a.Schemes {
			b.WriteByte(' ')
			b.WriteString(k)
		}
		b.WriteByte('\n')
		a.unauthorized = b.Bytes()
	}
	return a.unauthorized
}

func SecretAuth(sharedSecret []byte) AuthenticatorFunc {
	return func(_ net.Conn, _, _, cred []byte) bool {
		return subtle.ConstantTimeCompare(cred, sharedSecret) == 1
	}
}

func CertAuth(c net.Conn, user, _, cred []byte) bool {
	tc, ok := c.(*tls.Conn)
	if !ok {
		return false
	}
	// discard path suffix
	i := bytes.IndexByte(user, '/')
	if i > 1 {
		user = user[0:i]
	}
	s := tc.ConnectionState()
	for _, chain := range s.VerifiedChains {
		cert := chain[0]
		if ssmp.Equal(user, cert.Subject.CommonName) {
			return true
		}
		for _, altName := range cert.DNSNames {
			if ssmp.Equal(user, altName) {
				return true
			}
		}
		for _, altName := range cert.EmailAddresses {
			if ssmp.Equal(user, altName) {
				return true
			}
		}
	}
	return false
}
