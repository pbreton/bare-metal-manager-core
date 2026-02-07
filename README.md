# Carbide - Bare Metal Provisioning

Carbide delivers zero-touch lifecycle automation for bare-metal systems that secures datacenter infrastructure at its foundation.

It is an API-based microservice that provides site-local, zero-trust, bare-metal lifecycle management with DPU-enforced isolation. Carbide helps automate
the complexity of the bare-metal lifecycle, putting NCPs and ISVs on the fast track to building next generation AI Cloud offerings.

## Getting Started

- Go to the [Carbide overview](docs/overview.md) to get an overview of Carbide architecture and capabilities.
- Or jump to the [Installation guide](docs/installation/index.md) to start setting up your site for Carbide.



## Development

### Running Carbide API on Mac

⚠️ **Current Status**: Mac builds are currently blocked by missing feature guards in the codebase. See [dev/mac-local-dev/MAC_BUILD_STATUS.md](dev/mac-local-dev/MAC_BUILD_STATUS.md) for details and workarounds.

Mac-specific development tasks are in `dev/mac-local-dev/Makefile.toml`.

**Quick Start (once build issue is resolved):**

```bash
# Install cargo-make if you haven't already
cargo install cargo-make

# Run carbide-api (will set up everything automatically)
cargo make --makefile dev/mac-local-dev/Makefile.toml run-mac-carbide
```

**Current Workaround:**

Use Docker to run in a Linux environment:
```bash
docker run -it --rm -v $(pwd):/workspace -w /workspace \
  -p 1079:1079 -p 1080:1080 --network host \
  rust:latest bash

# Inside container
apt-get update && apt-get install -y libssl-dev pkg-config
cargo make --makefile dev/mac-local-dev/Makefile.toml run-mac-carbide
```

If you encounter any issues (port conflicts, missing tokens, etc.):

```bash
# Run diagnostics
cargo make --makefile dev/mac-local-dev/Makefile.toml diagnose

# Run the setup helper
cargo make --makefile dev/mac-local-dev/Makefile.toml setup-vault-token

# Then try again
cargo make --makefile dev/mac-local-dev/Makefile.toml run-mac-carbide
```

This will automatically:
- Detect and configure existing services (vault, postgres) or start new ones
- Run database migrations
- Start the carbide-api service with authentication bypassed for local development

**Prerequisites:**
- Docker Desktop (running)
- Rust toolchain
- [cargo-make](https://github.com/sagiegurari/cargo-make): `cargo install cargo-make`

**Note:** The local development config uses `bypass_rbac = true` and `permissive_mode = true`, so no external credentials are needed.

For more details and troubleshooting, see:
- [dev/mac-local-dev/README.md](dev/mac-local-dev/README.md) - Mac development guide
- [dev/mac-local-dev/TROUBLESHOOTING.md](dev/mac-local-dev/TROUBLESHOOTING.md) - Common issues
- [dev/mac-local-dev/MAC_BUILD_STATUS.md](dev/mac-local-dev/MAC_BUILD_STATUS.md) - Current build status
