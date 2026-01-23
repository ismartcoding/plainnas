package update

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	Repo                   = "ismartcoding/plainnas"
	GitHubLatestReleaseURL = "https://api.github.com/repos/" + Repo + "/releases/latest"
	DefaultHealthPath      = "/health_check"
	PubKeyEnv              = "PLAINNAS_UPDATE_PUBKEY_B64"
	PubKeyFile             = "/etc/plainnas/update_pubkey"
)

var ErrPubKeyNotConfigured = errors.New("update public key not configured")

// DefaultPubKeyB64 optionally embeds the Ed25519 update verification public key into the binary.
//
// Priority order (highest to lowest):
//  1. env var PLAINNAS_UPDATE_PUBKEY_B64
//  2. file /etc/plainnas/update_pubkey
//  3. this value
//
// This should be base64 of the raw 32-byte Ed25519 public key. It is safe to ship publicly.
//
// For official builds, set this at link time:
//
//	go build -ldflags "-X ismartcoding/plainnas/internal/update.DefaultPubKeyB64=<pubkey_b64>"
var DefaultPubKeyB64 = ""

type LatestRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

type Plan struct {
	ServiceName string `json:"serviceName"`
	BinaryPath  string `json:"binaryPath"`
	NewPath     string `json:"newPath"`
	OldPath     string `json:"oldPath"`
	HealthURL   string `json:"healthUrl"`
}

func DetectArch() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return "amd64", nil
	case "arm64":
		return "arm64", nil
	default:
		return "", fmt.Errorf("unsupported arch: %s", runtime.GOARCH)
	}
}

func AssetBaseName(goos, arch string) string {
	return fmt.Sprintf("plainnas-%s-%s", goos, arch)
}

func ZipName(goos, arch string) string {
	return AssetBaseName(goos, arch) + ".zip"
}

func MainBinaryName(goos, arch string) string {
	return AssetBaseName(goos, arch)
}

func UpdaterBinaryName(goos, arch string) string {
	return fmt.Sprintf("plainnas-updater-%s-%s", goos, arch)
}

func DownloadLatestRelease(ctx context.Context) (*LatestRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, GitHubLatestReleaseURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "plainnas")

	client := &http.Client{Timeout: 8 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github latest release http %d", res.StatusCode)
	}
	var out LatestRelease
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func ReleaseAssetURL(tag, filename string) string {
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", Repo, tag, filename)
}

func LoadPublicKey() (ed25519.PublicKey, error) {
	if b64 := strings.TrimSpace(os.Getenv(PubKeyEnv)); b64 != "" {
		return parsePubKeyB64(b64)
	}
	b, err := os.ReadFile(PubKeyFile)
	if err != nil {
		if b64 := strings.TrimSpace(DefaultPubKeyB64); b64 != "" {
			return parsePubKeyB64(b64)
		}
		return nil, ErrPubKeyNotConfigured
	}
	return parsePubKeyB64(strings.TrimSpace(string(b)))
}

func parsePubKeyB64(b64 string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("decode pubkey base64: %w", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid pubkey length: %d", len(raw))
	}
	return ed25519.PublicKey(raw), nil
}

func DownloadToFile(ctx context.Context, url, path string, mode os.FileMode) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "plainnas")
	client := &http.Client{Timeout: 60 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s http %d", url, res.StatusCode)
	}

	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, res.Body)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	return os.Rename(tmp, path)
}

func VerifySHA256File(zipPath, shaPath, sigPath string, pub ed25519.PublicKey) error {
	shaBytes, err := os.ReadFile(shaPath)
	if err != nil {
		return err
	}
	sigB64, err := os.ReadFile(sigPath)
	if err != nil {
		return err
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigB64)))
	if err != nil {
		return fmt.Errorf("decode signature base64: %w", err)
	}
	if len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("invalid signature length: %d", len(sig))
	}
	if !ed25519.Verify(pub, shaBytes, sig) {
		return errors.New("signature verification failed")
	}

	expected, err := parseSHA256Sum(shaBytes)
	if err != nil {
		return err
	}
	actual, err := sha256FileHex(zipPath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(expected, actual) {
		return fmt.Errorf("sha256 mismatch: expected %s got %s", expected, actual)
	}
	return nil
}

func parseSHA256Sum(b []byte) (string, error) {
	line := strings.TrimSpace(string(b))
	if line == "" {
		return "", errors.New("empty sha256 file")
	}
	fields := strings.Fields(line)
	if len(fields) < 1 {
		return "", errors.New("invalid sha256 file")
	}
	h := strings.TrimSpace(fields[0])
	if len(h) != 64 {
		return "", fmt.Errorf("invalid sha256 length: %d", len(h))
	}
	_, err := hex.DecodeString(h)
	if err != nil {
		return "", fmt.Errorf("invalid sha256 hex: %w", err)
	}
	return h, nil
}

func sha256FileHex(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func EnsureSameDirNewPath(binaryPath string) (string, error) {
	dir := filepath.Dir(binaryPath)
	base := filepath.Base(binaryPath)
	if base == "." || base == string(os.PathSeparator) {
		return "", fmt.Errorf("invalid binary path: %s", binaryPath)
	}
	return filepath.Join(dir, base+".new"), nil
}

func EnsureOldPath(binaryPath string) string {
	dir := filepath.Dir(binaryPath)
	base := filepath.Base(binaryPath)
	return filepath.Join(dir, base+".old")
}
