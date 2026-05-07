#!/usr/bin/env bash
# bootstrap.sh — cross-platform installer for ceftop
# Usage: bash <(curl -fsSL https://raw.githubusercontent.com/LuisPalacios/ceftop/main/scripts/bootstrap.sh)
set -euo pipefail

REPO="LuisPalacios/ceftop"
GITHUB_API="https://api.github.com"
VERSION_TAG=""
INSTALL_DIR="$HOME/bin"
NO_DESKTOP=false
PLATFORM=""
ARCH=""
ARTIFACT_NAME=""
DOWNLOAD_URL=""
RELEASE_TAG=""
TMP_DIR=""
USE_GH=false

# ── Output helpers ──────────────────────────────────────────────────

red()    { printf '\033[0;31m%s\033[0m\n' "$*"; }
green()  { printf '\033[0;32m%s\033[0m\n' "$*"; }
yellow() { printf '\033[0;33m%s\033[0m\n' "$*"; }
bold()   { printf '\033[1m%s\033[0m\n' "$*"; }

log()  { green  "[ceftop] $*"; }
warn() { yellow "[ceftop] $*"; }
die()  { red    "[ceftop] $*"; exit 1; }

# ── Help ────────────────────────────────────────────────────────────

show_help() {
  cat <<'HELP'
ceftop installer — download and install the ceftop GUI

Usage:
  bash <(curl -fsSL https://raw.githubusercontent.com/LuisPalacios/ceftop/main/scripts/bootstrap.sh) [OPTIONS]

Options:
  --version <tag>   Install a specific release (e.g. v0.2.0). Default: latest.
  --prefix <dir>    Linux/Windows install directory. Default: ~/bin.
                    macOS always installs to /Applications.
  --no-desktop      Linux only: skip writing a .desktop entry. The binary
                    is still installed; launch it from $INSTALL_DIR.
  -h, --help        Show this help.

Examples:
  # Install latest
  bash <(curl -fsSL https://raw.githubusercontent.com/LuisPalacios/ceftop/main/scripts/bootstrap.sh)

  # Install a specific version
  bash <(curl -fsSL https://raw.githubusercontent.com/LuisPalacios/ceftop/main/scripts/bootstrap.sh) --version v0.2.0

  # Custom Linux/Windows install directory
  bash <(curl -fsSL https://raw.githubusercontent.com/LuisPalacios/ceftop/main/scripts/bootstrap.sh) --prefix ~/.local/bin
HELP
  exit 0
}

# ── Argument parsing ────────────────────────────────────────────────

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --version)    VERSION_TAG="${2:?'--version requires a tag (e.g. v0.2.0)'}"; shift 2 ;;
      --prefix)     INSTALL_DIR="${2:?'--prefix requires a directory'}"; shift 2 ;;
      --no-desktop) NO_DESKTOP=true; shift ;;
      -h|--help)    show_help ;;
      *)            die "Unknown option: $1 (try --help)" ;;
    esac
  done
}

# ── Platform detection ──────────────────────────────────────────────

detect_platform() {
  case "${OSTYPE:-}" in
    darwin*)       PLATFORM="macos" ;;
    linux*)        PLATFORM="linux" ;;
    msys*|mingw*|cygwin*) PLATFORM="windows" ;;
    *)
      local uname_s
      uname_s="$(uname -s 2>/dev/null || true)"
      case "$uname_s" in
        Darwin)       PLATFORM="macos"   ;;
        Linux)        PLATFORM="linux"   ;;
        MINGW*|MSYS*) PLATFORM="windows" ;;
        *)            die "Unsupported OS: ${OSTYPE:-$uname_s}" ;;
      esac
      ;;
  esac

  local machine
  machine="$(uname -m)"
  case "$machine" in
    arm64|aarch64) ARCH="arm64" ;;
    x86_64|amd64)  ARCH="amd64" ;;
    *)             die "Unsupported architecture: $machine" ;;
  esac

  case "${PLATFORM}-${ARCH}" in
    macos-arm64)   ARTIFACT_NAME="ceftop-macos-arm64.zip" ;;
    macos-amd64)   ARTIFACT_NAME="ceftop-macos-amd64.zip" ;;
    linux-amd64)   ARTIFACT_NAME="ceftop-linux-amd64.zip" ;;
    windows-amd64) ARTIFACT_NAME="ceftop-win-amd64.zip" ;;
    windows-arm64) ARTIFACT_NAME="ceftop-win-arm64.zip" ;;
    linux-arm64)   die "Linux arm64 builds are not available yet." ;;
    *)             die "Unsupported platform/arch combo: ${PLATFORM}/${ARCH}" ;;
  esac

  log "Detected: $PLATFORM/$ARCH → $ARTIFACT_NAME"
}

# ── Dependency check ────────────────────────────────────────────────

check_dependencies() {
  local missing=()

  command -v curl &>/dev/null || missing+=("curl")

  if [[ "$PLATFORM" != "windows" ]]; then
    command -v unzip &>/dev/null || missing+=("unzip")
  fi

  if [[ ${#missing[@]} -gt 0 ]]; then
    die "Missing required tools: ${missing[*]}. Install them and try again."
  fi

  if command -v gh &>/dev/null && gh auth status &>/dev/null 2>&1; then
    USE_GH=true
  fi
}

# ── Download release ────────────────────────────────────────────────

get_release_info() {
  TMP_DIR="$(mktemp -d)"
  trap 'rm -rf "$TMP_DIR"' EXIT

  if [[ "$USE_GH" == true ]]; then
    log "Downloading via gh CLI..."
    local gh_args=(release download --repo "$REPO" --pattern "$ARTIFACT_NAME" --dir "$TMP_DIR")
    if [[ -n "$VERSION_TAG" ]]; then
      gh_args=(release download "$VERSION_TAG" --repo "$REPO" --pattern "$ARTIFACT_NAME" --dir "$TMP_DIR")
    fi
    if ! gh "${gh_args[@]}"; then
      die "Failed to download $ARTIFACT_NAME from $REPO. Check the version tag and try again."
    fi
    if [[ -n "$VERSION_TAG" ]]; then
      RELEASE_TAG="$VERSION_TAG"
    else
      RELEASE_TAG="$(gh release view --repo "$REPO" --json tagName -q .tagName 2>/dev/null || echo "latest")"
    fi
  else
    log "Downloading via GitHub API..."
    local api_url
    if [[ -n "$VERSION_TAG" ]]; then
      api_url="${GITHUB_API}/repos/${REPO}/releases/tags/${VERSION_TAG}"
    else
      api_url="${GITHUB_API}/repos/${REPO}/releases/latest"
    fi

    local api_response http_code
    http_code="$(curl -fsSL -w '%{http_code}' -o "$TMP_DIR/api.json" "$api_url" 2>/dev/null || true)"

    if [[ "$http_code" == "403" ]]; then
      die "GitHub API rate limit hit. Set GITHUB_TOKEN env var or install gh CLI (gh auth login)."
    elif [[ "$http_code" != "200" ]]; then
      die "Failed to fetch release info (HTTP $http_code). Check the version tag and network."
    fi

    api_response="$(<"$TMP_DIR/api.json")"

    RELEASE_TAG="$(printf '%s' "$api_response" | grep -m1 '"tag_name"' | sed 's/.*: *"\([^"]*\)".*/\1/')"
    DOWNLOAD_URL="$(printf '%s' "$api_response" | grep -o "https://[^\"]*/${ARTIFACT_NAME}" | head -1)"

    if [[ -z "$DOWNLOAD_URL" ]]; then
      die "Artifact $ARTIFACT_NAME not found in release $RELEASE_TAG."
    fi

    log "Downloading $ARTIFACT_NAME ($RELEASE_TAG)..."

    local curl_cmd
    if [[ -n "${GITHUB_TOKEN:-}" ]]; then
      curl_cmd=(curl -fSL --progress-bar -H "Authorization: token $GITHUB_TOKEN" -o "$TMP_DIR/$ARTIFACT_NAME" "$DOWNLOAD_URL")
    else
      curl_cmd=(curl -fSL --progress-bar -o "$TMP_DIR/$ARTIFACT_NAME" "$DOWNLOAD_URL")
    fi

    if ! "${curl_cmd[@]}"; then
      die "Download failed. Check your network and try again."
    fi
  fi
}

# ── Extract ─────────────────────────────────────────────────────────

extract_archive() {
  log "Extracting..."
  mkdir -p "$TMP_DIR/extracted"

  local zip_path="$TMP_DIR/$ARTIFACT_NAME"

  # Sanity: download step must have left the archive here.
  if [[ ! -f "$zip_path" ]]; then
    die "Archive missing at $zip_path — gh download or curl failed silently."
  fi

  if command -v unzip &>/dev/null; then
    unzip -o -q "$zip_path" -d "$TMP_DIR/extracted" \
      || die "unzip failed on $zip_path"
  elif [[ "$PLATFORM" == "windows" ]]; then
    # PowerShell needs Windows-style paths; Git Bash hands us Unix-style
    # ones (/c/Users/...). cygpath does the translation.
    local win_zip win_dest
    win_zip="$(cygpath -w "$zip_path")"
    win_dest="$(cygpath -w "$TMP_DIR/extracted")"
    powershell -NoProfile -Command "Expand-Archive -Force -Path '$win_zip' -DestinationPath '$win_dest'" \
      || die "Expand-Archive failed. Install unzip (pacman -S unzip in Git Bash) or extract manually."
  else
    die "unzip is required but not found."
  fi

  # Confirm the expected payload survived extraction. Without this, a silent
  # zero-byte extract leads to a confusing cp-failure later.
  local expected_file
  case "$PLATFORM" in
    macos)         expected_file="$TMP_DIR/extracted/CefTopApp.app" ;;
    linux)         expected_file="$TMP_DIR/extracted/CefTopApp" ;;
    windows)       expected_file="$TMP_DIR/extracted/CefTopApp.exe" ;;
  esac
  if [[ ! -e "$expected_file" ]]; then
    die "Extracted archive does not contain $expected_file (contents: $(ls -A "$TMP_DIR/extracted" 2>/dev/null | tr '\n' ' '))"
  fi
}

# ── Existing install detection ──────────────────────────────────────

detect_existing_install() {
  case "$PLATFORM" in
    macos)
      [[ -d "/Applications/CefTopApp.app" ]] && warn "Existing /Applications/CefTopApp.app found — replacing with $RELEASE_TAG."
      ;;
    linux)
      [[ -x "$INSTALL_DIR/CefTopApp" ]] && warn "Existing $INSTALL_DIR/CefTopApp found — replacing with $RELEASE_TAG."
      ;;
    windows)
      [[ -f "$INSTALL_DIR/CefTopApp.exe" ]] && warn "Existing $INSTALL_DIR/CefTopApp.exe found — replacing with $RELEASE_TAG."
      ;;
  esac
  # When the file does NOT exist, the `[[ -f ]] && warn` compound exits 1
  # (short-circuit). Without an explicit success here, the function returns
  # that 1 and `set -e` kills the entire script when control returns to
  # main() — which is the silent fresh-install failure mode we hit.
  return 0
}

# ── PATH helper ─────────────────────────────────────────────────────

ensure_path() {
  local dir="$1"
  local marker="# ceftop"

  if echo "$PATH" | tr ':' '\n' | grep -qx "$dir"; then
    return
  fi

  local rc_file=""
  case "$PLATFORM" in
    macos)   rc_file="$HOME/.zshrc" ;;
    linux)
      if [[ "$(basename "${SHELL:-/bin/bash}")" == "zsh" ]]; then
        rc_file="$HOME/.zshrc"
      else
        rc_file="$HOME/.bashrc"
      fi
      ;;
    windows) rc_file="$HOME/.bashrc" ;;
  esac

  if [[ -z "$rc_file" ]]; then
    warn "Could not determine shell rc file. Add $dir to your PATH manually."
    return
  fi

  if [[ -f "$rc_file" ]] && grep -qF "$marker" "$rc_file"; then
    return
  fi

  log "Adding $dir to PATH in $rc_file"
  printf '\n%s\nexport PATH="%s:$PATH"\n' "$marker" "$dir" >> "$rc_file"
}

# ── macOS install ───────────────────────────────────────────────────

install_macos() {
  rm -rf /Applications/CefTopApp.app
  cp -R "$TMP_DIR/extracted/CefTopApp.app" /Applications/CefTopApp.app
  # Strip "downloaded from internet" quarantine so Gatekeeper opens the
  # bundle without re-prompting on every launch. The first launch still
  # requires the user to right-click → Open (unsigned binaries).
  xattr -cr /Applications/CefTopApp.app 2>/dev/null || true
  log "GUI installed: /Applications/CefTopApp.app"
}

# ── Linux install ───────────────────────────────────────────────────

write_linux_desktop_entry() {
  local exec_path="$1"
  local apps_dir="$HOME/.local/share/applications"
  local icons_dir="$HOME/.local/share/icons/hicolor/256x256/apps"
  local desktop_file="$apps_dir/ceftop.desktop"
  local icon_path="$icons_dir/ceftop.png"
  local icon_url="https://raw.githubusercontent.com/${REPO}/main/assets/appicon.png"

  mkdir -p "$apps_dir" "$icons_dir"

  if curl -fsSL -o "$icon_path" "$icon_url"; then
    log "Icon installed: $icon_path"
  else
    warn "Could not download icon — desktop entry will use a generic one."
    icon_path="utilities-system-monitor"
  fi

  cat > "$desktop_file" <<DESKTOP
[Desktop Entry]
Type=Application
Name=CefTop
GenericName=CEF/Electron Process Monitor
Comment=Map the multi-process tree of CEF/Electron/Chromium apps
Exec=$exec_path
Icon=$icon_path
Terminal=false
Categories=Development;System;Monitor;
StartupWMClass=CefTopApp
DESKTOP

  if command -v update-desktop-database &>/dev/null; then
    update-desktop-database "$apps_dir" 2>/dev/null || true
  fi
  log "Desktop entry installed: $desktop_file"
}

install_linux() {
  mkdir -p "$INSTALL_DIR"
  cp "$TMP_DIR/extracted/CefTopApp" "$INSTALL_DIR/CefTopApp"
  chmod +x "$INSTALL_DIR/CefTopApp"
  log "GUI installed: $INSTALL_DIR/CefTopApp"

  if [[ "$NO_DESKTOP" == false ]]; then
    write_linux_desktop_entry "$INSTALL_DIR/CefTopApp"
  fi

  ensure_path "$INSTALL_DIR"
}

# ── Windows (Git Bash) install ──────────────────────────────────────

install_windows() {
  mkdir -p "$INSTALL_DIR"

  local win_path
  win_path="$(cygpath -w "$INSTALL_DIR" 2>/dev/null || echo "$INSTALL_DIR")"

  cp "$TMP_DIR/extracted/CefTopApp.exe" "$INSTALL_DIR/CefTopApp.exe"
  # Remove the "downloaded from internet" zone identifier so SmartScreen
  # does not block the unsigned binary on first launch.
  powershell -Command "Unblock-File -Path '${win_path}\\CefTopApp.exe'" 2>/dev/null || true
  log "GUI installed: $INSTALL_DIR/CefTopApp.exe"
  log "Windows path: $win_path"

  ensure_path "$INSTALL_DIR"
}

# ── Summary ─────────────────────────────────────────────────────────

print_summary() {
  echo ""
  bold "── ceftop $RELEASE_TAG installed ──"
  echo ""

  case "$PLATFORM" in
    macos)   echo "  GUI:  /Applications/CefTopApp.app" ;;
    linux)
      echo "  GUI:  $INSTALL_DIR/CefTopApp"
      [[ "$NO_DESKTOP" == false ]] && echo "  Menu: registered — search 'CefTop' or pin it to the dock"
      ;;
    windows) echo "  GUI:  $INSTALL_DIR/CefTopApp.exe" ;;
  esac

  echo ""

  if ! echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
    local rc_file=""
    case "$PLATFORM" in
      macos) rc_file="~/.zshrc" ;;
      linux)
        if [[ "$(basename "${SHELL:-/bin/bash}")" == "zsh" ]]; then
          rc_file="~/.zshrc"
        else
          rc_file="~/.bashrc"
        fi
        ;;
      windows) rc_file="~/.bashrc" ;;
    esac
    if [[ -n "$rc_file" ]]; then
      bold "  Reload your shell to pick up PATH changes:"
      echo "    source $rc_file"
      echo ""
    fi
  fi

  bold "  Get started:"
  case "$PLATFORM" in
    macos)   echo "    open -a CefTopApp" ;;
    linux)   echo "    CefTopApp   # or launch from Activities" ;;
    windows) echo "    CefTopApp.exe" ;;
  esac
  echo ""
}

# ── Main ────────────────────────────────────────────────────────────

main() {
  parse_args "$@"
  detect_platform
  check_dependencies
  get_release_info
  extract_archive
  detect_existing_install

  case "$PLATFORM" in
    macos)   install_macos   ;;
    linux)   install_linux   ;;
    windows) install_windows ;;
  esac

  print_summary
}

main "$@"
