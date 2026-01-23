#!/usr/bin/env bash
set -e

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[1;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# PlainNAS install script
# Downloads the latest release from GitHub and installs to /usr/local/bin

REPO="ismartcoding/plainnas"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="plainnas"
UPDATER_NAME="plainnas-updater"

function error_exit() {
    echo "[PlainNAS] ERROR: $1" >&2
    exit 1
}

function detect_arch() {
    local arch
    arch=$(uname -m)
    case "$arch" in
        x86_64|amd64)
            echo "amd64" ;;
        aarch64|arm64)
            echo "arm64" ;;
        *)
            error_exit "Unsupported architecture: $arch. Only amd64 and arm64 are supported."
            ;;
    esac
}

function get_latest_release() {
    curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" |
        grep "tag_name" | head -n 1 | cut -d '"' -f4
}

function download_binary() {
    local version="$1"
    local arch="$2"
    local url="https://github.com/$REPO/releases/download/$version/plainnas-linux-$arch.zip"
    echo "[PlainNAS] Downloading $url ..."
    curl -fLo "plainnas.zip" "$url" || error_exit "Failed to download binary zip."
    unzip -o plainnas.zip || error_exit "Failed to unzip binary."
    rm -f plainnas.zip
    mv plainnas-linux-$arch "$BINARY_NAME" || error_exit "Failed to rename binary."
    mv plainnas-updater-linux-$arch "$UPDATER_NAME" || error_exit "Failed to rename updater."
    chmod +x "$BINARY_NAME" "$UPDATER_NAME"
}

function install_binary() {
    sudo mv "$BINARY_NAME" "$INSTALL_DIR/" || error_exit "Failed to move binary to $INSTALL_DIR."
    sudo mv "$UPDATER_NAME" "$INSTALL_DIR/" || error_exit "Failed to move updater to $INSTALL_DIR."
    echo "[PlainNAS] Installed $BINARY_NAME to $INSTALL_DIR."
}

function run_install() {
    sudo "$INSTALL_DIR/$BINARY_NAME" install || error_exit "plainnas install step failed."
}

function start_service() {
    sudo systemctl enable --now plainnas || error_exit "Failed to start plainnas service."
}

main() {
    echo "[PlainNAS] Detecting platform..."
    ARCH=$(detect_arch)
    VERSION=$(get_latest_release)
    if [[ -z "$VERSION" ]]; then
        error_exit "Could not determine latest release version."
    fi
    echo "[PlainNAS] Latest version: $VERSION"
    download_binary "$VERSION" "$ARCH"
    install_binary
    run_install
    start_service
    echo -e "${GREEN}[PlainNAS] Installation successful!${NC}"
    # Get the first non-loopback IPv4 address
    SERVER_IP=$(hostname -I | awk '{for(i=1;i<=NF;i++) if ($i!~/^127\./ && $i~/\./) {print $i; exit}}')
    if [[ -z "$SERVER_IP" ]]; then
        SERVER_IP="localhost"
    fi
    # Color and clickable links (most terminals support clickable URLs)
    echo -e "${CYAN}[PlainNAS] Access the web UI:${NC}"
    echo -e "  ${BLUE}http://$SERVER_IP:8080${NC}  (${GREEN}HTTP${NC})"
    echo -e "  ${BLUE}https://$SERVER_IP:8443${NC} (${YELLOW}HTTPS${NC}, self-signed cert)"
}

main