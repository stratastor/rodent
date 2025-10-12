#!/bin/bash
# Development upload script - uploads installation scripts to R2
# This file is for local development convenience only

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CREDS_FILE="$SCRIPT_DIR/.r2-credentials"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Check if credentials file exists
if [ ! -f "$CREDS_FILE" ]; then
    echo "Error: Credentials file not found at $CREDS_FILE"
    exit 1
fi

# Source credentials
source "$CREDS_FILE"

# Export for AWS CLI
export AWS_ACCESS_KEY_ID="$CF_R2_ACCESS_KEY_ID"
export AWS_SECRET_ACCESS_KEY="$CF_R2_SECRET_ACCESS_KEY"
export AWS_DEFAULT_REGION="auto"
export AWS_ENDPOINT_URL="$R2_S3_API"

cd "$PROJECT_ROOT"

echo "Uploading installation scripts to R2..."
echo ""

# Upload bootstrap (no cache - always fresh)
echo "→ Uploading bootstrap script..."
aws s3 cp --endpoint-url "$AWS_ENDPOINT_URL" \
    scripts/install/bootstrap.sh s3://utils/install.sh \
    --content-type "text/x-shellscript" --cache-control "no-cache" --acl public-read

# Upload installer (no cache)
echo "→ Uploading full installer..."
aws s3 cp --endpoint-url "$AWS_ENDPOINT_URL" \
    scripts/install/install-rodent.sh s3://utils/rodent/install-rodent.sh \
    --content-type "text/x-shellscript" --cache-control "no-cache" --acl public-read

# Upload uninstall script (no cache)
echo "→ Uploading uninstall script..."
aws s3 cp --endpoint-url "$AWS_ENDPOINT_URL" \
    scripts/install/uninstall-rodent.sh s3://utils/rodent/uninstall-rodent.sh \
    --content-type "text/x-shellscript" --cache-control "no-cache" --acl public-read

# Upload setup script (no cache)
echo "→ Uploading setup script..."
aws s3 cp --endpoint-url "$AWS_ENDPOINT_URL" \
    scripts/setup_rodent_user.sh s3://utils/rodent/setup_rodent_user.sh \
    --content-type "text/x-shellscript" --cache-control "no-cache" --acl public-read

# Upload sample config (no cache)
echo "→ Uploading sample config..."
aws s3 cp --endpoint-url "$AWS_ENDPOINT_URL" \
    scripts/rodent.sample.yml s3://utils/rodent/rodent.sample.yml \
    --content-type "text/plain" --cache-control "no-cache" --acl public-read

# Upload service file (no cache)
echo "→ Uploading service file..."
aws s3 cp --endpoint-url "$AWS_ENDPOINT_URL" \
    scripts/rodent.service s3://utils/rodent/rodent.service \
    --content-type "text/plain" --cache-control "no-cache" --acl public-read

echo ""
echo "✓ All scripts uploaded successfully!"
echo ""
echo "Installation URL:"
echo "  curl -fsSL https://utils.strata.host/install.sh | sudo bash"
