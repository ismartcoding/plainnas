package update

import (
	"archive/zip"
	"context"
	"crypto/ed25519"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type PrepareResult struct {
	Tag         string
	ReleaseURL  string
	ZipPath     string
	SHAPath     string
	SigPath     string
	NewBinPath  string
	BinaryPath  string
	UpdaterHint string
}

// DownloadAndPrepare downloads the signed release zip and prepares `<binary>.new`.
// It does NOT stop/restart services and does NOT replace the running binary.
func DownloadAndPrepare(ctx context.Context, tag string, binaryPath string) (*PrepareResult, error) {
	arch, err := DetectArch()
	if err != nil {
		return nil, err
	}
	goos := runtime.GOOS
	if goos != "linux" {
		return nil, fmt.Errorf("unsupported OS: %s", goos)
	}

	pub, err := LoadPublicKey()
	if err != nil {
		return nil, err
	}

	zipName := ZipName(goos, arch)
	shaName := zipName + ".sha256"
	sigName := shaName + ".sig"

	zipURL := ReleaseAssetURL(tag, zipName)
	shaURL := ReleaseAssetURL(tag, shaName)
	sigURL := ReleaseAssetURL(tag, sigName)

	workDir, err := os.MkdirTemp("", "plainnas-update-*")
	if err != nil {
		return nil, err
	}
	// best-effort cleanup. New binary is written elsewhere.
	defer os.RemoveAll(workDir)

	zipPath := filepath.Join(workDir, zipName)
	shaPath := filepath.Join(workDir, shaName)
	sigPath := filepath.Join(workDir, sigName)

	if err := DownloadToFile(ctx, zipURL, zipPath, 0644); err != nil {
		return nil, err
	}
	if err := DownloadToFile(ctx, shaURL, shaPath, 0644); err != nil {
		return nil, err
	}
	if err := DownloadToFile(ctx, sigURL, sigPath, 0644); err != nil {
		return nil, err
	}

	if err := VerifySHA256File(zipPath, shaPath, sigPath, ed25519.PublicKey(pub)); err != nil {
		return nil, err
	}

	newBinPath, err := EnsureSameDirNewPath(binaryPath)
	if err != nil {
		return nil, err
	}
	if err := extractFileFromZip(zipPath, MainBinaryName(goos, arch), newBinPath, 0755); err != nil {
		return nil, err
	}

	return &PrepareResult{
		Tag:         tag,
		ReleaseURL:  strings.TrimSpace(fmt.Sprintf("https://github.com/%s/releases/tag/%s", Repo, tag)),
		ZipPath:     zipPath,
		SHAPath:     shaPath,
		SigPath:     sigPath,
		NewBinPath:  newBinPath,
		BinaryPath:  binaryPath,
		UpdaterHint: UpdaterBinaryName(goos, arch),
	}, nil
}

func extractFileFromZip(zipPath, memberName, destPath string, mode os.FileMode) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	var member *zip.File
	for _, f := range r.File {
		if f.Name == memberName {
			member = f
			break
		}
	}
	if member == nil {
		return fmt.Errorf("zip missing %s", memberName)
	}

	rc, err := member.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tmp := destPath + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, rc)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	// Give filesystem a tiny moment for metadata durability on some setups.
	_ = os.Chtimes(tmp, time.Now(), time.Now())
	return os.Rename(tmp, destPath)
}
