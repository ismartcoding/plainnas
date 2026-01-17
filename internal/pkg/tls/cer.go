package tls

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"os"
	"time"
)

func MakeCert(certFile string, keyFile string) {
	priv, err := generateKey()
	pub := priv.(crypto.Signer).Public()

	expiration := time.Now().AddDate(20, 0, 0)

	tpl := &x509.Certificate{
		SerialNumber: randomSerialNumber(),
		Subject: pkix.Name{
			Organization: []string{"PlainBox"},
			CommonName:   "PlainBox",
		},

		NotBefore: time.Now(), NotAfter: expiration,

		KeyUsage: x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
	}

	cert, err := x509.CreateCertificate(rand.Reader, tpl, tpl, pub, priv)
	fatalIfErr(err, "failed to generate certificate")

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert})
	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	fatalIfErr(err, "failed to encode certificate key")
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER})

	err = os.WriteFile(certFile, certPEM, 0644)
	fatalIfErr(err, "failed to save certificate")
	err = os.WriteFile(keyFile, privPEM, 0600)
	fatalIfErr(err, "failed to save certificate key")
}

func generateKey() (crypto.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

func randomSerialNumber() *big.Int {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	fatalIfErr(err, "failed to generate serial number")
	return serialNumber
}

func fatalIfErr(err error, msg string) {
	if err != nil {
		log.Fatalf("ERROR: %s: %s", msg, err)
	}
}
