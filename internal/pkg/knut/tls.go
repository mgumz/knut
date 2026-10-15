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
	"fmt"
	"io"
	"math/big"
	"net"
	"net/netip"
	"os"
	"strings"
	"time"
)

// notAfterNever is the "no well-defined expiration date" of RFC 5280,
// 4.1.2.5: the certificate is gone with the process anyway.
var notAfterNever = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)

// clockSkew is how far a client's clock may lag behind knut's.
const clockSkew = time.Hour

type OnetimeTLS struct {
	Listener net.Listener
	Cert     *x509.Certificate

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
func (ot *OnetimeTLS) Create(addr string) error {

	ot.createListener(addr)
	ot.createNames(addr)
	ot.createPrivateKey()
	ot.createSerialNumber(128)
	ot.createTemplate()
	ot.createPKBytes()
	ot.createCertBytes()
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
	fmt.Fprintln(w, "tls: onetime certificate, valid while knut runs")
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
	if ot.err == nil {
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
			NotBefore:   time.Now().Add(-clockSkew),
			NotAfter:    notAfterNever,
			KeyUsage:    x509.KeyUsageDigitalSignature,
			ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
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
