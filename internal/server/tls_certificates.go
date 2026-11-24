package server

import (
	"crypto/tls"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/Fallen-Breath/pavonis/internal/config"
	log "github.com/sirupsen/logrus"
)

type tlsCertificates struct {
	files   []config.TLSCertificateConfig
	current atomic.Pointer[[]tls.Certificate]
}

func newTLSCertificates(files []config.TLSCertificateConfig) (*tlsCertificates, error) {
	c := &tlsCertificates{files: files}
	if err := c.reload(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *tlsCertificates) reload() error {
	previous := c.current.Load()
	next := make([]tls.Certificate, len(c.files))
	for i, file := range c.files {
		pair, err := tls.LoadX509KeyPair(file.CertFile, file.KeyFile)
		if err != nil {
			err = fmt.Errorf("load TLS certificate %q and key %q: %w", file.CertFile, file.KeyFile, err)
			if previous == nil {
				return err
			}
			log.Error(err)
			next[i] = (*previous)[i]
			continue
		}
		next[i] = pair
	}
	c.current.Store(&next)
	return nil
}

func (c *tlsCertificates) reloadLoop(stop <-chan struct{}) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.reload()
		case <-stop:
			return
		}
	}
}

func (c *tlsCertificates) selectCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	// With no static Certificates, (nil, nil) makes Go send unrecognized_name.
	if hello.ServerName == "" {
		return nil, nil
	}
	certs := *c.current.Load()
	for i := range certs {
		if hello.SupportsCertificate(&certs[i]) == nil {
			return &certs[i], nil
		}
	}
	return nil, nil
}
