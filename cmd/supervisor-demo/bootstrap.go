package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

func bootstrap(root string) error {
	if err := os.Mkdir(root, 0700); err != nil {
		return err
	}
	now := time.Now()
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Supervisor Demo CA"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(7 * 24 * time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caPub, caKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, caPub, caKey)
	if err != nil {
		return err
	}
	signPub, signKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	token := make([]byte, 32)
	if _, err = rand.Read(token); err != nil {
		return err
	}
	for i, role := range []string{"server", "admin", "agent", "signer"} {
		dir := filepath.Join(root, role)
		if err = os.Mkdir(dir, 0700); err != nil {
			return err
		}
		write := func(name string, data []byte) error { return os.WriteFile(filepath.Join(dir, name), data, 0600) }
		if role == "signer" {
			if err = write("sign.key", signKey); err != nil {
				return err
			}
			continue
		}
		name := role
		usage := x509.ExtKeyUsageClientAuth
		if role == "agent" {
			name = "demo-01"
		}
		if role == "server" {
			usage = x509.ExtKeyUsageServerAuth
		}
		cert := &x509.Certificate{SerialNumber: big.NewInt(int64(i + 2)), Subject: pkix.Name{CommonName: name, OrganizationalUnit: []string{role}}, NotBefore: ca.NotBefore, NotAfter: ca.NotAfter, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}}
		if role == "server" {
			cert.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
			cert.DNSNames = []string{"localhost"}
		}
		pub, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		der, err := x509.CreateCertificate(rand.Reader, cert, ca, pub, caKey)
		if err != nil {
			return err
		}
		keyDER, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			return err
		}
		for name, data := range map[string][]byte{
			"ca.pem":      pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}),
			role + ".pem": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
			role + ".key": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}),
		} {
			if err := write(name, data); err != nil {
				return err
			}
		}
		if role == "server" || role == "agent" {
			if err = write("sign.pub", signPub); err != nil {
				return err
			}
			if err = write("enrollment", []byte(hex.EncodeToString(token))); err != nil {
				return err
			}
		}
	}
	return nil
}
