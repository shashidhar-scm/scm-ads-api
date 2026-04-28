# MongoDB Region Sync Scripts

Scripts to sync posters and ad_posters data between MongoDB regions.

## Overview

These scripts copy ACTIVE and SCHEDULED posters and ad_posters from one region/city to another, updating all location-specific fields.

## Files

- **`sync-region-mongo.js`** - MongoDB JavaScript script that performs the sync
- **`run-mongo-sync.sh`** - Wrapper script to run the sync from kubectl
- **`sync-region-mongo.sh`** - Alternative bash script using mongoexport/mongoimport

## Quick Start

### Method 1: Using the wrapper script (Recommended)

```bash
# Sync from da to cg
./scripts/run-mongo-sync.sh da cg

# Sync from da/da to cg/cg (with explicit cities)
./scripts/run-mongo-sync.sh da cg da cg

# Use custom MongoDB pod
MONGO_POD=prod-mongodb-0 ./scripts/run-mongo-sync.sh da cg
```

### Method 2: Run directly in MongoDB shell

```bash
# Copy script to pod
kubectl cp scripts/sync-region-mongo.js default/prod-mongodb-2:/tmp/sync-region-mongo.js

# Execute in MongoDB
kubectl exec -it prod-mongodb-2 -- mongo --eval "var sourceRegion='da'; var targetRegion='cg';" /tmp/sync-region-mongo.js

# Cleanup
kubectl exec -it prod-mongodb-2 -- rm /tmp/sync-region-mongo.js
```

### Method 3: Manual execution in MongoDB shell

```bash
# Connect to MongoDB
kubectl exec -it prod-mongodb-2 -- mongo

# Then paste and run the script content, or:
# 1. Set variables
var sourceRegion = 'da';
var sourceCity = 'da';
var targetRegion = 'cg';
var targetCity = 'cg';

# 2. Load and run the script
load('/tmp/sync-region-mongo.js');
```

## What the script does

1. **Queries source region** for all ACTIVE and SCHEDULED posters
2. **Updates location fields**:
   - `region` → target region
   - `city` → target city
   - `region_name` → target region
   - `city_name` → target city
   - `location.region` → target region
   - `location.city` → target city
3. **Upserts to target region** (inserts new or updates existing)
4. **Repeats for ad_posters**
5. **Reports statistics**

## Configuration

### Environment Variables

- `MONGO_POD` - MongoDB pod name (default: `prod-mongodb-2`)
- `NAMESPACE` - Kubernetes namespace (default: `db`)

### Script Variables

When running the MongoDB JavaScript directly, you can set:

```javascript
var sourceRegion = 'da';    // Source region database
var sourceCity = 'da';      // Source city value
var targetRegion = 'cg';    // Target region database
var targetCity = 'cg';      // Target city value
```

## Examples

### Sync da → cg
```bash
./scripts/run-mongo-sync.sh da cg
```

### Sync au → us
```bash
./scripts/run-mongo-sync.sh au us
```

### Sync with different cities
```bash
./scripts/run-mongo-sync.sh da cg dallas chicago
```

## Output

The script provides detailed progress:

```
=========================================
MongoDB Region Sync
=========================================
Source: da / da
Target: cg / cg
=========================================

--- Syncing Posters ---
Found 1234 posters to sync
Processed 100 posters...
Processed 200 posters...
...
✓ Posters sync complete
  Total: 1234
  Inserted: 1000
  Updated: 234

--- Syncing Ad Posters ---
Found 567 ad_posters to sync
Processed 100 ad_posters...
...
✓ Ad Posters sync complete
  Total: 567
  Inserted: 500
  Updated: 67

=========================================
✓ Region sync completed successfully!
=========================================
Summary:
  Posters: 1234 (1000 new, 234 updated)
  Ad Posters: 567 (500 new, 67 updated)
=========================================
```

## Notes

- The script uses **upsert** mode, so it will:
  - Insert new documents if they don't exist in target
  - Update existing documents if they already exist
- Only **ACTIVE** and **SCHEDULED** status items are synced
- The script preserves the original `_id` for upsert matching
- Run on the **PRIMARY** MongoDB node for best performance

## Troubleshooting

### "Cannot find MongoDB pod"
Make sure the pod name is correct:
```bash
kubectl get pods | grep mongo
MONGO_POD=your-pod-name ./scripts/run-mongo-sync.sh da cg
```

### "Permission denied"
Ensure the script is executable:
```bash
chmod +x scripts/run-mongo-sync.sh
```

### "Database not found"
Check that the source region database exists:
```bash
kubectl exec -it prod-mongodb-2 -- mongo --eval "show dbs"
```
