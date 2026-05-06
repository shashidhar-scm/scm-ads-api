#!/bin/bash

# Run RSS to Posters inside Kubernetes MongoDB pod
# Usage: ./run-in-k8s.sh [--dry-run]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Configuration
NAMESPACE="${NAMESPACE:-db}"
POD_NAME="${POD_NAME:-prod-mongodb-0}"
MONGO_DB="${MONGO_DB:-cg}"
TARGET_CITY="${TARGET_CITY:-cg}"
TARGET_REGION="${TARGET_REGION:-cg}"
RSS_FEED_URL="${RSS_FEED_URL:-https://chicago.eater.com/rss/index.xml}"
POSTER_LIFESPAN_DAYS="${POSTER_LIFESPAN_DAYS:-7}"
MAX_ARTICLES="${MAX_ARTICLES:-10}"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${GREEN}=== RSS to Posters - Kubernetes Runner ===${NC}"
echo ""
echo "Configuration:"
echo "  Namespace: $NAMESPACE"
echo "  Pod: $POD_NAME"
echo "  MongoDB Database: $MONGO_DB"
echo "  Target City/Region: $TARGET_CITY/$TARGET_REGION"
echo "  RSS Feed: $RSS_FEED_URL"
echo "  Max Articles: $MAX_ARTICLES"
echo ""

# Check if pod exists
echo -e "${YELLOW}Checking if pod exists...${NC}"
if ! kubectl get pod "$POD_NAME" -n "$NAMESPACE" &>/dev/null; then
    echo -e "${RED}Error: Pod $POD_NAME not found in namespace $NAMESPACE${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Pod found${NC}"
echo ""

# Create temp directory in pod
echo -e "${YELLOW}Creating temp directory in pod...${NC}"
kubectl exec -it "$POD_NAME" -n "$NAMESPACE" -- mkdir -p /tmp/rss-to-posters
echo ""

# Copy files to pod
echo -e "${YELLOW}Copying files to pod...${NC}"
kubectl cp "$SCRIPT_DIR/package.json" "$NAMESPACE/$POD_NAME:/tmp/rss-to-posters/package.json"
kubectl cp "$SCRIPT_DIR/index.js" "$NAMESPACE/$POD_NAME:/tmp/rss-to-posters/index.js"
echo -e "${GREEN}✓ Files copied${NC}"
echo ""

# Install dependencies and run script
echo -e "${YELLOW}Installing dependencies and running script...${NC}"
echo ""

DRY_RUN_FLAG=""
if [ "$1" == "--dry-run" ]; then
    DRY_RUN_FLAG="--dry-run"
    echo -e "${YELLOW}Running in DRY RUN mode${NC}"
    echo ""
fi

kubectl exec -it "$POD_NAME" -n "$NAMESPACE" -- bash -c "
cd /tmp/rss-to-posters

# Install Node.js if not present
if ! command -v node &> /dev/null; then
    echo 'Node.js not found, installing...'
    curl -fsSL https://deb.nodesource.com/setup_18.x | bash -
    apt-get install -y nodejs
fi

# Install dependencies
echo 'Installing npm dependencies...'
npm install --production --silent

# Set environment variables and run
export MONGO_URI='mongodb://admin:asterisk@localhost:27017/admin?authSource=admin'
export MONGO_DB='$MONGO_DB'
export TARGET_CITY='$TARGET_CITY'
export TARGET_REGION='$TARGET_REGION'
export RSS_FEED_URL='$RSS_FEED_URL'
export POSTER_LIFESPAN_DAYS='$POSTER_LIFESPAN_DAYS'
export MAX_ARTICLES='$MAX_ARTICLES'

echo ''
echo '=== Running RSS to Posters Converter ==='
echo ''

node index.js $DRY_RUN_FLAG
"

EXIT_CODE=$?

echo ""
if [ $EXIT_CODE -eq 0 ]; then
    echo -e "${GREEN}✓ Script completed successfully${NC}"
else
    echo -e "${RED}✗ Script failed with exit code $EXIT_CODE${NC}"
fi

# Cleanup
echo ""
echo -e "${YELLOW}Cleaning up...${NC}"
kubectl exec -it "$POD_NAME" -n "$NAMESPACE" -- rm -rf /tmp/rss-to-posters
echo -e "${GREEN}✓ Cleanup complete${NC}"

exit $EXIT_CODE
