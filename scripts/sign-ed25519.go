package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"os"
)

func main() {
	inPath := flag.String("in", "", "input file to sign")
	outPath := flag.String("out", "", "output signature file (base64)")
	keyB64 := flag.String("key", "", "ed25519 private key (base64, 64 bytes)")
	flag.Parse()

	if *inPath == "" || *outPath == "" {
		exitErr(errors.New("--in and --out are required"))
	}
	if *keyB64 == "" {
		*keyB64 = os.Getenv("UPDATE_PRIVATE_KEY")
	}
	if *keyB64 == "" {
		exitErr(errors.New("missing private key: set --key or UPDATE_PRIVATE_KEY"))
	}

	keyRaw, err := base64.StdEncoding.DecodeString(*keyB64)
	if err != nil {
		exitErr(fmt.Errorf("decode key base64: %w", err))
	}
	if len(keyRaw) != ed25519.PrivateKeySize {
		exitErr(fmt.Errorf("invalid private key length: %d", len(keyRaw)))
	}

	msg, err := os.ReadFile(*inPath)
	if err != nil {
		exitErr(err)
	}

	sig := ed25519.Sign(ed25519.PrivateKey(keyRaw), msg)
	out := base64.StdEncoding.EncodeToString(sig) + "\n"
	if err := os.WriteFile(*outPath, []byte(out), 0644); err != nil {
		exitErr(err)
	}
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(1)
}
