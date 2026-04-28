#!/bin/bash

# MongoDB Region Sync Script
# Syncs ACTIVE and SCHEDULED posters and adposters from source region to target region

set -e

# Configuration
SOURCE_REGION="${SOURCE_REGION:-da}"
SOURCE_CITY="${SOURCE_CITY:-da}"
TARGET_REGION="${TARGET_REGION:-cg}"
TARGET_CITY="${TARGET_CITY:-cg}"
MONGO_POD="${MONGO_POD:-prod-mongodb-2}"
NAMESPACE="${NAMESPACE:-default}"

echo "========================================="
echo "MongoDB Region Sync"
echo "========================================="
echo "Source: $SOURCE_REGION / $SOURCE_CITY"
echo "Target: $TARGET_REGION / $TARGET_CITY"
echo "MongoDB Pod: $MONGO_POD"
echo "========================================="

# Create temporary directory for exports
TEMP_DIR=$(mktemp -d)
echo "Using temp directory: $TEMP_DIR"

# Export posters from source region
echo ""
echo "Step 1: Exporting posters from $SOURCE_REGION..."
kubectl exec -it $MONGO_POD -n $NAMESPACE -- mongoexport \
  --db=$SOURCE_REGION \
  --collection=posters \
  --query='{"$or":[{"status":"ACTIVE"},{"status":"SCHEDULED"}]}' \
  --out=/tmp/posters_export.json

# Copy export from pod to local
echo "Copying posters export from pod..."
kubectl cp $NAMESPACE/$MONGO_POD:/tmp/posters_export.json $TEMP_DIR/posters_export.json

# Transform the data - update region and city fields
echo ""
echo "Step 2: Transforming posters data..."
cat > $TEMP_DIR/transform_posters.js << 'EOF'
const fs = require('fs');
const readline = require('readline');

const sourceRegion = process.env.SOURCE_REGION || 'da';
const sourceCity = process.env.SOURCE_CITY || 'da';
const targetRegion = process.env.TARGET_REGION || 'cg';
const targetCity = process.env.TARGET_CITY || 'cg';

const inputFile = process.argv[2];
const outputFile = process.argv[3];

const rl = readline.createInterface({
  input: fs.createReadStream(inputFile),
  crlfDelay: Infinity
});

const output = fs.createWriteStream(outputFile);

rl.on('line', (line) => {
  try {
    const doc = JSON.parse(line);
    
    // Update region and city fields
    if (doc.region) doc.region = targetRegion;
    if (doc.city) doc.city = targetCity;
    if (doc.region_name) doc.region_name = targetRegion;
    if (doc.city_name) doc.city_name = targetCity;
    
    // Update any nested location fields
    if (doc.location) {
      if (doc.location.region) doc.location.region = targetRegion;
      if (doc.location.city) doc.location.city = targetCity;
    }
    
    output.write(JSON.stringify(doc) + '\n');
  } catch (e) {
    console.error('Error parsing line:', e.message);
  }
});

rl.on('close', () => {
  output.end();
  console.log('Transformation complete');
});
EOF

# Run transformation using Node.js (if available) or use sed as fallback
if command -v node &> /dev/null; then
  SOURCE_REGION=$SOURCE_REGION SOURCE_CITY=$SOURCE_CITY \
  TARGET_REGION=$TARGET_REGION TARGET_CITY=$TARGET_CITY \
  node $TEMP_DIR/transform_posters.js \
    $TEMP_DIR/posters_export.json \
    $TEMP_DIR/posters_transformed.json
else
  echo "Node.js not found, using sed for transformation..."
  sed -e "s/\"region\":\"$SOURCE_REGION\"/\"region\":\"$TARGET_REGION\"/g" \
      -e "s/\"city\":\"$SOURCE_CITY\"/\"city\":\"$TARGET_CITY\"/g" \
      -e "s/\"region_name\":\"$SOURCE_REGION\"/\"region_name\":\"$TARGET_REGION\"/g" \
      -e "s/\"city_name\":\"$SOURCE_CITY\"/\"city_name\":\"$TARGET_CITY\"/g" \
      $TEMP_DIR/posters_export.json > $TEMP_DIR/posters_transformed.json
fi

# Copy transformed data back to pod
echo ""
echo "Step 3: Copying transformed posters to pod..."
kubectl cp $TEMP_DIR/posters_transformed.json $NAMESPACE/$MONGO_POD:/tmp/posters_import.json

# Import posters to target region
echo "Importing posters to $TARGET_REGION..."
kubectl exec -it $MONGO_POD -n $NAMESPACE -- mongoimport \
  --db=$TARGET_REGION \
  --collection=posters \
  --file=/tmp/posters_import.json \
  --mode=upsert \
  --upsertFields=_id

echo "✓ Posters sync complete"

# Export adposters from source region
echo ""
echo "Step 4: Exporting adposters from $SOURCE_REGION..."
kubectl exec -it $MONGO_POD -n $NAMESPACE -- mongoexport \
  --db=$SOURCE_REGION \
  --collection=ad_posters \
  --query='{"$or":[{"status":"ACTIVE"},{"status":"SCHEDULED"}]}' \
  --out=/tmp/adposters_export.json

# Copy export from pod to local
echo "Copying adposters export from pod..."
kubectl cp $NAMESPACE/$MONGO_POD:/tmp/adposters_export.json $TEMP_DIR/adposters_export.json

# Transform adposters data
echo ""
echo "Step 5: Transforming adposters data..."
if command -v node &> /dev/null; then
  SOURCE_REGION=$SOURCE_REGION SOURCE_CITY=$SOURCE_CITY \
  TARGET_REGION=$TARGET_REGION TARGET_CITY=$TARGET_CITY \
  node $TEMP_DIR/transform_posters.js \
    $TEMP_DIR/adposters_export.json \
    $TEMP_DIR/adposters_transformed.json
else
  sed -e "s/\"region\":\"$SOURCE_REGION\"/\"region\":\"$TARGET_REGION\"/g" \
      -e "s/\"city\":\"$SOURCE_CITY\"/\"city\":\"$TARGET_CITY\"/g" \
      -e "s/\"region_name\":\"$SOURCE_REGION\"/\"region_name\":\"$TARGET_REGION\"/g" \
      -e "s/\"city_name\":\"$SOURCE_CITY\"/\"city_name\":\"$TARGET_CITY\"/g" \
      $TEMP_DIR/adposters_export.json > $TEMP_DIR/adposters_transformed.json
fi

# Copy transformed data back to pod
echo ""
echo "Step 6: Copying transformed adposters to pod..."
kubectl cp $TEMP_DIR/adposters_transformed.json $NAMESPACE/$MONGO_POD:/tmp/adposters_import.json

# Import adposters to target region
echo "Importing adposters to $TARGET_REGION..."
kubectl exec -it $MONGO_POD -n $NAMESPACE -- mongoimport \
  --db=$TARGET_REGION \
  --collection=ad_posters \
  --file=/tmp/adposters_import.json \
  --mode=upsert \
  --upsertFields=_id

echo "✓ Adposters sync complete"

# Cleanup
echo ""
echo "Step 7: Cleaning up..."
kubectl exec -it $MONGO_POD -n $NAMESPACE -- rm -f /tmp/posters_export.json /tmp/posters_import.json /tmp/adposters_export.json /tmp/adposters_import.json
rm -rf $TEMP_DIR

echo ""
echo "========================================="
echo "✓ Region sync completed successfully!"
echo "========================================="
echo "Synced from: $SOURCE_REGION/$SOURCE_CITY"
echo "Synced to: $TARGET_REGION/$TARGET_CITY"
echo "========================================="
