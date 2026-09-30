// Package acme wraps golang.org/x/crypto/acme/autocert so the service can
// obtain and renew Let's Encrypt certificates automatically when a domain is
// configured.
package acme

import (
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"

	"wallpaper-api/internal/config"
)

// letsEncryptStaging is the staging directory URL, used to avoid hitting rate
// limits while testing.
const letsEncryptStaging = "https://acme-staging-v02.api.letsencrypt.org/directory"

// NewManager builds an autocert manager for the configured domains.
func NewManager(cfg config.Config) *autocert.Manager {
	m := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(cfg.Domains...),
		Cache:      autocert.DirCache(cfg.ACMECacheDir),
		Email:      cfg.ACMEEmail,
	}
	if cfg.ACMEStaging {
		m.Client = &acme.Client{DirectoryURL: letsEncryptStaging}
	}
	return m
}
