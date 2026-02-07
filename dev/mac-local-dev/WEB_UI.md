# Carbide Web UI Access

## Quick Access

Once `carbide-api` is running, open your browser to:

### Main URL
**https://localhost:1079/admin**

**Notes:** 
- The web UI is mounted under the `/admin` path. All web pages use this prefix.
- ⚠️ **Self-signed certificate**: You'll see a browser warning. Click through it to access the site (this is normal for local development).
- **Both HTTP/2 (gRPC) and HTTPS work** with this configuration.

## Available Pages

### Overview & Management
- **Main Dashboard**: https://localhost:1079/admin/
- **Domain Info**: https://localhost:1079/admin/domain

### Machine Management
- **All DPUs**: http://localhost:1079/admin/dpu
- **DPU Versions**: http://localhost:1079/admin/dpu/versions
- **Explored Endpoints**: http://localhost:1079/admin/explored-endpoint
  - **Paired Endpoints**: http://localhost:1079/admin/explored-endpoint/paired
  - **Unpaired Endpoints**: http://localhost:1079/admin/explored-endpoint/unpaired
- **Expected Machines**: http://localhost:1079/admin/expected-machine
- **Managed Hosts**: http://localhost:1079/admin/managed-host
- **Switches**: http://localhost:1079/admin/switch
- **Racks**: http://localhost:1079/admin/rack
- **Power Shelves**: http://localhost:1079/admin/power-shelf

### Networking
- **Network Segments**: http://localhost:1079/admin/network-segment
- **Network Security Groups**: http://localhost:1079/admin/network-security-group
- **Network Devices**: http://localhost:1079/admin/network-device
- **Network Status**: http://localhost:1079/admin/network-status
- **Interfaces**: http://localhost:1079/admin/interface
- **Resource Pools**: http://localhost:1079/admin/resource-pool

### InfiniBand
- **IB Fabrics**: http://localhost:1079/admin/ib-fabric
- **IB Partitions**: http://localhost:1079/admin/ib-partition
- **NVLink Partitions**: http://localhost:1079/admin/nvlink-partition

### Virtual Infrastructure
- **VPCs**: http://localhost:1079/admin/vpc
- **Instances**: http://localhost:1079/admin/instance
- **Instance Types**: http://localhost:1079/admin/instance-type
- **DPAs**: http://localhost:1079/admin/dpa

### Multi-tenancy
- **Tenants**: http://localhost:1079/admin/tenant
- **Tenant Keysets**: http://localhost:1079/admin/tenant-keyset

### SKUs
- **SKU List**: http://localhost:1079/admin/sku

### Health & Validation
- **Machine Health**: http://localhost:1079/admin/health
- **Machine Validation**: http://localhost:1079/admin/validation

### Attestation & Security
- **Attestation Summary**: http://localhost:1079/admin/attestation

### Browser Tools
- **Redfish Browser**: http://localhost:1079/admin/redfish-browser
- **UFM Browser**: http://localhost:1079/admin/ufm-browser
- **NMXM Browser**: http://localhost:1079/admin/nmxm-browser

### History & State
- **Machine State History**: http://localhost:1079/admin/machine-state-history
- **Switch State History**: http://localhost:1079/admin/switch-state-history
- **Power Shelf State History**: http://localhost:1079/admin/power-shelf-state-history

## Authentication

With local development configuration (`bypass_rbac = true` and `permissive_mode = true`):
- **Authentication is bypassed** - no credentials needed
- If you see a basic auth prompt (rare), use any credentials:
  - Username: `admin`
  - Password: `Welcome123`

## JSON APIs

Most pages have corresponding JSON endpoints by adding `.json` to the path:
- HTML: http://localhost:1079/admin/dpu
- JSON: http://localhost:1079/admin/dpu.json

Example:
```bash
# Get DPU list as JSON
curl http://localhost:1079/admin/dpu.json

# Get domain info as JSON
curl http://localhost:1079/admin/domain.json
```

## Troubleshooting

### Cannot Connect to Web UI

**Error**: Browser shows "Connection refused" or "This site can't be reached"

**Solutions**:
1. Verify carbide-api is running:
   ```bash
   lsof -i :1079
   ```

2. Check logs for errors:
   ```bash
   # If running in terminal, check the output
   # Or check recent logs
   cargo make --makefile dev/mac-local-dev/Makefile.toml diagnose
   ```

3. Restart carbide-api:
   ```bash
   cargo make --makefile dev/mac-local-dev/Makefile.toml stop-docker
   cargo make --makefile dev/mac-local-dev/Makefile.toml run-mac-carbide
   ```

### Page Shows Empty Data

**Cause**: No machines/resources have been discovered yet (normal for fresh install)

**What to expect**:
- Fresh installation: Most pages will be empty or show "No items found"
- After site exploration: The SiteExplorer will discover endpoints and populate machine data
- Watch the logs for `SiteExplorer` activity

**To manually trigger exploration** (advanced):
- Navigate to: http://localhost:1079/admin/explored-endpoint
- Enter BMC IP addresses of machines in your environment

## gRPC API

The same port (1079) also serves the gRPC API:

```bash
# List available gRPC services
grpcurl -plaintext localhost:1079 list

# Describe a specific service
grpcurl -plaintext localhost:1079 describe forge.Forge
```

## Related Files

- Configuration: `dev/mac-local-dev/carbide-api-config.toml`
- Templates: `crates/api/templates/*.html`
- Web handlers: `crates/api/src/web/`
