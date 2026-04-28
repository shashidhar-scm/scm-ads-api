# Manual MongoDB Region Sync

Since MongoDB requires authentication, here's how to run the sync manually:

## Step 1: Connect to MongoDB

```bash
kubectl exec -it prod-mongodb-2 -n db -- mongo
```

## Step 2: Authenticate (if required)

If you need to authenticate, use:
```javascript
use admin
db.auth("username", "password")
```

## Step 3: Set Variables

```javascript
var sourceRegion = 'da';
var sourceCity = 'da';
var targetRegion = 'cg';
var targetCity = 'cg';
```

## Step 4: Copy and paste the sync script

Copy the entire content from `sync-region-mongo.js` (starting from line 8, after the variable declarations) and paste it into the MongoDB shell.

Or load it directly:

```bash
# First, copy the script to the pod
kubectl cp scripts/sync-region-mongo.js db/prod-mongodb-2:/tmp/sync.js

# Then in MongoDB shell:
load('/tmp/sync.js')
```

## Alternative: One-liner approach

```bash
kubectl exec -it prod-mongodb-2 -n db -- bash
```

Then inside the pod:
```bash
cat > /tmp/quick-sync.js << 'SYNCEOF'
// Set your regions here
var sourceRegion = 'da';
var targetRegion = 'cg';

// Function to update location fields
function updateLocationFields(doc, targetReg, targetCty) {
    if (doc.region) doc.region = targetReg;
    if (doc.city) doc.city = targetCty;
    if (doc.region_name) doc.region_name = targetReg;
    if (doc.city_name) doc.city_name = targetCty;
    if (doc.location) {
        if (doc.location.region) doc.location.region = targetReg;
        if (doc.location.city) doc.location.city = targetCty;
    }
    return doc;
}

// Connect to databases
var sourceDB = db.getSiblingDB(sourceRegion);
var targetDB = db.getSiblingDB(targetRegion);

// Sync posters
print('Syncing posters from ' + sourceRegion + ' to ' + targetRegion + '...');
var count = 0;
sourceDB.posters.find({$or: [{status: 'ACTIVE'}, {status: 'SCHEDULED'}]}).forEach(function(doc) {
    var newDoc = updateLocationFields(doc, targetRegion, targetRegion);
    var docId = newDoc._id;
    delete newDoc._id;
    targetDB.posters.updateOne({_id: docId}, {$set: newDoc}, {upsert: true});
    count++;
    if (count % 100 === 0) print('Processed ' + count + ' posters...');
});
print('✓ Synced ' + count + ' posters');

// Sync ad_posters
print('Syncing ad_posters from ' + sourceRegion + ' to ' + targetRegion + '...');
count = 0;
sourceDB.ad_posters.find({$or: [{status: 'ACTIVE'}, {status: 'SCHEDULED'}]}).forEach(function(doc) {
    var newDoc = updateLocationFields(doc, targetRegion, targetRegion);
    var docId = newDoc._id;
    delete newDoc._id;
    targetDB.ad_posters.updateOne({_id: docId}, {$set: newDoc}, {upsert: true});
    count++;
    if (count % 100 === 0) print('Processed ' + count + ' ad_posters...');
});
print('✓ Synced ' + count + ' ad_posters');
print('Done!');
SYNCEOF

# Run it
mongo < /tmp/quick-sync.js
```

## Quick Command (if no auth required)

```bash
kubectl exec -it prod-mongodb-2 -n db -- bash -c 'mongo --eval "
var sourceRegion=\"da\";
var targetRegion=\"cg\";
var sourceDB = db.getSiblingDB(sourceRegion);
var targetDB = db.getSiblingDB(targetRegion);
var count = 0;
sourceDB.posters.find({\$or: [{status: \"ACTIVE\"}, {status: \"SCHEDULED\"}]}).forEach(function(doc) {
    if (doc.region) doc.region = targetRegion;
    if (doc.city) doc.city = targetRegion;
    var docId = doc._id;
    delete doc._id;
    targetDB.posters.updateOne({_id: docId}, {\$set: doc}, {upsert: true});
    count++;
});
print(\"Synced \" + count + \" posters\");
"'
```
