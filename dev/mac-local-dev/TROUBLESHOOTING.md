# Troubleshooting Carbide Local Development

## Common Issues and Solutions

### 1. Port 8200 Already in Use

**Error:**
```
docker: Error response from daemon: Bind for 0.0.0.0:8200 failed: port is already allocated
```

**Cause:** Another service (likely a kind cluster or existing vault) is using port 8200.

**Solutions:**

**Option A: Use the existing vault (recommended)**
```bash
# Run the helper script to detect and configure the token
cargo make --makefile dev/mac-local-dev/Makefile.toml setup-vault-token

# Then run carbide-api
cargo make --makefile dev/mac-local-dev/Makefile.toml run-mac-carbide
```

**Option B: Stop the conflicting service**
```bash
# If it's a kind cluster
kind delete cluster --name carbide-local

# If it's a standalone vault container
docker stop vault

# Then run carbide-api
cargo make --makefile dev/mac-local-dev/Makefile.toml run-mac-carbide
```

### 2. Missing Vault Token

**Error:**
```
❌ No vault token found!
```

**Solution:**
```bash
# Try the automatic setup
cargo make --makefile dev/mac-local-dev/Makefile.toml setup-vault-token

# Or manually get it from kind cluster
cargo make --makefile dev/mac-local-dev/Makefile.toml get-kind-vault-token

# Or provide it via environment variable
export VAULT_TOKEN=your-token-here
cargo make --makefile dev/mac-local-dev/Makefile.toml run-mac-carbide
```

### 3. SOPS_AGE_KEY_FILE Issues

**Error:**
```
Failed to get the data key required to decrypt the SOPS file
```

**Cause:** You have `FORGED_DIRECTORY` set but SOPS can't decrypt the secrets.

**Solution:**

**Option A: Unset FORGED_DIRECTORY (recommended for local dev)**
```bash
unset FORGED_DIRECTORY
cargo make --makefile dev/mac-local-dev/Makefile.toml run-mac-carbide
```
Auth is bypassed in local dev mode, so you don't need real OAuth2 credentials.

**Option B: Configure SOPS properly**
```bash
export SOPS_AGE_KEY_FILE=~/.config/sops/age/keys.txt
# Make sure the key file exists and has the correct key
```

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

### 7. Permission Denied on /opt/carbide/firmware

**Error:** `sudo mkdir` fails or requires password

**Solution:** This is expected. Enter your password when prompted. The directory is only created once.

## Quick Reset

To start completely fresh:

```bash
# Stop all containers
cargo make --makefile dev/mac-local-dev/Makefile.toml stop-docker
docker rm -f pgdev vault 2>/dev/null || true

# Clean tokens
rm -f /tmp/localdev-docker-vault-root-token

# Stop kind cluster if running
kind delete cluster --name carbide-local

# Start fresh
cargo make --makefile dev/mac-local-dev/Makefile.toml run-mac-carbide
```

## Diagnostic Commands

```bash
# Check if required tools are installed
cargo make --version
which docker cargo sops kubectl

# Check if Docker is running
docker ps

# Check what's using port 8200
lsof -i :8200

# Check vault status
curl -s http://localhost:8200/v1/sys/health | jq

# Check if postgres is responding
psql -h localhost -U postgres -c "SELECT version();"

# Test carbide-api is running
grpcurl -plaintext localhost:1079 list
```

## Getting Help

If you're still having issues:

1. Check the full error output
2. Review this troubleshooting guide
3. Try the "Quick Reset" steps above
4. Check if you have a kind cluster that might be conflicting:
   ```bash
   kind get clusters
   docker ps
   ```
