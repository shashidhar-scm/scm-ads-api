#!/bin/bash

# RSS to Posters Runner Script
# Usage: ./run.sh [--dry-run]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== RSS to Posters Converter ===${NC}"
echo ""

# Check if .env file exists
if [ -f .env ]; then
    echo -e "${YELLOW}Loading configuration from .env${NC}"
    export $(cat .env | grep -v '^#' | xargs)
else
    echo -e "${YELLOW}No .env file found, using defaults${NC}"
fi

# Check if node_modules exists
if [ ! -d "node_modules" ]; then
    echo -e "${YELLOW}Installing dependencies...${NC}"
    npm install
    echo ""
fi

# Run the script
echo -e "${GREEN}Starting RSS to Posters conversion...${NC}"
echo ""

if [ "$1" == "--dry-run" ]; then
    echo -e "${YELLOW}Running in DRY RUN mode${NC}"
    node index.js --dry-run
else
    node index.js
fi

echo ""
echo -e "${GREEN}Done!${NC}"
