# MongoDB Region Sync Guide

Complete guide for syncing posters and ad_posters data between MongoDB regions.

## Overview

This sync process copies **ACTIVE** and **SCHEDULED** posters and ad_posters from a source region to a target region, updating all location-specific fields (region, city, region_name, city_name, etc.).

## Quick Start

### Prerequisites

- Access to Kubernetes cluster with MongoDB pods
- MongoDB admin password
- kubectl configured and connected to the cluster

### Run the Sync

```bash
cd /Users/gkg/workspace/scm/scm-ads-api

# Sync from da to cg
MONGO_PASSWORD='your_password' ./scripts/sync-with-auth.sh da cg

# Sync from au to us
MONGO_PASSWORD='your_password' ./scripts/sync-with-auth.sh au us
```

## What Gets Synced

### Data Selection
- **Collections**: `posters` and `ad_posters`
- **Status Filter**: Only `ACTIVE` and `SCHEDULED` items
- **Source**: Specified source region database
- **Target**: Specified target region database

### Fields Updated

The sync automatically updates these location fields:
- `region` → target region
- `city` → target city
- `region_name` → target region
- `city_name` → target city
- `location.region` → target region (if exists)
- `location.city` → target city (if exists)

### Sync Behavior

- **Upsert Mode**: Documents are inserted if new, updated if they already exist
- **ID Preservation**: Original `_id` values are preserved for matching
- **Overwrite**: Existing documents in target are overwritten with updated data

## Usage Examples

### Example 1: Sync Dallas (da) to Chicago (cg)

```bash
MONGO_PASSWORD='asterisk' ./scripts/sync-with-auth.sh da cg
```

**Output:**
```
=========================================
MongoDB Region Sync (Authenticated)
=========================================
Source: da
Target: cg
MongoDB Pod: prod-mongodb-2
=========================================

Syncing posters from da to cg...
Processed 100 posters...
Processed 200 posters...
...
✓ Posters: 2856 total (2856 new, 0 updated)

Syncing ad_posters from da to cg...
✓ Ad Posters: 35 total (35 new, 0 updated)

========================================
✓ Sync Complete!
========================================
```

### Example 2: Sync Australia (au) to US (us)

```bash
MONGO_PASSWORD='asterisk' ./scripts/sync-with-auth.sh au us
```

### Example 3: Use Different MongoDB Pod

```bash
MONGO_POD=prod-mongodb-1 MONGO_PASSWORD='asterisk' ./scripts/sync-with-auth.sh da cg
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MONGO_PASSWORD` | (required) | MongoDB admin password |
| `MONGO_POD` | `prod-mongodb-2` | MongoDB pod name (PRIMARY node) |
| `NAMESPACE` | `db` | Kubernetes namespace |
| `MONGO_USER` | `admin` | MongoDB username |

### Script Parameters

```bash
./scripts/sync-with-auth.sh [SOURCE_REGION] [TARGET_REGION]
```

- **SOURCE_REGION**: Source region database name (default: `da`)
- **TARGET_REGION**: Target region database name (default: `cg`)

## Available Scripts

### 1. sync-with-auth.sh (Recommended)

Automated sync with authentication. Runs non-interactively.

```bash
MONGO_PASSWORD='password' ./scripts/sync-with-auth.sh da cg
```

**Pros:**
- ✅ Fully automated
- ✅ No manual intervention needed
- ✅ Progress reporting
- ✅ Error handling

### 2. Manual MongoDB Shell Execution

For manual control or debugging.

```bash
# Connect to MongoDB
kubectl exec -it prod-mongodb-2 -n db -- mongo admin -u admin --authenticationDatabase admin

# Enter password when prompted
# Then paste the sync script (see MANUAL_SYNC.md)
```

**Pros:**
- ✅ Full control
- ✅ Can inspect data during sync
- ✅ Step-by-step execution

**Cons:**
- ❌ Manual intervention required
- ❌ Connection can timeout on large datasets

## Verification

### Check Synced Data

```bash
# Connect to MongoDB
kubectl exec -it prod-mongodb-2 -n db -- mongo admin -u admin --authenticationDatabase admin

# In MongoDB shell:
use cg
db.posters.count({status: {$in: ['ACTIVE', 'SCHEDULED']}})
db.ad_posters.count({status: {$in: ['ACTIVE', 'SCHEDULED']}})

# Verify region field is updated
db.posters.findOne({}, {region: 1, city: 1, region_name: 1})
```

### Check PostgreSQL Replica

The MongoDB sync will eventually replicate to PostgreSQL via the automated sync process.

```bash
# Check PostgreSQL replica
kubectl exec -it psql-cluster-1 -- psql -U appuser -d scm -c "SELECT COUNT(*) FROM citypost.posters WHERE region = 'cg';"
```

## Troubleshooting

### Error: "command find requires authentication"

**Cause**: Not authenticated or wrong credentials

**Solution**: Ensure you're using the correct MongoDB admin password

```bash
# Verify password
MONGO_PASSWORD='correct_password' ./scripts/sync-with-auth.sh da cg
```

### Error: "not master and slaveOk=false"

**Cause**: Connected to a SECONDARY node instead of PRIMARY

**Solution**: Use `prod-mongodb-2` (PRIMARY node)

```bash
MONGO_POD=prod-mongodb-2 MONGO_PASSWORD='password' ./scripts/sync-with-auth.sh da cg
```

### Error: "command terminated with exit code 137"

**Cause**: Connection timeout or pod killed during sync

**Solution**: Use the automated script instead of manual execution

```bash
# Automated script handles this better
MONGO_PASSWORD='password' ./scripts/sync-with-auth.sh da cg
```

### Sync Shows "0 new, 0 updated"

**Cause**: Documents already exist with same `_id` and content

**Explanation**: This is normal if:
- You're re-running the sync
- Documents were already synced previously
- The upsert didn't modify any fields

**Verification**: Check if documents exist in target:

```javascript
use cg
db.posters.count()
```

## Performance

### Sync Speed

- **~100 documents/second** (typical)
- **2,856 posters** = ~30 seconds
- **35 ad_posters** = <1 second

### Large Dataset Recommendations

For regions with >10,000 documents:

1. Run during off-peak hours
2. Monitor MongoDB resource usage
3. Consider batching if needed

## Safety & Best Practices

### ✅ Safe Operations

- Running sync multiple times (idempotent)
- Syncing to existing target region (upsert mode)
- Running on PRIMARY node

### ⚠️ Cautions

- **Overwrites target data**: Existing documents with same `_id` are replaced
- **No backup**: Original target data is overwritten without backup
- **Region field changes**: All location fields are updated to target region

### 🔒 Recommendations

1. **Test first**: Run on test regions before production
2. **Verify source**: Ensure source region has correct data
3. **Check counts**: Verify document counts before and after
4. **Monitor**: Watch MongoDB logs during sync

## Region Codes

Common region/city codes:

| Code | Region/City |
|------|-------------|
| `da` | Dallas |
| `cg` | Chicago |
| `au` | Australia |
| `us` | United States |
| `jc` | Jersey City |
| `kcmo` | Kansas City |

## Complete Workflow

### Step-by-Step Sync Process

1. **Verify source data**
   ```bash
   kubectl exec -it prod-mongodb-2 -n db -- mongo admin -u admin --authenticationDatabase admin
   use da
   db.posters.count({status: {$in: ['ACTIVE', 'SCHEDULED']}})
   ```

2. **Run sync**
   ```bash
   MONGO_PASSWORD='password' ./scripts/sync-with-auth.sh da cg
   ```

3. **Verify target data**
   ```bash
   kubectl exec -it prod-mongodb-2 -n db -- mongo admin -u admin --authenticationDatabase admin
   use cg
   db.posters.count()
   db.posters.findOne({}, {region: 1, city: 1})
   ```

4. **Wait for PostgreSQL replication** (automatic, ~5-10 minutes)

5. **Verify PostgreSQL**
   ```bash
   kubectl exec -it psql-cluster-1 -- psql -U appuser -d scm -c "SELECT COUNT(*) FROM citypost.posters WHERE region = 'cg';"
   ```

## Support

For issues or questions:
1. Check logs: `kubectl logs -n db prod-mongodb-2 -c mongod --tail=100`
2. Verify MongoDB status: `kubectl get pods -n db | grep mongo`
3. Review this guide and MANUAL_SYNC.md

## Last Successful Sync

**Date**: April 28, 2026  
**Source**: da (Dallas)  
**Target**: cg (Chicago)  
**Results**:
- Posters: 2,856 synced
- Ad Posters: 35 synced
- Status: ✅ Success
