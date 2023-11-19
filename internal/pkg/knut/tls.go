// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package knut

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"time"
)

type OnetimeTLS struct {
	Listener net.Listener

	privKey      *ecdsa.PrivateKey
	sn           *big.Int
	template     *x509.Certificate
	privKeyBytes []byte
	certBytes    []byte
	tlsConfig    tls.Config
	err          error
}

// create an in-memory root-certificate, sign it with an in-memory private key
// (elliptic curve521), and create a tlsListener based upon that certificate.
// it's only purpose is to have a tls-cert with a onetime, throw-away
// certificate. fyi: http://safecurves.cr.yp.to/
func (ot *OnetimeTLS) Create(addr string) error {

	ot.createListener(addr)
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

func (ot *OnetimeTLS) createListener(addr string) {
	if ot.err == nil {
		ot.Listener, ot.err = net.Listen("tcp", addr)
	}
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
			Issuer: pkix.Name{
				Organization:       []string{"Association of united Knuts"},
				OrganizationalUnit: []string{"Knut CA of onetime, throw-away certificates"},
				CommonName:         "Onetime, throwaway certificate of *knut*",
			},
			Subject: pkix.Name{
				Organization:       []string{"Knut"},
				OrganizationalUnit: []string{"Knut operators"},
				CommonName:         "Onetime, throwaway certificate of *knut*",
			},
			NotBefore:   time.Now(),
			NotAfter:    time.Now().Add(24 * time.Hour),
			KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
			ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
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
	if ot.err == nil {
		ot.certBytes, ot.err = x509.CreateCertificate(rand.Reader, ot.template, ot.template, &ot.privKey.PublicKey, ot.privKey)
	}
	if ot.err == nil {
		ot.certBytes = bytesToPem(ot.certBytes, "CERTIFICATE")
	}
}
func (ot *OnetimeTLS) fillTLSConfig() {
	if ot.err == nil {
		ot.tlsConfig.NextProtos = []string{"http/1.1"}
		ot.tlsConfig.MinVersion = tls.VersionTLS11
		ot.tlsConfig.CurvePreferences = []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256}
		ot.tlsConfig.SessionTicketsDisabled = true
		ot.tlsConfig.Certificates = make([]tls.Certificate, 1)
		ot.tlsConfig.Certificates[0], ot.err = tls.X509KeyPair(ot.certBytes, ot.privKeyBytes)
	}
}

func bytesToPem(in []byte, blockType string) []byte {
	buf := bytes.NewBuffer(nil)
	pem.Encode(buf, &pem.Block{Type: blockType, Bytes: in})
	return buf.Bytes()
}
