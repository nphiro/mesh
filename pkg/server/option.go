package server

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
)

type Option func(c *config)

type config struct {
	certificates []tls.Certificate
	clientCAs    *x509.CertPool

	healthcheckPath string
	error
}

func WithCertificate(cert, key []byte) Option {
	return func(c *config) {
		certificate, err := tls.X509KeyPair(cert, key)
		if err != nil {
			c.error = err
			return
		}
		c.certificates = []tls.Certificate{certificate}
	}
}

func WithClientAuth(ca []byte) Option {
	return func(c *config) {
		certPool := x509.NewCertPool()
		ok := certPool.AppendCertsFromPEM(ca)
		if !ok {
			c.error = errors.New("tls: failed to parse root certificate")
			return
		}
		c.clientCAs = certPool
	}
}

func WithCustomHealthcheckPath(path string) Option {
	return func(c *config) {
		c.healthcheckPath = path
	}
}
