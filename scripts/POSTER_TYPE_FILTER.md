# Poster Type Filtering for Region Sync

## Overview

The region sync now supports filtering by poster types. You can sync only specific types of posters (e.g., restaurants, social services) instead of syncing all posters.

## Quick Usage

### Sync Specific Poster Types

```bash
# Sync only restaurants from Dallas to Chicago
MONGO_PASSWORD='your_password' ./scripts/sync-with-auth.sh da cg "restaurants"

# Sync restaurants and social services
MONGO_PASSWORD='your_password' ./scripts/sync-with-auth.sh da cg "restaurants,social_services"

# Sync multiple types
MONGO_PASSWORD='your_password' ./scripts/sync-with-auth.sh da cg "restaurants,social_services,entertainment,retail"
```

### Sync All Poster Types (Default)

```bash
# Omit the third parameter to sync all types
MONGO_PASSWORD='your_password' ./scripts/sync-with-auth.sh da cg
```

## Common Poster Types

Based on the codebase, here are common poster types you can filter by:

- `restaurants` - Restaurant posters
- `social_services` - Social service announcements
- `rss_restaurant` - RSS feed restaurant posters
- `entertainment` - Entertainment events
- `retail` - Retail advertisements
- `healthcare` - Healthcare services

## How It Works

### In sync-with-auth.sh

The script accepts a third parameter for poster types:

```bash
./scripts/sync-with-auth.sh [SOURCE] [TARGET] [POSTER_TYPES]
```

Example:
```bash
MONGO_PASSWORD='pass' ./scripts/sync-with-auth.sh da cg "restaurants,social_services"
```

### In sync-region-mongo.js

For direct MongoDB shell execution:

```bash
mongo --eval "var sourceRegion='da'; var targetRegion='cg'; var posterTypes=['restaurants', 'social_services'];" sync-region-mongo.js
```

Or in MongoDB shell:

```javascript
var sourceRegion = 'da';
var targetRegion = 'cg';
var posterTypes = ['restaurants', 'social_services'];
// Then load the script
```

## Query Behavior

When poster types are specified, the sync adds a filter to the MongoDB query:

```javascript
// Without filter (default)
{$or: [{status: 'ACTIVE'}, {status: 'SCHEDULED'}]}

// With filter
{
  $or: [{status: 'ACTIVE'}, {status: 'SCHEDULED'}],
  posterType: {$in: ['restaurants', 'social_services']}
}
```

This applies to both `posters` and `ad_posters` collections.

## Examples

### Example 1: Sync Only Restaurants

```bash
MONGO_PASSWORD='asterisk' ./scripts/sync-with-auth.sh da cg "restaurants"
```

**Output:**
```
=========================================
MongoDB Region Sync (Authenticated)
=========================================
Source: da
Target: cg
Poster Types: restaurants
MongoDB Pod: prod-mongodb-2
=========================================

Filtering by poster types: restaurants
Syncing posters from da to cg...
✓ Posters: 234 total (234 new, 0 updated)

Syncing ad_posters from da to cg...
✓ Ad Posters: 5 total (5 new, 0 updated)
```

### Example 2: Sync Multiple Types

```bash
MONGO_PASSWORD='asterisk' ./scripts/sync-with-auth.sh da cg "restaurants,social_services,entertainment"
```

### Example 3: Verify Synced Data

```bash
# Connect to MongoDB
kubectl exec -it prod-mongodb-2 -n db -- mongo admin -u admin --authenticationDatabase admin

# Check synced posters by type
use cg
db.posters.count({posterType: 'restaurants', status: {$in: ['ACTIVE', 'SCHEDULED']}})
db.posters.find({posterType: 'restaurants'}).limit(5)
```

## Important Notes

1. **Comma-separated, no spaces**: Use `"restaurants,social_services"` not `"restaurants, social_services"`
2. **Case-sensitive**: Poster types are case-sensitive, use exact values
3. **Array field**: The `posterType` field is an array in MongoDB, so the `$in` operator matches if any element matches
4. **Both collections**: The filter applies to both `posters` and `ad_posters` collections
5. **Status still applies**: Only `ACTIVE` and `SCHEDULED` items are synced, even with type filter

## Troubleshooting

### No Results with Filter

If you get 0 results when filtering by type:

1. **Check the poster type values in source database:**
   ```javascript
   use da
   db.posters.distinct('posterType')
   ```

2. **Verify case sensitivity:**
   - Use exact values from the database
   - Common mistake: `"Restaurants"` vs `"restaurants"`

3. **Check if field is array:**
   ```javascript
   db.posters.findOne({}, {posterType: 1})
   // Should show: posterType: ['restaurants']
   ```

### Sync All Types Instead

If you're having trouble with filtering, omit the third parameter to sync all types:

```bash
MONGO_PASSWORD='pass' ./scripts/sync-with-auth.sh da cg
```

## See Also

- [SYNC_GUIDE.md](./SYNC_GUIDE.md) - Complete sync documentation
- [sync-with-auth.sh](./sync-with-auth.sh) - Automated sync script
- [sync-region-mongo.js](./sync-region-mongo.js) - MongoDB sync script
