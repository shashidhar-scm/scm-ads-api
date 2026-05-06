#!/bin/bash

# Run RSS to Posters with MongoDB port-forward
# Usage: ./run-with-port-forward.sh [--dry-run]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Configuration
NAMESPACE="${NAMESPACE:-db}"
POD_NAME="${POD_NAME:-prod-mongodb-0}"
LOCAL_PORT="${LOCAL_PORT:-27017}"
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

echo -e "${GREEN}=== RSS to Posters - Port Forward Runner ===${NC}"
echo ""
echo "Configuration:"
echo "  Namespace: $NAMESPACE"
echo "  Pod: $POD_NAME"
echo "  Local Port: $LOCAL_PORT"
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

# Start port-forward in background
echo -e "${YELLOW}Starting port-forward...${NC}"
kubectl port-forward "$POD_NAME" -n "$NAMESPACE" "$LOCAL_PORT:27017" > /dev/null 2>&1 &
PORT_FORWARD_PID=$!

# Wait for port-forward to be ready
sleep 3

# Check if port-forward is running
if ! kill -0 $PORT_FORWARD_PID 2>/dev/null; then
    echo -e "${RED}Error: Port-forward failed to start${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Port-forward started (PID: $PORT_FORWARD_PID)${NC}"
echo ""

# Cleanup function
cleanup() {
    echo ""
    echo -e "${YELLOW}Stopping port-forward...${NC}"
    kill $PORT_FORWARD_PID 2>/dev/null || true
    echo -e "${GREEN}✓ Cleanup complete${NC}"
}

# Set trap to cleanup on exit
trap cleanup EXIT INT TERM

# Run the script
echo -e "${YELLOW}Running RSS to Posters converter...${NC}"
echo ""

cd "$SCRIPT_DIR"

DRY_RUN_FLAG=""
if [ "$1" == "--dry-run" ]; then
    DRY_RUN_FLAG="--dry-run"
    echo -e "${YELLOW}Running in DRY RUN mode${NC}"
    echo ""
fi

# Source nvm and run with Node 18
bash -c "
source ~/.nvm/nvm.sh
nvm use 18

export MONGO_URI='mongodb://admin:asterisk@localhost:$LOCAL_PORT/admin?authSource=admin'
export MONGO_DB='$MONGO_DB'
export TARGET_CITY='$TARGET_CITY'
export TARGET_REGION='$TARGET_REGION'
export RSS_FEED_URL='$RSS_FEED_URL'
export POSTER_LIFESPAN_DAYS='$POSTER_LIFESPAN_DAYS'
export MAX_ARTICLES='$MAX_ARTICLES'

node index.js $DRY_RUN_FLAG
"

EXIT_CODE=$?

echo ""
if [ $EXIT_CODE -eq 0 ]; then
    echo -e "${GREEN}✓ Script completed successfully${NC}"
else
    echo -e "${RED}✗ Script failed with exit code $EXIT_CODE${NC}"
fi

exit $EXIT_CODE
