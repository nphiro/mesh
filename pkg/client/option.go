package client

import (
	"crypto/tls"
)

type Option func(c *config)

type config struct {
	certificates []tls.Certificate
	headers      map[string]string
	proxy        string
	requestLog   bool
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

func WithHeaders(headers map[string]string) Option {
	return func(c *config) {
		c.headers = headers
	}
}

func WithProxy(proxy string) Option {
	return func(c *config) {
		c.proxy = proxy
	}
}

func WithRequestLog() Option {
	return func(c *config) {
		c.requestLog = true
	}
}
