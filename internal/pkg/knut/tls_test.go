package knut

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"slices"
	"testing"
	"time"
)

func TestCertNames(t *testing.T) {
	ifAddrs := []net.Addr{
		&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)},
		&net.IPNet{IP: net.ParseIP("192.168.1.20"), Mask: net.CIDRMask(24, 32)},
		&net.IPNet{IP: net.ParseIP("169.254.3.4"), Mask: net.CIDRMask(16, 32)},
		&net.IPNet{IP: net.ParseIP("::1"), Mask: net.CIDRMask(128, 128)},
		&net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)},
		&net.IPNet{IP: net.ParseIP("2001:db8::20"), Mask: net.CIDRMask(64, 128)},
	}
	all := []string{"192.168.1.20", "2001:db8::20", "myhost", "localhost", "127.0.0.1", "::1"}

	tests := []struct {
		host string
		want []string
	}{
		{"192.168.1.20", []string{"192.168.1.20"}},
		{"myhost", []string{"myhost"}},
		{"::1", []string{"::1"}},
		{"", all},
		{"0.0.0.0", all},
		{"::", all},
	}

	for _, test := range tests {
		got := certNames(test.host, ifAddrs, "myhost")
		if !slices.Equal(got, test.want) {
			t.Errorf("%q: got %v, want %v", test.host, got, test.want)
		}
	}
}

func TestOnetimeTLSCertificate(t *testing.T) {
	ot := createOnetimeTLS(t)
	cert := ot.Cert

	if !cert.NotBefore.Before(time.Now().Add(-clockSkew + time.Minute)) {
		t.Errorf("NotBefore %v leaves no room for a lagging clock", cert.NotBefore)
	}
	if !cert.NotAfter.Equal(notAfterNever) {
		t.Errorf("NotAfter %v, want %v", cert.NotAfter, notAfterNever)
	}
	if cert.KeyUsage != x509.KeyUsageDigitalSignature {
		t.Errorf("KeyUsage %v, want DigitalSignature only", cert.KeyUsage)
	}
	if cert.IsCA {
		t.Error("certificate claims to be a CA")
	}
	if !slices.Equal(cert.ExtKeyUsage, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}) {
		t.Errorf("ExtKeyUsage %v, want ServerAuth", cert.ExtKeyUsage)
	}
	if err := cert.VerifyHostname("127.0.0.1"); err != nil {
		t.Errorf("bound address not covered: %v", err)
	}
}

func TestOnetimeTLSHandshake(t *testing.T) {
	ot := createOnetimeTLS(t)
	addr := ot.Listener.Addr().String()

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.Proto))
	})}
	go srv.Serve(ot.Listener)
	t.Cleanup(func() { srv.Close() })

	roots := x509.NewCertPool()
	roots.AddCert(ot.Cert)

	t.Run("h2", func(t *testing.T) {
		client := &http.Client{Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{RootCAs: roots},
			ForceAttemptHTTP2: true,
		}}
		resp, err := client.Get("https://" + addr + "/")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.ProtoMajor != 2 {
			t.Errorf("negotiated %s, want HTTP/2", resp.Proto)
		}
	})

	t.Run("tls1.1 refused", func(t *testing.T) {
		conn, err := tls.Dial("tcp", addr, &tls.Config{
			RootCAs:    roots,
			MinVersion: tls.VersionTLS10,
			MaxVersion: tls.VersionTLS11,
		})
		if err == nil {
			conn.Close()
			t.Error("TLS 1.1 handshake succeeded")
		}
	})

	t.Run("x25519 accepted", func(t *testing.T) {
		conn, err := tls.Dial("tcp", addr, &tls.Config{
			RootCAs:          roots,
			CurvePreferences: []tls.CurveID{tls.X25519},
		})
		if err != nil {
			t.Fatal(err)
		}
		conn.Close()
	})
}

func createOnetimeTLS(t *testing.T) *OnetimeTLS {
	t.Helper()
	ot := &OnetimeTLS{}
	if err := ot.Create("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ot.Listener.Close() })
	return ot
}
