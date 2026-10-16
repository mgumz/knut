package knut

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
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
	ot := createOnetimeTLS(t, &OnetimeTLS{})
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
	ot := createOnetimeTLS(t, &OnetimeTLS{})
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

func createOnetimeTLS(t *testing.T, ot *OnetimeTLS) *OnetimeTLS {
	t.Helper()
	if err := ot.Create("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ot.Listener.Close() })
	return ot
}

func TestOnetimeAttrsSet(t *testing.T) {
	tests := []struct {
		value string
		want  OnetimeAttrs
	}{
		{"true", OnetimeAttrs{On: true}},
		{"false", OnetimeAttrs{}},
		{"keep", OnetimeAttrs{On: true, Keep: true}},
		{"for=30d", OnetimeAttrs{On: true, For: 30 * 24 * time.Hour}},
		{"for=12h,keep", OnetimeAttrs{On: true, Keep: true, For: 12 * time.Hour}},
	}
	for _, test := range tests {
		var got OnetimeAttrs
		if err := got.Set(test.value); err != nil {
			t.Errorf("%q: %v", test.value, err)
		} else if got != test.want {
			t.Errorf("%q: got %+v, want %+v", test.value, got, test.want)
		}
	}

	for _, value := range []string{"kep", "keep,", "for=", "for=x", "for=0d", "for=-1h", "keep&for=1d"} {
		var got OnetimeAttrs
		if err := got.Set(value); err == nil {
			t.Errorf("%q: no error, got %+v", value, got)
		}
	}
}

func TestOnetimeAttrsFlag(t *testing.T) {
	tests := []struct {
		args []string
		want OnetimeAttrs
	}{
		{[]string{"-tls-onetime", "/:."}, OnetimeAttrs{On: true}},
		{[]string{"-tls-onetime=keep,for=1d", "/:."}, OnetimeAttrs{On: true, Keep: true, For: 24 * time.Hour}},
		{[]string{"-tls-onetime", "keep"}, OnetimeAttrs{On: true}},
	}
	for _, test := range tests {
		f := flag.NewFlagSet("knut", flag.ContinueOnError)
		opts := SetupFlags(f)
		if err := f.Parse(test.args); err != nil {
			t.Errorf("%v: %v", test.args, err)
		} else if opts.TlsOnetime != test.want {
			t.Errorf("%v: got %+v, want %+v", test.args, opts.TlsOnetime, test.want)
		}
	}
}

func TestOnetimeTLSFor(t *testing.T) {
	start := time.Now()
	ot := createOnetimeTLS(t, &OnetimeTLS{Attrs: OnetimeAttrs{On: true, For: time.Hour}})
	if d := ot.Cert.NotAfter.Sub(start.Add(time.Hour)); d.Abs() > 2*time.Second {
		t.Errorf("NotAfter %v, want about an hour from now", ot.Cert.NotAfter)
	}
}

func TestOnetimeTLSKeep(t *testing.T) {
	dir := t.TempDir()
	keep := OnetimeAttrs{On: true, Keep: true}
	create := func(addr string, attrs OnetimeAttrs, now time.Time) *OnetimeTLS {
		t.Helper()
		ot := &OnetimeTLS{Attrs: attrs, Dir: dir, now: func() time.Time { return now }}
		if err := ot.Create(addr); err != nil {
			t.Fatal(err)
		}
		ot.Listener.Close()
		return ot
	}
	check := func(ot *OnetimeTLS, state string) {
		t.Helper()
		if ot.state != state {
			t.Errorf("state %q, want %q", ot.state, state)
		}
	}
	now := time.Now()

	first := create("127.0.0.1:0", keep, now)
	check(first, "new, nothing kept yet")
	for _, name := range []string{keepKeyFile, keepCertFile} {
		fi, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0o600 {
			t.Errorf("%s: mode %v, want 0600", name, fi.Mode().Perm())
		}
	}

	again := create("127.0.0.1:0", keep, now)
	check(again, "reused")
	if Fingerprint(again.Cert) != Fingerprint(first.Cert) {
		t.Error("reused certificate has another fingerprint")
	}

	moved := create("[::1]:0", keep, now)
	check(moved, "issued again for other names, same key")
	if Fingerprint(moved.Cert) == Fingerprint(first.Cert) || PublicKeyPin(moved.Cert) != PublicKeyPin(first.Cert) {
		t.Error("want another fingerprint, the same pin")
	}

	lifetime := OnetimeAttrs{On: true, Keep: true, For: time.Hour}
	limited := create("[::1]:0", lifetime, now.Add(-2*time.Hour))
	check(limited, "issued again for another lifetime, same key")

	expired := create("[::1]:0", lifetime, now)
	check(expired, "new, the kept one has expired")
	if PublicKeyPin(expired.Cert) == PublicKeyPin(first.Cert) {
		t.Error("an expired certificate kept its key")
	}

	if err := os.WriteFile(filepath.Join(dir, keepCertFile), []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	broken := create("[::1]:0", lifetime, now)
	if !strings.HasPrefix(broken.state, "new, the kept one is unusable") {
		t.Errorf("state %q, want unusable", broken.state)
	}
}
