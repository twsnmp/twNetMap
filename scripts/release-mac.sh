#!/bin/bash
set -euo pipefail

# Print steps with nice colors
info() {
  echo -e "\033[1;34m[INFO]\033[0m $1"
}

success() {
  echo -e "\033[1;32m[SUCCESS]\033[0m $1"
}

error() {
  echo -e "\033[1;31m[ERROR]\033[0m $1" >&2
  exit 1
}

warn() {
  echo -e "\033[1;33m[WARNING]\033[0m $1"
}

# Check for required tools
for tool in wails codesign pkgbuild productsign xcrun; do
  if ! command -v "$tool" &> /dev/null; then
    error "$tool is required but not installed. Exiting."
  fi
done

info "Starting macOS Release Build and Packaging Process"

# Resolve version from Git tag and commit hash
VERSION=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.1.0")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_VERSION="${VERSION}(${COMMIT})"
info "Build version: ${BUILD_VERSION}"

# Step 1: Build the frontend explicitly (wails build may skip this)
info "Building frontend assets..."
(cd frontend && npm install && npm run build)
success "Frontend built successfully."

# Step 2: Build the application using Wails
info "Building Wails application for macOS (Universal)..."
wails build -platform darwin/universal -clean -ldflags "-X main.version=${BUILD_VERSION}"

ORIG_APP_PATH="build/bin/twNetMap.app"
if [ ! -d "$ORIG_APP_PATH" ]; then
  error "Build failed: $ORIG_APP_PATH not found."
fi

mkdir -p dist
APP_PATH="dist/twNetMap.app"
rm -rf "$APP_PATH"
mv "$ORIG_APP_PATH" "$APP_PATH"

success "Application built and prepared successfully at $APP_PATH"

# Step 2: Select/Determine Application Signing Identity
APP_IDENTITY=${MACOS_APP_IDENTITY:-""}

if [ -z "$APP_IDENTITY" ]; then
  info "MACOS_APP_IDENTITY is not set. Scanning Keychain for valid 'Developer ID Application' certificates..."
  
  # Fetch and parse identities
  identities=$(security find-identity -v)
  app_certs=()
  
  while IFS= read -r line; do
    if [[ "$line" =~ \"(Developer\ ID\ Application:[^\"]+)\" ]]; then
      app_certs+=("${BASH_REMATCH[1]}")
    fi
  done <<< "$identities"

  if [ ${#app_certs[@]} -eq 0 ]; then
    error "No valid 'Developer ID Application' certificates found in your Keychain.\nPlease set MACOS_APP_IDENTITY environment variable or import a valid certificate."
  elif [ ${#app_certs[@]} -eq 1 ]; then
    APP_IDENTITY="${app_certs[0]}"
    info "Found exactly one certificate. Automatically selected: $APP_IDENTITY"
  else
    info "Multiple certificates found. Please select one:"
    for i in "${!app_certs[@]}"; do
      echo "  $((i+1))) ${app_certs[i]}"
    done
    
    while true; do
      read -p "Select certificate number (1-${#app_certs[@]}): " cert_idx < /dev/tty
      if [[ "$cert_idx" =~ ^[0-9]+$ ]] && [ "$cert_idx" -ge 1 ] && [ "$cert_idx" -le "${#app_certs[@]}" ]; then
        APP_IDENTITY="${app_certs[$((cert_idx-1))]}"
        break
      else
        warn "Invalid selection. Please enter a number between 1 and ${#app_certs[@]}."
      fi
    done
  fi
fi

info "Selected Application Identity: $APP_IDENTITY"

# Step 3: Sign the .app bundle with Hardened Runtime
info "Signing the application bundle with hardened runtime..."
codesign --force --options runtime --sign "$APP_IDENTITY" --deep "$APP_PATH"
success "Application bundle signed successfully."

# Step 4: Select/Determine Installer Signing Identity
PKG_IDENTITY=${MACOS_PKG_IDENTITY:-""}

if [ -z "$PKG_IDENTITY" ]; then
  info "MACOS_PKG_IDENTITY is not set. Scanning Keychain for valid 'Developer ID Installer' certificates..."
  
  # Fetch and parse identities
  identities=$(security find-identity -v)
  pkg_certs=()
  
  while IFS= read -r line; do
    if [[ "$line" =~ \"(Developer\ ID\ Installer:[^\"]+)\" ]]; then
      pkg_certs+=("${BASH_REMATCH[1]}")
    fi
  done <<< "$identities"

  if [ ${#pkg_certs[@]} -eq 0 ]; then
    error "No valid 'Developer ID Installer' certificates found in your Keychain.\nPlease set MACOS_PKG_IDENTITY environment variable or import a valid certificate."
  elif [ ${#pkg_certs[@]} -eq 1 ]; then
    PKG_IDENTITY="${pkg_certs[0]}"
    info "Found exactly one installer certificate. Automatically selected: $PKG_IDENTITY"
  else
    info "Multiple installer certificates found. Please select one:"
    for i in "${!pkg_certs[@]}"; do
      echo "  $((i+1))) ${pkg_certs[i]}"
    done
    
    while true; do
      read -p "Select installer certificate number (1-${#pkg_certs[@]}): " cert_idx < /dev/tty
      if [[ "$cert_idx" =~ ^[0-9]+$ ]] && [ "$cert_idx" -ge 1 ] && [ "$cert_idx" -le "${#pkg_certs[@]}" ]; then
        PKG_IDENTITY="${pkg_certs[$((cert_idx-1))]}"
        break
      else
        warn "Invalid selection. Please enter a number between 1 and ${#pkg_certs[@]}."
      fi
    done
  fi
fi

info "Selected Installer Identity: $PKG_IDENTITY"

# Step 5: Package the signed .app into an unsigned .pkg (disabling relocation)
UNSIGNED_PKG="dist/twNetMap-unsigned.pkg"
SIGNED_PKG="dist/twNetMap.pkg"
COMPONENT_PLIST="dist/components.plist"

info "Creating the installer package..."
# Generate component plist to configure package behavior
pkgbuild --analyze --root "dist" "$COMPONENT_PLIST"
# Set BundleIsRelocatable to false so macOS installer doesn't relocate the app to development folders
plutil -replace 0.BundleIsRelocatable -bool NO "$COMPONENT_PLIST"

# Build package using the component plist
pkgbuild --root "dist" --component-plist "$COMPONENT_PLIST" --install-location "/Applications" "$UNSIGNED_PKG"
rm -f "$COMPONENT_PLIST"
success "Installer package created at $UNSIGNED_PKG"

# Step 6: Sign the installer package
info "Signing the installer package..."
productsign --sign "$PKG_IDENTITY" "$UNSIGNED_PKG" "$SIGNED_PKG"
rm -f "$UNSIGNED_PKG"
success "Signed installer package created at $SIGNED_PKG"

# Step 7: Notarize the package
NOTARY_PROFILE=${MACOS_NOTARY_PROFILE:-""}

if [ -z "$NOTARY_PROFILE" ]; then
  info "MACOS_NOTARY_PROFILE environment variable is not set."
  read -p "Enter Apple Notarization profile name (or press Enter to exit/skip notarization): " NOTARY_PROFILE < /dev/tty
  if [ -z "$NOTARY_PROFILE" ]; then
    warn "Apple Notarization profile is required to complete notarization. Skipping notarization."
    info "You can notarize manually or rerun after setting MACOS_NOTARY_PROFILE environment variable."
    exit 0
  fi
fi

info "Submitting the signed package to Apple for notarization using profile: $NOTARY_PROFILE..."
xcrun notarytool submit "$SIGNED_PKG" --keychain-profile "$NOTARY_PROFILE" --wait

info "Stapling notarization ticket to the installer package..."
xcrun stapler staple "$SIGNED_PKG"

success "macOS packaging and notarization completed successfully! Signed and notarized installer is at: $SIGNED_PKG"
