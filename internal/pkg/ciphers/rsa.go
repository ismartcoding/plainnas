package ciphers

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"errors"
	"ismartcoding/plainnas/internal/pkg/log"
)

type rsaUtil struct{}

var RSA = &rsaUtil{}

// GenerateKeyPair generates a new key pair
func (r *rsaUtil) GenerateKeyPair(bits int) (*rsa.PrivateKey, *rsa.PublicKey) {
	privkey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		log.Error(err)
	}
	return privkey, &privkey.PublicKey
}

// PrivateKeyToBytes private key to bytes
func (r *rsaUtil) PrivateKeyToBytes(priv *rsa.PrivateKey) []byte {
	return x509.MarshalPKCS1PrivateKey(priv)
}

// PublicKeyToBytes public key to bytes
func (r *rsaUtil) PublicKeyToBytes(pub *rsa.PublicKey) []byte {
	pubASN1, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		log.Error(err)
	}

	return pubASN1
}

// BytesToPrivateKey bytes to private key
func (r *rsaUtil) BytesToPrivateKey(priv []byte) *rsa.PrivateKey {
	key, err := x509.ParsePKCS1PrivateKey(priv)
	if err != nil {
		log.Error(err)
	}
	return key
}

// BytesToPublicKey bytes to public key
func (r *rsaUtil) BytesToPublicKey(pub []byte) *rsa.PublicKey {
	ifc, err := x509.ParsePKIXPublicKey(pub)
	if err != nil {
		log.Error(err)
	}
	key, ok := ifc.(*rsa.PublicKey)
	if !ok {
		log.Error(errors.New("not ok"))
	}
	return key
}

// EncryptWithPublicKey encrypts data with public key
func (r *rsaUtil) EncryptWithPublicKey(msg []byte, pub *rsa.PublicKey) []byte {
	hash := sha1.New()
	ciphertext, err := rsa.EncryptOAEP(hash, rand.Reader, pub, msg, nil)
	if err != nil {
		log.Error(err)
	}
	return ciphertext
}

// DecryptWithPrivateKey decrypts data with private key
func (r *rsaUtil) DecryptWithPrivateKey(ciphertext []byte, priv *rsa.PrivateKey) []byte {
	hash := sha1.New()
	plaintext, err := rsa.DecryptOAEP(hash, rand.Reader, priv, ciphertext, nil)
	if err != nil {
		log.Error(err)
	}
	return plaintext
}
