#!/usr/bin/env bash
# Generate real cryptographic material for the vault+configs e2e test.
# Output: examples/e2e-vault-configs/fixtures/*.pem|*.pub|*.txt
#         examples/e2e-vault-configs/fixtures/suffix.txt (unique handle suffix)

set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FIX="$HERE/fixtures"
mkdir -p "$FIX"

# --- unique handle suffix (8 hex chars from /dev/urandom) ---
if [[ ! -f "$FIX/suffix.txt" || "${FORCE_NEW_SUFFIX:-0}" == "1" ]]; then
  head -c 4 /dev/urandom | xxd -p > "$FIX/suffix.txt"
fi
SUFFIX="$(cat "$FIX/suffix.txt")"
echo "Handle suffix: $SUFFIX"

# --- TLS bundle: CA + server cert signed by CA + server private key ---
echo "Generating TLS bundle..."
openssl genrsa -out "$FIX/ca.key" 2048 2>/dev/null
openssl req -x509 -new -nodes -key "$FIX/ca.key" -sha256 -days 365 \
  -subj "/CN=Bahriya Test CA/O=Bahriya E2E" \
  -out "$FIX/ca.pem" 2>/dev/null

openssl genrsa -out "$FIX/server.key" 2048 2>/dev/null
openssl req -new -key "$FIX/server.key" \
  -subj "/CN=e2e-${SUFFIX}.bahriya.test/O=Bahriya E2E" \
  -out "$FIX/server.csr" 2>/dev/null
openssl x509 -req -in "$FIX/server.csr" -CA "$FIX/ca.pem" -CAkey "$FIX/ca.key" \
  -CAcreateserial -days 365 -sha256 \
  -out "$FIX/server.pem" 2>/dev/null
rm -f "$FIX/server.csr" "$FIX/ca.srl" "$FIX/ca.key"

# --- standalone X509 cert (self-signed) ---
echo "Generating X509 cert..."
openssl req -x509 -new -newkey rsa:2048 -nodes \
  -keyout "$FIX/x509-throwaway.key" \
  -subj "/CN=e2e-x509-${SUFFIX}.bahriya.test/O=Bahriya E2E" \
  -out "$FIX/x509.pem" -days 365 2>/dev/null
rm -f "$FIX/x509-throwaway.key"

# --- SSH keypair (ed25519) ---
echo "Generating SSH keypair..."
rm -f "$FIX/ssh" "$FIX/ssh.pub"
ssh-keygen -t ed25519 -N '' -C "e2e-${SUFFIX}@bahriya.test" -f "$FIX/ssh" >/dev/null

# --- GPG keypair (uses isolated GNUPGHOME so we don't pollute user keyring) ---
echo "Generating GPG keypair..."
GHOME="$(mktemp -d)"
trap 'rm -rf "$GHOME"' EXIT
GNUPGHOME="$GHOME" gpg --batch --pinentry-mode loopback --passphrase '' \
  --quick-generate-key "Bahriya E2E ${SUFFIX} <e2e-${SUFFIX}@bahriya.test>" \
  ed25519 default never 2>/dev/null
KEYID="$(GNUPGHOME="$GHOME" gpg --list-keys --with-colons | awk -F: '/^pub:/ {print $5; exit}')"
GNUPGHOME="$GHOME" gpg --export --armor "$KEYID" > "$FIX/gpg.pub"
GNUPGHOME="$GHOME" gpg --export-secret-keys --armor --pinentry-mode loopback \
  --passphrase '' "$KEYID" > "$FIX/gpg.key"

# --- Encryption key (32 random bytes, base64) ---
echo "Generating encryption key..."
openssl rand -base64 32 | tr -d '\n' > "$FIX/encryption.key"

# --- Env file content ---
cat > "$FIX/env-file.txt" <<'EOF'
# E2E env file
APP_MODE=production
LOG_LEVEL=info
FEATURE_X_ENABLED=true
EOF

# --- YAML config ---
cat > "$FIX/config.yaml" <<EOF
service:
  name: e2e-${SUFFIX}
  replicas: 1
  features:
    - alpha
    - beta
EOF

# --- JSON config ---
cat > "$FIX/config.json" <<EOF
{
  "service": "e2e-${SUFFIX}",
  "replicas": 1,
  "features": ["alpha", "beta"]
}
EOF

# --- Plain config ---
cat > "$FIX/config.plain" <<EOF
# Plain text config for e2e-${SUFFIX}
key1=value1
key2=value2
EOF

echo
echo "Fixtures generated in $FIX"
ls -1 "$FIX"
