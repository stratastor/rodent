#!/bin/bash
# Upload Rodent artifacts to Cloudflare R2
# Copyright 2025 The StrataSTOR Authors and Contributors
# SPDX-License-Identifier: Apache-2.0

set -e
set -o pipefail

# Colors
RED='\033[38;2;255;183;178m'
GREEN='\033[38;2;152;251;152m'
YELLOW='\033[38;2;230;230;250m'
BLUE='\033[38;2;176;224;230m'
NC='\033[0m'

print_info() {
    echo -e "${BLUE}ℹ${NC} $@"
}

print_success() {
    echo -e "${GREEN}✓${NC} $@"
}

print_error() {
    echo -e "${RED}✗${NC} $@" >&2
}

print_header() {
    echo -e "${BLUE}================================================${NC}"
    echo -e "${BLUE}  Uploading to Cloudflare R2${NC}"
    echo -e "${BLUE}================================================${NC}"
    echo ""
}

# Required environment variables
REQUIRED_VARS=(
    "CF_ACCOUNT_ID"
    "CF_R2_ACCESS_KEY_ID"
    "CF_R2_SECRET_ACCESS_KEY"
    "R2_S3_API"
    "VERSION"
)

# Check required variables
for var in "${REQUIRED_VARS[@]}"; do
    if [ -z "${!var}" ]; then
        print_error "Required environment variable not set: $var"
        exit 1
    fi
done

# Default values
DIST_DIR="${DIST_DIR:-dist}"
UPLOAD_TYPE="${UPLOAD_TYPE:-binary}"  # binary or script

print_header

print_info "Upload configuration:"
print_info "  Version: $VERSION"
print_info "  Type: $UPLOAD_TYPE"
print_info "  Directory: $DIST_DIR"
print_info "  S3 API: $R2_S3_API"
echo ""

# Configure AWS CLI for R2
print_info "Configuring AWS CLI for R2..."

export AWS_ACCESS_KEY_ID="$CF_R2_ACCESS_KEY_ID"
export AWS_SECRET_ACCESS_KEY="$CF_R2_SECRET_ACCESS_KEY"
export AWS_DEFAULT_REGION="auto"
export AWS_ENDPOINT_URL="$R2_S3_API"

# Test connection by checking if we can access the specific bucket
# Note: Some R2 tokens may not have ListBuckets permission, so we test with the target bucket
TEST_BUCKET="pkg"
if [ "$UPLOAD_TYPE" = "script" ]; then
    TEST_BUCKET="utils"
fi

print_info "Testing R2 connection to bucket: $TEST_BUCKET"
if ! aws s3 ls "s3://$TEST_BUCKET/" --endpoint-url "$R2_S3_API" &> /dev/null; then
    print_error "Failed to connect to R2 bucket: $TEST_BUCKET"
    print_error "Please check credentials and bucket permissions."
    exit 1
fi

print_success "R2 connection verified (bucket: $TEST_BUCKET)"

# Upload based on type
if [ "$UPLOAD_TYPE" = "binary" ]; then
    # Upload binaries to pkg bucket
    BUCKET="pkg"

    print_info "Uploading binaries to bucket: $BUCKET"

    for file in "$DIST_DIR"/rodent-linux-*; do
        if [ ! -f "$file" ]; then
            print_error "Binary file not found: $file"
            continue
        fi

        filename=$(basename "$file")

        # Upload to versioned path
        print_info "Uploading $filename to rodent/$VERSION/$filename..."
        aws s3 cp \
            --endpoint-url "$R2_S3_API" \
            "$file" \
            "s3://$BUCKET/rodent/$VERSION/$filename" \
            --content-type "application/octet-stream"

        # Upload checksum if exists
        if [ -f "$file.sha256" ]; then
            print_info "Uploading checksum for $filename..."
            aws s3 cp \
                --endpoint-url "$R2_S3_API" \
                "$file.sha256" \
                "s3://$BUCKET/rodent/$VERSION/$filename.sha256" \
                --content-type "text/plain"
        fi

        print_success "Uploaded $filename"
    done

    # Update 'latest' symlink by copying files (skip if already latest)
    if [ "$VERSION" != "latest" ]; then
        print_info "Updating 'latest' version..."
        for file in "$DIST_DIR"/rodent-linux-*; do
            if [ ! -f "$file" ]; then
                continue
            fi

            filename=$(basename "$file")

            # Copy to latest
            aws s3 cp \
                --endpoint-url "$R2_S3_API" \
                "s3://$BUCKET/rodent/$VERSION/$filename" \
                "s3://$BUCKET/rodent/latest/$filename"

            # Copy checksum to latest
            if [ -f "$file.sha256" ]; then
                aws s3 cp \
                    --endpoint-url "$R2_S3_API" \
                    "s3://$BUCKET/rodent/$VERSION/$filename.sha256" \
                    "s3://$BUCKET/rodent/latest/$filename.sha256"
            fi
        done

        print_success "Updated 'latest' version"
    else
        print_info "Version is 'latest', skipping copy step"
    fi

elif [ "$UPLOAD_TYPE" = "script" ]; then
    # Upload scripts to utils bucket
    BUCKET="utils"

    print_info "Uploading scripts to bucket: $BUCKET"

    # Upload bootstrap script as install.sh
    if [ -f "$DIST_DIR/bootstrap.sh" ]; then
        print_info "Uploading bootstrap script as install.sh..."
        aws s3 cp \
            --endpoint-url "$R2_S3_API" \
            "$DIST_DIR/bootstrap.sh" \
            "s3://$BUCKET/install.sh" \
            --content-type "text/x-shellscript" \
            --cache-control "max-age=300"

        print_success "Uploaded install.sh"
    fi

    # Upload full installer
    if [ -f "$DIST_DIR/install-rodent.sh" ]; then
        print_info "Uploading full installer..."
        aws s3 cp \
            --endpoint-url "$R2_S3_API" \
            "$DIST_DIR/install-rodent.sh" \
            "s3://$BUCKET/rodent/install-rodent.sh" \
            --content-type "text/x-shellscript" \
            --cache-control "max-age=3600"

        print_success "Uploaded install-rodent.sh"
    fi

    # Upload uninstall script
    if [ -f "$DIST_DIR/uninstall-rodent.sh" ]; then
        print_info "Uploading uninstall script..."
        aws s3 cp \
            --endpoint-url "$R2_S3_API" \
            "$DIST_DIR/uninstall-rodent.sh" \
            "s3://$BUCKET/rodent/uninstall-rodent.sh" \
            --content-type "text/x-shellscript" \
            --cache-control "max-age=3600"

        print_success "Uploaded uninstall-rodent.sh"
    fi

    # Upload setup script
    if [ -f "$DIST_DIR/setup_rodent_user.sh" ]; then
        print_info "Uploading setup script..."
        aws s3 cp \
            --endpoint-url "$R2_S3_API" \
            "$DIST_DIR/setup_rodent_user.sh" \
            "s3://$BUCKET/rodent/setup_rodent_user.sh" \
            --content-type "text/x-shellscript" \
            --cache-control "max-age=3600"

        print_success "Uploaded setup_rodent_user.sh"
    fi

    # Upload sample config
    if [ -f "$DIST_DIR/rodent.sample.yml" ]; then
        print_info "Uploading sample config..."
        aws s3 cp \
            --endpoint-url "$R2_S3_API" \
            "$DIST_DIR/rodent.sample.yml" \
            "s3://$BUCKET/rodent/rodent.sample.yml" \
            --content-type "text/plain" \
            --cache-control "max-age=3600"

        print_success "Uploaded rodent.sample.yml"
    fi

    # Upload service file
    if [ -f "$DIST_DIR/rodent.service" ]; then
        print_info "Uploading service file..."
        aws s3 cp \
            --endpoint-url "$R2_S3_API" \
            "$DIST_DIR/rodent.service" \
            "s3://$BUCKET/rodent/rodent.service" \
            --content-type "text/plain" \
            --cache-control "max-age=3600"

        print_success "Uploaded rodent.service"
    fi

else
    print_error "Unknown upload type: $UPLOAD_TYPE"
    print_error "Valid types: binary, script"
    exit 1
fi

echo ""
print_success "Upload completed successfully"

# Print download URLs
if [ "$UPLOAD_TYPE" = "binary" ]; then
    echo ""
    print_info "Download URLs:"
    print_info "  https://pkg.strata.host/rodent/$VERSION/rodent-linux-amd64"
    print_info "  https://pkg.strata.host/rodent/$VERSION/rodent-linux-arm64"
    print_info "  https://pkg.strata.host/rodent/latest/rodent-linux-amd64"
    print_info "  https://pkg.strata.host/rodent/latest/rodent-linux-arm64"
elif [ "$UPLOAD_TYPE" = "script" ]; then
    echo ""
    print_info "Installation URL:"
    print_info "  curl -fsSL https://utils.strata.host/install.sh | sudo bash"
fi
