# Carbide Local Development Setup - Summary

## What Changed

Carbide-api now uses a **dedicated vault container** that won't interfere with your existing vault instances:

### Before
- Tried to reuse any vault on port 8200
- Conflicted with kind cluster or other vaults
- Required manual token management

### After
- **Dedicated vault**: `carbide-vault` container on port **8201**
- **No conflicts**: Your kind cluster vault (port 8200) remains untouched
- **Auto-configured**: Secrets and PKI automatically initialized
- **Clean separation**: Stop carbide services without affecting other projects

## Quick Reference

### Start Carbide API
```bash
# Everything is automatic
cargo make --makefile dev/mac-local-dev/Makefile.toml run-mac-carbide
```

### Check Status
```bash
cargo make --makefile dev/mac-local-dev/Makefile.toml diagnose
```

### Stop Carbide Services Only
```bash
cargo make --makefile dev/mac-local-dev/Makefile.toml stop-docker
```

## Architecture

```
Port 8200: Your existing vault (kind cluster, etc.) ← Untouched
Port 8201: Carbide vault (carbide-vault container) ← Dedicated for carbide-api
Port 5432: Carbide postgres (pgdev container)      ← Dedicated for carbide-api
Port 1079: Carbide API gRPC service
```

## Key Files

- **Container**: `carbide-vault` (not `vault`)
- **Token file**: `/tmp/carbide-localdev-vault-root-token`
- **Vault address**: `http://localhost:8201` (not 8200)

## Secrets Included

The `carbide-vault` container is automatically initialized with:
- BMC credentials: `secrets/machines/bmc/site/root`
- DPU auth: `secrets/machines/all_dpus/site_default/uefi-metadata-items/auth`
- Host auth: `secrets/machines/all_hosts/site_default/uefi-metadata-items/auth`
- PKI configuration at `certs/` mount

## What Errors Are Fixed

✅ **SiteExplorer errors** - No more "Missing credential machines/bmc/site/root"
✅ **Port conflicts** - Carbide uses 8201, won't conflict with your kind cluster
✅ **Token confusion** - Dedicated token file for carbide only
✅ **Vault interference** - Your other vaults are completely isolated

## If You Need to Start Fresh

```bash
# Clean slate for carbide only
cargo make --makefile dev/mac-local-dev/Makefile.toml stop-docker
docker rm -f pgdev carbide-vault
rm -f /tmp/carbide-localdev-vault-root-token

# Restart
cargo make --makefile dev/mac-local-dev/Makefile.toml run-mac-carbide
```

Your kind cluster and other services remain untouched!
