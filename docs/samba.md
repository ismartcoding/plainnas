# LAN Share (SMB / Samba)

PlainNAS can share one folder over your local network using SMB (Samba 3).

## Configure

In the Web UI:

- Settings → LAN Share
- Username is fixed: `nas`
- Access modes:
  - Anyone can modify (guest)
  - Anyone read-only (guest)
  - Password required (read-write)
  - Password required (read-only)

If you select **Password required**, you must set a password at least once. After that, you can leave it blank to keep the current password.

## How to access

- Windows: `\\<NAS-IP>\<share-name>`
- macOS: Finder → Go → Connect to Server → `smb://<NAS-IP>/<share-name>`
- Linux: `smb://<NAS-IP>/<share-name>` (file manager) or `mount -t cifs`

## Notes

- PlainNAS manages `/etc/samba/smb.conf`. Manual edits may be overwritten after you click Save in Settings.
- Ensure Samba is installed via `sudo go run main.go install` (or install your distro `samba` package).
- PlainNAS will try to enable/restart the Samba systemd service automatically when applying settings.

### macOS Finder compatibility

PlainNAS enables Samba's `fruit` compatibility options when the required Samba VFS modules are installed.
For share paths that do not support extended attributes (common on some USB / exFAT mounts), PlainNAS falls
back to AppleDouble sidecar files for macOS metadata.

If your generated `smb.conf` does not contain any `fruit:` options, it usually means the `fruit` module is not
installed on your system.

- Debian/Ubuntu: install `samba-vfs-modules` (PlainNAS install will do this on newer versions)
  - `sudo apt-get install -y samba-vfs-modules`
- Verify module exists:
  - `smbd -b | grep MODULESDIR`
  - `ls -la "$(smbd -b | awk -F': ' '/^MODULESDIR:/{print $2}')/vfs" | grep fruit`
