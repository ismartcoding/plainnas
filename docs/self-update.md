# Self-update (two-process, rollback-safe)

PlainNAS uses a **two-process update model**:

- **Main program (`plainnas`)**: checks for updates, downloads release artifacts, verifies **SHA256 + Ed25519 signature**, and writes the new binary as `plainnas.new` (does **not** overwrite itself).
- **Updater (`plainnas-updater`)**: runs as a separate process, stops/starts the systemd service, swaps binaries atomically, performs health check, and rolls back on failure.

## Files and atomicity

- The new version is written as `<binary>.new` in the **same directory** as the current binary.
- The updater uses atomic renames on the same filesystem:
  - `plainnas -> plainnas.old`
  - `plainnas.new -> plainnas`

## Health check

The updater waits for the HTTP endpoint:

- `GET /health_check` (default: `http://127.0.0.1:8080/health_check`)

## Keys (required)

Updates require an Ed25519 **public key** that matches the private key used to sign releases.

For this project, the intended setup is:

- The **public key is embedded into the released `plainnas` binary**.
- The **private key is used only in CI** to sign the release artifacts.
- End users do not need to configure anything for updates to work.

The public key value is **base64 of the raw 32-byte Ed25519 public key** (not SSH/OpenSSH text format).

### Generate keys (one-time)

Generate a new keypair (prints the private key base64, 64 bytes when decoded):

```bash
go run - <<'GO'
package main
import (
  "crypto/ed25519"
  "crypto/rand"
  "encoding/base64"
  "fmt"
)
func main() {
  pub, priv, err := ed25519.GenerateKey(rand.Reader)
  if err != nil { panic(err) }
  fmt.Println(base64.StdEncoding.EncodeToString(pub))
  fmt.Println(base64.StdEncoding.EncodeToString(priv))
}
GO
```

### Install the public key on the target machine

Not needed for official releases (the key is embedded).

## CLI usage

- Check latest release:
  - `plainnas update check`

- Download + verify + prepare `plainnas.new`:
  - `plainnas update download`

- Apply prepared update (runs updater via `systemd-run` so it survives service stop):
  - `plainnas update apply`

## Release artifacts

The GitHub release workflow publishes:

- `plainnas-linux-<arch>.zip`
- `plainnas-linux-<arch>.zip.sha256`
- `plainnas-linux-<arch>.zip.sha256.sig`

The signature is Ed25519 over the **exact bytes** of the `.sha256` file (base64 encoded in `.sig`).

## GitHub Actions secret

To build releasable artifacts and make updates work out-of-the-box, set these repository secrets:

- `UPDATE_PRIVATE_KEY` (base64 of the 64-byte Ed25519 private key; used only to sign)
- `UPDATE_PUBLIC_KEY` (base64 of the 32-byte Ed25519 public key; embedded into the `plainnas` binary)

The workflow embeds the public key using `-ldflags` and uses `scripts/sign-ed25519.go` to sign.
