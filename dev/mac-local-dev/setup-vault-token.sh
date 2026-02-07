#!/usr/bin/env bash
#
# Helper script to set up vault token for local development
#

set -euo pipefail

echo "=== Carbide Local Dev - Vault Token Setup ==="
echo ""

# Check if vault is accessible on port 8200
if ! curl -s http://localhost:8200/v1/sys/health >/dev/null 2>&1; then
    echo "❌ No vault found on port 8200"
    echo ""
    echo "To start a new vault, run:"
    echo "  just run-docker-vault"
    exit 1
fi

echo "✓ Vault is accessible on port 8200"
echo ""

# Check if we already have a token
if [ -f /tmp/localdev-docker-vault-root-token ]; then
    echo "Token file exists at /tmp/localdev-docker-vault-root-token"
    EXISTING_TOKEN=$(cat /tmp/localdev-docker-vault-root-token | tr -d '\n\r ')
    
    # Check if token is not empty
    if [ -n "$EXISTING_TOKEN" ] && [ "$EXISTING_TOKEN" != "" ]; then
        # Test if it works
        if curl -s -H "X-Vault-Token: $EXISTING_TOKEN" http://localhost:8200/v1/sys/health 2>&1 | grep -q "initialized"; then
            echo "✓ Existing token is valid"
            echo ""
            echo "You're all set! Run: just run-mac-carbide"
            exit 0
        else
            echo "⚠️  Token exists but appears invalid, will try to find a new one"
        fi
    else
        echo "⚠️  Token file is empty, will try to find a valid token"
    fi
fi

echo "Checking for kind cluster with vault..."

# Check if this is a kind cluster vault
if docker ps --format '{{.Names}}' | grep -q 'carbide-local-control-plane'; then
    echo "✓ Found kind cluster: carbide-local"
    echo ""
    echo "For kind cluster vault, you have a few options:"
    echo ""
    echo "1. Use a development/test token (if vault is in dev mode):"
    echo "   echo 'root' > /tmp/localdev-docker-vault-root-token"
    echo ""
    echo "2. Try to extract the token from kubernetes:"
    echo "   just get-kind-vault-token"
    echo ""
    echo "3. Check vault logs for the token:"
    echo "   kubectl logs -n carbide deployment/vault | grep 'Root Token'"
    echo ""
    echo "4. If vault was initialized, check the init output file if it was saved"
    echo ""
    
    # Try common development tokens
    echo "Trying common development tokens..."
    for token in "root" "dev-token" "myroot"; do
        if curl -s -H "X-Vault-Token: $token" http://localhost:8200/v1/sys/health 2>/dev/null | grep -q "initialized"; then
            echo "✓ Found working token: $token"
            echo "$token" > /tmp/localdev-docker-vault-root-token
            echo "✓ Token saved to /tmp/localdev-docker-vault-root-token"
            echo ""
            echo "You're all set! Run: just run-mac-carbide"
            exit 0
        fi
    done
    
    echo "❌ None of the common tokens worked"
    echo ""
    echo "Please manually set the token:"
    echo "  echo 'YOUR_TOKEN' > /tmp/localdev-docker-vault-root-token"
    exit 1
else
    echo "⚠️  Unknown vault setup"
    echo ""
    echo "Please manually set the token:"
    echo "  echo 'YOUR_TOKEN' > /tmp/localdev-docker-vault-root-token"
    echo ""
    echo "Or start a fresh vault:"
    echo "  just stop-docker"
    echo "  just run-docker-vault"
    exit 1
fi
