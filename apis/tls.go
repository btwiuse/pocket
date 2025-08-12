package apis

import (
	"crypto/tls"

	"github.com/webteleport/utils"
)

var (
	HOST = utils.EnvHost("localhost")
	CERT = utils.EnvCert("localhost.pem")
	KEY  = utils.EnvKey("localhost-key.pem")
)

// disable HTTP/2, because http.Hijacker is not supported,
// which is required by https://github.com/elazarl/goproxy
var NextProtos = []string{"http/1.1"}

func LocalTLSConfig(certFile, keyFile string) *tls.Config {
	GetCertificate := func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		// Always get latest localhost.crt and localhost.key
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, err
		}
		return &cert, nil
	}
	return &tls.Config{
		GetCertificate: GetCertificate,
		NextProtos:     NextProtos,
	}
}
