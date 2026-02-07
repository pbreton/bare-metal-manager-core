#!/usr/bin/env bash
#
# Quick fix script to add feature guards for Mac builds
#

set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

echo "=== Applying Mac Build Fix ==="
echo ""
echo "This script will add #[cfg(feature = \"linux-build\")] guards to measured_boot handlers."
echo ""

# Backup the original file
cp crates/api/src/api.rs crates/api/src/api.rs.backup
echo "✓ Created backup: crates/api/src/api.rs.backup"

# Find all measured_boot handler implementations and add feature guards
perl -i -pe 's{^(\s*)(async fn \w+.*measured_boot)}{$1#[cfg(feature = "linux-build")]\n$1$2}g' crates/api/src/api.rs

echo "✓ Added feature guards to measured_boot handlers"
echo ""
echo "Testing compilation..."

if cargo check --package carbide-api --no-default-features 2>&1 | grep -q "error"; then
    echo "❌ Compilation still has errors. Restoring backup..."
    mv crates/api/src/api.rs.backup crates/api/src/api.rs
    echo ""
    echo "The automated fix didn't work. Manual code changes are needed."
    echo "See MAC_BUILD_STATUS.md for details."
    exit 1
fi

echo "✓ Compilation successful!"
echo ""
echo "Mac build fix applied successfully."
echo "You can now run: just run-mac-carbide"
echo ""
echo "To revert: mv crates/api/src/api.rs.backup crates/api/src/api.rs"
