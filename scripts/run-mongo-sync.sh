#!/bin/bash

# Wrapper script to run MongoDB region sync
# Usage: ./run-mongo-sync.sh [source_region] [target_region] [source_city] [target_city]

SOURCE_REGION="${1:-da}"
TARGET_REGION="${2:-cg}"
SOURCE_CITY="${3:-$SOURCE_REGION}"
TARGET_CITY="${4:-$TARGET_REGION}"
MONGO_POD="${MONGO_POD:-prod-mongodb-2}"
NAMESPACE="${NAMESPACE:-db}"

echo "========================================="
echo "MongoDB Region Sync"
echo "========================================="
echo "Source: $SOURCE_REGION / $SOURCE_CITY"
echo "Target: $TARGET_REGION / $TARGET_CITY"
echo "MongoDB Pod: $MONGO_POD"
echo "========================================="
echo ""

# Copy the sync script to the pod
echo "Copying sync script to MongoDB pod..."
kubectl cp scripts/sync-region-mongo.js $NAMESPACE/$MONGO_POD:/tmp/sync-region-mongo.js

# Run the sync script
echo "Running sync script..."
echo "Note: You may need to authenticate if prompted"
kubectl exec -it $MONGO_POD -n $NAMESPACE -- bash -c "mongo --eval \"var sourceRegion='$SOURCE_REGION'; var sourceCity='$SOURCE_CITY'; var targetRegion='$TARGET_REGION'; var targetCity='$TARGET_CITY';\" /tmp/sync-region-mongo.js"

# Cleanup
echo ""
echo "Cleaning up..."
kubectl exec -it $MONGO_POD -n $NAMESPACE -- rm -f /tmp/sync-region-mongo.js

echo ""
echo "✓ Done!"
