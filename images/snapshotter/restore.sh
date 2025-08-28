#!/bin/sh

# This file relies on the following env variables to be set:
# S3_PROVIDER
# S3_BUCKET_NAME
# S3_ACCESS_KEY_ID
# S3_SECRET_ACCESS_KEY
# S3_ENDPOINT_URL
# S3_BUCKET_NAME
# PATHFINDER_NETWORK
# PATHFINDER_FILE_NAME
# PATHFINDER_CHECKSUM
# EXTRACT_DIR: Defaults to /scratch
# DATA_DIR: Defaults to /data
# RSYNC_CONFIG: If set, replaces the automatically generated configuration

set -e

# Default to the standard system for now
S3_PROVIDER=${S3_PROVIDER:-Cloudflare}
S3_BUCKET_NAME=${S3_BUCKET_NAME:-"pathfinder-snapshots"}
S3_ACCESS_KEY_ID=${S3_ACCESS_KEY_ID:-"7635ce5752c94f802d97a28186e0c96d"}
S3_SECRET_ACCESS_KEY=${S3_SECRET_ACCESS_KEY:-"529f8db483aae4df4e2a781b9db0c8a3a7c75c82ff70787ba2620310791c7821"}
S3_ENDPOINT_URL=${S3_ENDPOINT_URL:-"https://cbf011119e7864a873158d83f3304e27.r2.cloudflarestorage.com"}

EXTRACT_DIR=${EXTRACT_DIR:-/scratch}
DATA_DIR=${DATA_DIR:-/data}
echo "Starting snapshot download and extraction process..."

# Trap to ensure cleanup happens even if script fails
cleanup() {
    echo "Cleaning up scratch space..."
    rm -rf $EXTRACT_DIR/* 2>/dev/null || true
    rm -rf $EXTRACT_DIR/.* 2>/dev/null || true
    echo "Scratch space cleanup completed."
}
trap cleanup EXIT

create_config() {
    echo "Creating configuration..."
    if [ -z "$RSYNC_CONFIG" ]; then
        # Add your configuration creation logic here
        tee /app/rclone.conf <<EOF
[pathfinder-snapshots]
type = s3
provider = $S3_PROVIDER
env_auth = false
access_key_id = $S3_ACCESS_KEY_ID
secret_access_key = $S3_SECRET_ACCESS_KEY
endpoint = $S3_ENDPOINT_URL
acl = private
EOF
    else
        echo "$RSYNC_CONFIG" > /app/rclone.conf
    fi
}

# Create directories
mkdir -p $EXTRACT_DIR
mkdir -p $DATA_DIR

# Create the configuration
create_config

echo "Downloading snapshot: $PATHFINDER_FILE_NAME from remote server"
echo "Disk usage before download:"
rclone copy -P --config /app/rclone.conf pathfinder-snapshots:$S3_BUCKET_NAME/$PATHFINDER_FILE_NAME $EXTRACT_DIR/

echo "Verifying checksum..."
ACTUAL_CHECKSUM=$(sha256sum /scratch/$PATHFINDER_FILE_NAME | cut -d' ' -f1)

if [ "$ACTUAL_CHECKSUM" != "$PATHFINDER_CHECKSUM" ]; then
echo "Checksum mismatch! Expected: $PATHFINDER_CHECKSUM, Got: $ACTUAL_CHECKSUM"
exit 1
fi

echo "Checksum verified. Extracting snapshot..."
zstd -d /scratch/$PATHFINDER_FILE_NAME -o $DATA_DIR/${PATHFINDER_NETWORK}.sqlite

echo "Snapshot extraction completed successfully."
echo "Database file ready at: $DATA_DIR/${PATHFINDER_NETWORK}.sqlite"
