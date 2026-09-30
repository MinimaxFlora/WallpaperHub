package acme

import (
	"context"
	"testing"

	"wallpaper-api/internal/config"
)

func TestHostPolicyAllowsConfiguredDomains(t *testing.T) {
	m := NewManager(config.Config{
		Domains:      []string{"wall.example.com", "cdn.example.com"},
		ACMECacheDir: t.TempDir(),
	})
	ctx := context.Background()
	for _, host := range []string{"wall.example.com", "cdn.example.com"} {
		if err := m.HostPolicy(ctx, host); err != nil {
			t.Fatalf("HostPolicy(%q) = %v, want nil", host, err)
		}
	}
	if err := m.HostPolicy(ctx, "evil.example.com"); err == nil {
		t.Fatal("HostPolicy accepted an unconfigured host")
	}
}

func TestStagingUsesStagingDirectory(t *testing.T) {
	m := NewManager(config.Config{
		Domains:      []string{"wall.example.com"},
		ACMECacheDir: t.TempDir(),
		ACMEStaging:  true,
	})
	if m.Client == nil || m.Client.DirectoryURL != letsEncryptStaging {
		t.Fatalf("staging directory = %+v", m.Client)
	}
}

func TestProductionUsesDefaultDirectory(t *testing.T) {
	m := NewManager(config.Config{
		Domains:      []string{"wall.example.com"},
		ACMECacheDir: t.TempDir(),
	})
	if m.Client != nil {
		t.Fatalf("production client = %+v, want nil (library default)", m.Client)
	}
}

func TestTLSConfigEnablesHTTP2AndALPN(t *testing.T) {
	m := NewManager(config.Config{
		Domains:      []string{"wall.example.com"},
		ACMECacheDir: t.TempDir(),
	})
	tlsCfg := m.TLSConfig()
	hasH2, hasALPN := false, false
	for _, proto := range tlsCfg.NextProtos {
		switch proto {
		case "h2":
			hasH2 = true
		case "acme-tls/1":
			hasALPN = true
		}
	}
	if !hasH2 || !hasALPN {
		t.Fatalf("NextProtos = %v, want h2 and acme-tls/1", tlsCfg.NextProtos)
	}
	if tlsCfg.GetCertificate == nil {
		t.Fatal("GetCertificate is nil")
	}
}
