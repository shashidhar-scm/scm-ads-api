#!/bin/bash

# MongoDB Region Sync with Authentication
# Usage: MONGO_PASSWORD=yourpass ./sync-with-auth.sh da cg
# Usage with poster types: MONGO_PASSWORD=yourpass ./sync-with-auth.sh da cg "restaurants,social_services"

SOURCE_REGION="${1:-da}"
TARGET_REGION="${2:-cg}"
POSTER_TYPES="${3:-}"
MONGO_POD="${MONGO_POD:-prod-mongodb-0}"
NAMESPACE="${NAMESPACE:-db}"
MONGO_USER="${MONGO_USER:-admin}"
MONGO_PASSWORD="${MONGO_PASSWORD}"

if [ -z "$MONGO_PASSWORD" ]; then
    echo "Error: MONGO_PASSWORD environment variable is required"
    echo "Usage: MONGO_PASSWORD=yourpass ./sync-with-auth.sh da cg"
    echo "Usage with types: MONGO_PASSWORD=yourpass ./sync-with-auth.sh da cg 'restaurants,social_services'"
    exit 1
fi

echo "========================================="
echo "MongoDB Region Sync (Authenticated)"
echo "========================================="
echo "Source: $SOURCE_REGION"
echo "Target: $TARGET_REGION"
if [ -n "$POSTER_TYPES" ]; then
    echo "Poster Types: $POSTER_TYPES"
fi
echo "MongoDB Pod: $MONGO_POD"
echo "========================================="
echo ""

# Create the sync script
cat > /tmp/mongo-sync-temp.js << 'SYNCEOF'
var sourceRegion = '__SOURCE_REGION__';
var targetRegion = '__TARGET_REGION__';
var posterTypes = __POSTER_TYPES__;

function updateLocationFields(doc, targetReg) {
    if (doc.region) doc.region = targetReg;
    if (doc.city) doc.city = targetReg;
    if (doc.region_name) doc.region_name = targetReg;
    if (doc.city_name) doc.city_name = targetReg;
    if (doc.location) {
        if (doc.location.region) doc.location.region = targetReg;
        if (doc.location.city) doc.location.city = targetReg;
    }
    return doc;
}

var sourceDB = db.getSiblingDB(sourceRegion);
var targetDB = db.getSiblingDB(targetRegion);

var posterQuery = {$or: [{status: 'ACTIVE'}, {status: 'SCHEDULED'}]};
if (posterTypes.length > 0) {
    posterQuery.posterType = {$in: posterTypes};
    print('Filtering by poster types: ' + posterTypes.join(', '));
}

print('Syncing posters from ' + sourceRegion + ' to ' + targetRegion + '...');
var pCount = 0;
var pInserted = 0;
var pUpdated = 0;

sourceDB.posters.find(posterQuery).forEach(function(doc) {
    var newDoc = updateLocationFields(doc, targetRegion);
    var docId = newDoc._id;
    delete newDoc._id;
    var result = targetDB.posters.updateOne({_id: docId}, {$set: newDoc}, {upsert: true});
    pCount++;
    if (result.upsertedCount > 0) pInserted++;
    if (result.modifiedCount > 0) pUpdated++;
    if (pCount % 100 === 0) print('Processed ' + pCount + ' posters...');
});
print('✓ Posters: ' + pCount + ' total (' + pInserted + ' new, ' + pUpdated + ' updated)');

var adPosterQuery = {$or: [{status: 'ACTIVE'}, {status: 'SCHEDULED'}]};
if (posterTypes.length > 0) {
    adPosterQuery.posterType = {$in: posterTypes};
}

print('Syncing ad_posters from ' + sourceRegion + ' to ' + targetRegion + '...');
var aCount = 0;
var aInserted = 0;
var aUpdated = 0;

sourceDB.ad_posters.find(adPosterQuery).forEach(function(doc) {
    var newDoc = updateLocationFields(doc, targetRegion);
    var docId = newDoc._id;
    delete newDoc._id;
    var result = targetDB.ad_posters.updateOne({_id: docId}, {$set: newDoc}, {upsert: true});
    aCount++;
    if (result.upsertedCount > 0) aInserted++;
    if (result.modifiedCount > 0) aUpdated++;
    if (aCount % 100 === 0) print('Processed ' + aCount + ' ad_posters...');
});
print('✓ Ad Posters: ' + aCount + ' total (' + aInserted + ' new, ' + aUpdated + ' updated)');

print('');
print('========================================');
print('✓ Sync Complete!');
print('========================================');
print('Posters: ' + pCount + ' (' + pInserted + ' new, ' + pUpdated + ' updated)');
print('Ad Posters: ' + aCount + ' (' + aInserted + ' new, ' + aUpdated + ' updated)');
print('========================================');
SYNCEOF

# Convert poster types to JavaScript array format
if [ -n "$POSTER_TYPES" ]; then
    # Convert comma-separated string to JavaScript array: "a,b" -> ["a","b"]
    POSTER_TYPES_ARRAY="['$(echo $POSTER_TYPES | sed "s/,/','/g")']"
else
    POSTER_TYPES_ARRAY="[]"
fi

# Replace placeholders
sed -i.bak "s/__SOURCE_REGION__/$SOURCE_REGION/g" /tmp/mongo-sync-temp.js
sed -i.bak "s/__TARGET_REGION__/$TARGET_REGION/g" /tmp/mongo-sync-temp.js
sed -i.bak "s/__POSTER_TYPES__/$POSTER_TYPES_ARRAY/g" /tmp/mongo-sync-temp.js

# Copy script to pod
echo "Copying sync script to MongoDB pod..."
kubectl cp /tmp/mongo-sync-temp.js $NAMESPACE/$MONGO_POD:/tmp/sync-exec.js

# Run the sync with authentication (non-interactive to prevent timeout)
echo "Running sync..."
kubectl exec $MONGO_POD -n $NAMESPACE -- bash -c "mongo admin -u $MONGO_USER -p '$MONGO_PASSWORD' --authenticationDatabase admin /tmp/sync-exec.js"

# Cleanup
echo ""
echo "Cleaning up..."
kubectl exec $MONGO_POD -n $NAMESPACE -- rm -f /tmp/sync-exec.js
rm -f /tmp/mongo-sync-temp.js /tmp/mongo-sync-temp.js.bak

echo ""
echo "✓ Done!"
