# Mac Build Status

## Current Issue

The carbide-core codebase currently **cannot be built on Mac** with `--no-default-features` due to missing feature guards on `measured_boot` handlers in the API code.

### Error

```
error[E0433]: failed to resolve: could not find `measured_boot` in `handlers`
```

This occurs because:
1. The `measured_boot` module is gated behind `#[cfg(feature = "linux-build")]`
2. The API trait implementations call `measured_boot` handlers without feature guards
3. When building with `--no-default-features` (required for Mac), the module doesn't exist but the calls remain

### Confirmed

- ✅ The old `nvmetal/carbide` repo **also fails** with the same error
- ✅ Using default features (which includes `linux-build`) **also fails** on Mac due to Linux-only dependencies (libudev, procfs)

## Options to Run Carbide-API on Mac

### Option 1: Fix the Code (Recommended for Long-term)

Add proper feature guards to all `measured_boot` handler calls in `crates/api/src/api.rs`.

**Estimated effort:** 2-4 hours to add `#[cfg(feature = "linux-build")]` to ~43 trait method implementations

**Steps:**
1. Wrap each `measured_boot` handler call with `#[cfg(feature = "linux-build")]`
2. Add stub implementations that return "Not supported" errors for non-Linux builds
3. Test compilation on Mac
4. Submit PR to fix this properly

### Option 2: Use Docker/Lima (Quick Workaround)

Run carbide-api in a Linux container on Mac.

**Setup:**
```bash
# Using Docker
docker run -it --rm -v $(pwd):/workspace -w /workspace \
  -p 1079:1079 -p 1080:1080 \
  --network host \
  rust:latest bash

# Inside container
apt-get update && apt-get install -y libssl-dev pkg-config
just run-mac-carbide
```

Or use Lima/Colima for a better Linux VM experience.

### Option 3: Use Remote Development

Use VS Code Remote SSH or similar to develop on a Linux machine.

### Option 4: Accept Partial Functionality

Modify the code to skip measured_boot handlers for Mac builds (not recommended for production).

## Recommendation

For immediate use: **Option 2** (Docker/Lima)

For long-term: **Option 1** (Fix the code and contribute back)

The justfile and setup scripts we created will work once the code compilation issue is resolved.

## Testing the Fix

Once the code is fixed, you can test with:

```bash
# Should compile successfully
cargo check --package carbide-api --no-default-features

# Should run successfully
just run-mac-carbide
```

## Related Files

- `crates/api/src/api.rs` - Contains the trait implementations that need feature guards
- `crates/api/src/handlers/mod.rs` - Already has proper feature guards on the module
- `crates/api/Cargo.toml` - Defines the `linux-build` feature

## Note

The infrastructure we set up (Makefile.toml tasks, vault setup, postgres, diagnostics) is all working correctly. The only blocker is the Rust compilation issue with feature flags.

All Mac-specific development tasks are self-contained in `dev/mac-local-dev/Makefile.toml`.
