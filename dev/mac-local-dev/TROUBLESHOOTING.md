# Troubleshooting Carbide Local Development

## Common Issues and Solutions

### 1. Port Conflicts (RESOLVED)

Carbide-api now uses its own **dedicated vault on port 8201**, so it won't conflict with other vault instances (e.g., kind cluster on port 8200).

If you see any port conflicts:
```bash
# Check current status
cargo make --makefile dev/mac-local-dev/Makefile.toml diagnose
```

This will show:
- Carbide vault on port 8201 (dedicated for carbide-api)
- Other vaults (if any) on port 8200 (your existing services)

No action needed - they coexist peacefully!

### 2. Missing Vault Token

**Error:**
```
❌ No vault token found at /tmp/carbide-localdev-vault-root-token
```

**Cause:** Carbide vault didn't initialize properly.

**Solution:**
```bash
# Restart carbide vault
cargo make --makefile dev/mac-local-dev/Makefile.toml stop-docker
cargo make --makefile dev/mac-local-dev/Makefile.toml run-docker-vault
```

The token file should be automatically created at `/tmp/carbide-localdev-vault-root-token`.

### 3. Missing OAuth2 Environment Variables

**Error:**
```
Error: Internal error: CARBIDE_WEB_ALLOWED_ACCESS_GROUPS: environment variable not found
```

**Cause:** `CARBIDE_WEB_AUTH_TYPE=oauth2` is set but required OAuth2 variables are missing.

**Solution:**

**Option A: Use basic auth (recommended for local dev)**
```bash
# Don't set CARBIDE_WEB_AUTH_TYPE, or set it to basic
unset CARBIDE_WEB_AUTH_TYPE
# or
export CARBIDE_WEB_AUTH_TYPE=basic

cargo make --makefile dev/mac-local-dev/Makefile.toml run-mac-carbide
```
Auth is bypassed in local dev mode (bypass_rbac=true), so OAuth2 is not needed.

**Option B: Configure full OAuth2 (only if testing OAuth2 flows)**
Set all required environment variables - see README.md "Enable OAuth2 Mode" section.

### 4. Postgres Already Running

**Error:**
```
Postgres container 'pgdev' is already running
```

**Solution:** This is actually fine! The Makefile detects it and skips setup. If you want to restart with fresh data:
```bash
cargo make --makefile dev/mac-local-dev/Makefile.toml stop-docker
docker rm -f pgdev  # Remove existing container
cargo make --makefile dev/mac-local-dev/Makefile.toml run-mac-carbide
```

### 5. Certificate Generation Fails

**Error:** Issues with `gen-certs.sh`

**Solution:**
```bash
cd dev/certs/localhost
./gen-certs.sh
# If that fails, check that openssl is installed
which openssl
```

### 6. Migration Failures

**Error:** Database migration errors

**Solution:**
```bash
# Clean the database and restart
cargo make --makefile dev/mac-local-dev/Makefile.toml clean-postgres
cargo make --makefile dev/mac-local-dev/Makefile.toml run-mac-carbide
```

### 7. Vault Secrets Missing (SiteExplorer Errors)

**Error:**
```
level=ERROR msg="SiteExplorer run failed due to: Internal { message: \"Missing credential machines/bmc/site/root\" }"
```

**Cause:** Vault is running but doesn't have the required secrets configured.

**Solution:**
```bash
# Populate vault with required secrets
cargo make --makefile dev/mac-local-dev/Makefile.toml populate-vault-secrets

# Or run the script directly
VAULT_ADDR=http://localhost:8200 VAULT_TOKEN=root ./dev/mac-local-dev/populate-vault-secrets.sh
```

**Note:** `run-mac-carbide` now automatically runs `populate-vault-secrets`, so this should only happen if vault was set up externally.

### 8. Permission Denied on /opt/carbide/firmware

**Error:** `sudo mkdir` fails or requires password

**Solution:** This is expected. Enter your password when prompted. The directory is only created once.

## Quick Reset

To start completely fresh:

```bash
# Stop carbide containers only (doesn't affect other services)
cargo make --makefile dev/mac-local-dev/Makefile.toml stop-docker
docker rm -f pgdev carbide-vault 2>/dev/null || true

# Clean token
rm -f /tmp/carbide-localdev-vault-root-token

# Start fresh
cargo make --makefile dev/mac-local-dev/Makefile.toml run-mac-carbide
```

**Note:** This only affects carbide-api's dedicated containers. Your kind cluster or other vaults remain untouched.

## Diagnostic Commands

```bash
# Run the full diagnostic
cargo make --makefile dev/mac-local-dev/Makefile.toml diagnose

# Check if Docker is running
docker ps

# Check carbide vault status (port 8201)
curl -s http://localhost:8201/v1/sys/health | jq

# Check what's using ports
lsof -i :8200  # Other vaults
lsof -i :8201  # Carbide vault
lsof -i :5432  # Postgres

# Check if postgres is responding
psql -h localhost -U postgres -c "SELECT version();"

# Test carbide-api is running
grpcurl -plaintext localhost:1079 list
```

## Getting Help

If you're still having issues:

1. Run the diagnostic:
   ```bash
   cargo make --makefile dev/mac-local-dev/Makefile.toml diagnose
   ```
2. Check the full error output in terminal
3. Review this troubleshooting guide
4. Try the "Quick Reset" steps above
