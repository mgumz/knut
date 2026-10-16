// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package knut

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/big"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// notAfterNever is the "no well-defined expiration date" of RFC 5280,
// 4.1.2.5: the certificate is gone with the process anyway.
var notAfterNever = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)

// clockSkew is how far a client's clock may lag behind knut's.
const clockSkew = time.Hour

// OnetimeAttrs is the value of -tls-onetime. Alone the flag is a bool;
// "-tls-onetime=keep,for=30d" turns it on with attributes.
type OnetimeAttrs struct {
	On   bool
	Keep bool          // store the certificate and reuse it on the next start
	For  time.Duration // how long the certificate is valid, 0: no expiry
}

func (oa *OnetimeAttrs) IsBoolFlag() bool { return true }

func (oa *OnetimeAttrs) String() string {
	if oa == nil || !oa.On {
		return ""
	}
	attrs := []string{}
	if oa.Keep {
		attrs = append(attrs, "keep")
	}
	if oa.For > 0 {
		attrs = append(attrs, "for="+oa.For.String())
	}
	if len(attrs) == 0 {
		return "true"
	}
	return strings.Join(attrs, ",")
}

func (oa *OnetimeAttrs) Set(value string) error {
	if on, err := strconv.ParseBool(value); err == nil {
		*oa = OnetimeAttrs{On: on}
		return nil
	}
	attrs := OnetimeAttrs{On: true}
	for _, attr := range strings.Split(value, ",") {
		name, arg, _ := strings.Cut(attr, "=")
		switch name {
		case "keep":
			attrs.Keep = true
		case "for":
			d, err := parseLifetime(arg)
			if err != nil {
				return fmt.Errorf("for=%s: %w", arg, err)
			}
			attrs.For = d
		default:
			return fmt.Errorf("unknown attribute %q", attr)
		}
	}
	*oa = attrs
	return nil
}

// IsOnetimeAttr tells if arg reads like an attribute of -tls-onetime: given
// as "-tls-onetime keep", the flag stays a plain bool and "keep" ends up as
// a mapping.
func IsOnetimeAttr(arg string) bool {
	return arg == "keep" || strings.HasPrefix(arg, "for=")
}

// parseLifetime takes "30d" next to what time.ParseDuration takes.
func parseLifetime(s string) (time.Duration, error) {
	var d time.Duration
	var err error
	if days, ok := strings.CutSuffix(s, "d"); ok {
		var n int
		n, err = strconv.Atoi(days)
		d = time.Duration(n) * 24 * time.Hour
	} else {
		d, err = time.ParseDuration(s)
	}
	if err == nil && d <= 0 {
		err = fmt.Errorf("not a positive duration")
	}
	return d, err
}

// KeepDir is where -tls-onetime=keep stores the certificate and its key.
func KeepDir() (string, error) {
	dir, err := os.UserCacheDir()
	return filepath.Join(dir, "knut", "tls-onetime"), err
}

const (
	keepKeyFile  = "key.pem"
	keepCertFile = "cert.pem"
)

type OnetimeTLS struct {
	Listener net.Listener
	Cert     *x509.Certificate
	Attrs    OnetimeAttrs
	Dir      string // the folder of a kept certificate, see KeepDir

	now          func() time.Time
	state        string // what became of a kept certificate
	names        []string
	privKey      *ecdsa.PrivateKey
	sn           *big.Int
	template     *x509.Certificate
	privKeyBytes []byte
	certBytes    []byte
	tlsConfig    tls.Config
	err          error
}

// create an in-memory self-signed certificate, sign it with an in-memory
// private key and create a tlsListener based upon that certificate. it's only
// purpose is to have a tls-cert with a onetime, throw-away certificate.
//
// with Attrs.Keep the certificate is read from Dir, and written there when
// it had to be issued.
func (ot *OnetimeTLS) Create(addr string) error {

	if ot.now == nil {
		ot.now = time.Now
	}

	ot.createListener(addr)
	ot.createNames(addr)
	ot.loadKept()
	if ot.Cert == nil {
		ot.createPrivateKey()
		ot.createSerialNumber(128)
		ot.createTemplate()
		ot.createCertBytes()
	}
	ot.createPKBytes()
	ot.storeKept()
	ot.fillTLSConfig()

	if ot.err == nil {
		ot.Listener = tls.NewListener(ot.Listener, &ot.tlsConfig)
	}
	return ot.err
}

// Describe writes what a client needs to check the certificate instead of
// just clicking through the warning about it.
func (ot *OnetimeTLS) Describe(w io.Writer, url string) {
	pin := "sha256//" + PublicKeyPin(ot.Cert)
	validity := "valid while knut runs"
	switch {
	case ot.Attrs.For > 0:
		validity = "valid until " + ot.Cert.NotAfter.Local().Format("2006-01-02 15:04 MST")
	case ot.Attrs.Keep:
		validity = "doesn't expire"
	}
	fmt.Fprintf(w, "tls: onetime certificate, %s\n", validity)
	if ot.Attrs.Keep {
		fmt.Fprintf(w, "  kept    %s\n", ot.Dir)
		fmt.Fprintf(w, "          %s\n", ot.state)
	}
	fmt.Fprintf(w, "  sha256  %s\n", Fingerprint(ot.Cert))
	fmt.Fprintln(w, "          compare with the certificate details of the browser")
	fmt.Fprintf(w, "  pin     %s\n", pin)
	fmt.Fprintf(w, "          curl --pinnedpubkey '%s' -k %s\n", pin, url)
	fmt.Fprintf(w, "  names   %s\n", strings.Join(ot.names, ", "))
}

// Fingerprint is the SHA-256 hash of the certificate, the way browsers show
// it.
func Fingerprint(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	hex := make([]string, len(sum))
	for i, b := range sum {
		hex[i] = fmt.Sprintf("%02X", b)
	}
	return strings.Join(hex, ":")
}

// PublicKeyPin is the base64 encoded SHA-256 hash of the public key, the way
// "curl --pinnedpubkey sha256//<pin>" takes it.
func PublicKeyPin(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	return base64.StdEncoding.EncodeToString(sum[:])
}

func (ot *OnetimeTLS) createListener(addr string) {
	if ot.err == nil {
		ot.Listener, ot.err = net.Listen("tcp", addr)
	}
}
func (ot *OnetimeTLS) createNames(addr string) {
	if ot.err != nil {
		return
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		ot.err = err
		return
	}
	// both are optional: without them the certificate covers the
	// loopback names only.
	ifAddrs, _ := net.InterfaceAddrs()
	hostname, _ := os.Hostname()
	ot.names = certNames(host, ifAddrs, hostname)
}
func (ot *OnetimeTLS) createPrivateKey() {
	if ot.err == nil && ot.privKey == nil {
		// * in general a good read: https://safecurves.cr.yp.to/
		// * elliptic.P256/() returns a Curve which implements P-256 (see
		//   FIPS 186-3, section D.2.3)
		// * Chrome redraw support for elliptic.P521() (see
		//   https://bugs.chromium.org/p/chromium/issues/detail?id=478225
		//   https://boringssl.googlesource.com/boringssl/+/e9fc3e547e557492316932b62881c3386973ceb2%5E!)
		// * http://security.stackexchange.com/questions/31772/what-elliptic-curves-are-supported-by-browsers
		curve := elliptic.P256()
		ot.privKey, ot.err = ecdsa.GenerateKey(curve, rand.Reader)
	}
}
func (ot *OnetimeTLS) createSerialNumber(n uint) {
	if ot.err == nil {
		ot.sn, ot.err = rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), n))
	}
}
func (ot *OnetimeTLS) createTemplate() {
	if ot.err == nil {
		ot.template = &x509.Certificate{
			SerialNumber: ot.sn,
			Subject: pkix.Name{
				Organization:       []string{"Knut"},
				OrganizationalUnit: []string{"Knut operators"},
				CommonName:         "Onetime, throwaway certificate of *knut*",
			},
			NotBefore:   ot.now().Add(-clockSkew),
			NotAfter:    notAfterNever,
			KeyUsage:    x509.KeyUsageDigitalSignature,
			ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		}
		if ot.Attrs.For > 0 {
			ot.template.NotAfter = ot.now().Add(ot.Attrs.For)
		}
		for _, name := range ot.names {
			if ip := net.ParseIP(name); ip != nil {
				ot.template.IPAddresses = append(ot.template.IPAddresses, ip)
			} else {
				ot.template.DNSNames = append(ot.template.DNSNames, name)
			}
		}
	}
}
func (ot *OnetimeTLS) createPKBytes() {
	if ot.err == nil {
		ot.privKeyBytes, ot.err = x509.MarshalECPrivateKey(ot.privKey)
	}
	if ot.err == nil {
		ot.privKeyBytes = bytesToPem(ot.privKeyBytes, "EC PRIVATE KEY")
	}
}
func (ot *OnetimeTLS) createCertBytes() {
	var der []byte
	if ot.err == nil {
		der, ot.err = x509.CreateCertificate(rand.Reader, ot.template, ot.template, &ot.privKey.PublicKey, ot.privKey)
	}
	if ot.err == nil {
		ot.Cert, ot.err = x509.ParseCertificate(der)
	}
	if ot.err == nil {
		ot.certBytes = bytesToPem(der, "CERTIFICATE")
	}
}

// loadKept reuses a kept certificate as long as it is valid, covers the
// names and has the lifetime asked for. a kept key outlives a certificate
// which no longer fits: the pin stays the same. an expired certificate takes
// its key along.
func (ot *OnetimeTLS) loadKept() {
	if ot.err != nil || !ot.Attrs.Keep {
		return
	}
	key, cert, err := readKept(ot.Dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		ot.state = "new, nothing kept yet"
	case err != nil:
		ot.state = fmt.Sprintf("new, the kept one is unusable: %v", err)
	case !ot.now().Before(cert.NotAfter):
		ot.state = "new, the kept one has expired"
	case !coversExactly(cert, ot.names):
		ot.privKey = key
		ot.state = "issued again for other names, same key"
	case !hasLifetime(cert, ot.Attrs.For):
		ot.privKey = key
		ot.state = "issued again for another lifetime, same key"
	default:
		ot.privKey, ot.Cert = key, cert
		ot.certBytes = bytesToPem(cert.Raw, "CERTIFICATE")
		ot.state = "reused"
	}
}

// storeKept writes what loadKept had no use for.
func (ot *OnetimeTLS) storeKept() {
	if ot.err != nil || !ot.Attrs.Keep || ot.state == "reused" {
		return
	}
	ot.err = os.MkdirAll(ot.Dir, 0o700)
	if ot.err == nil {
		ot.err = os.WriteFile(filepath.Join(ot.Dir, keepKeyFile), ot.privKeyBytes, 0o600)
	}
	if ot.err == nil {
		ot.err = os.WriteFile(filepath.Join(ot.Dir, keepCertFile), ot.certBytes, 0o600)
	}
}

func readKept(dir string) (*ecdsa.PrivateKey, *x509.Certificate, error) {
	keyPEM, err := os.ReadFile(filepath.Join(dir, keepKeyFile))
	if err != nil {
		return nil, nil, err
	}
	certPEM, err := os.ReadFile(filepath.Join(dir, keepCertFile))
	if err != nil {
		return nil, nil, err
	}
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, nil, err
	}
	key, ok := pair.PrivateKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, nil, fmt.Errorf("not an ECDSA key")
	}
	cert, err := x509.ParseCertificate(pair.Certificate[0])
	return key, cert, err
}

// coversExactly tells if the SANs of cert are the names, no more, no less.
func coversExactly(cert *x509.Certificate, names []string) bool {
	have := []string{}
	for _, ip := range cert.IPAddresses {
		have = append(have, ip.String())
	}
	have = append(have, cert.DNSNames...)
	want := []string{}
	for _, name := range names {
		if ip := net.ParseIP(name); ip != nil {
			name = ip.String()
		}
		want = append(want, name)
	}
	slices.Sort(have)
	slices.Sort(want)
	return slices.Equal(have, want)
}

// hasLifetime tells if cert was issued with -tls-onetime=for=lifetime;
// certificates keep their dates to the second.
func hasLifetime(cert *x509.Certificate, lifetime time.Duration) bool {
	if lifetime == 0 {
		return cert.NotAfter.Equal(notAfterNever)
	}
	issued := cert.NotBefore.Add(clockSkew)
	return (cert.NotAfter.Sub(issued) - lifetime).Abs() < time.Second
}

// fillTLSConfig keeps the defaults of crypto/tls for versions and curves.
// "h2" is offered via ALPN: http.Serve takes it from there when the
// http.Server has no TLSConfig of its own.
func (ot *OnetimeTLS) fillTLSConfig() {
	if ot.err == nil {
		ot.tlsConfig.NextProtos = []string{"h2", "http/1.1"}
		ot.tlsConfig.SessionTicketsDisabled = true
		ot.tlsConfig.Certificates = make([]tls.Certificate, 1)
		ot.tlsConfig.Certificates[0], ot.err = tls.X509KeyPair(ot.certBytes, ot.privKeyBytes)
	}
}

// certNames lists the names a client might use to reach knut bound to host.
// a concrete host is the only name; an empty or unspecified host stands for
// every interface, so every usable interface address and the hostname count,
// plus the loopback names.
//
// link-local addresses are left out: a client would need the zone to reach
// them, which no URL in a browser carries.
func certNames(host string, ifAddrs []net.Addr, hostname string) []string {

	if host != "" {
		if ip, err := netip.ParseAddr(host); err != nil || !ip.IsUnspecified() {
			return []string{host}
		}
	}

	names := []string{}
	seen := map[string]bool{}
	add := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}

	for _, a := range ifAddrs {
		prefix, err := netip.ParsePrefix(a.String())
		if err != nil {
			continue
		}
		ip := prefix.Addr()
		if ip.IsLoopback() || ip.IsMulticast() || ip.IsLinkLocalUnicast() {
			continue
		}
		add(ip.String())
	}
	add(hostname)
	add("localhost")
	add("127.0.0.1")
	add("::1")

	return names
}

func bytesToPem(in []byte, blockType string) []byte {
	buf := bytes.NewBuffer(nil)
	pem.Encode(buf, &pem.Block{Type: blockType, Bytes: in})
	return buf.Bytes()
}
