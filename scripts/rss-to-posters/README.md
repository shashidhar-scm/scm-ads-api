# RSS to Posters Converter

Converts RSS feed articles (e.g., Eater Chicago) into MongoDB poster documents for display on digital signage.

## Features

- ✅ Parses Atom/RSS feeds
- ✅ Extracts article metadata (title, author, dates, categories)
- ✅ Extracts images from HTML content
- ✅ Creates MongoDB poster documents
- ✅ Deduplication (skips already processed articles)
- ✅ Dry-run mode for testing
- ✅ Configurable via environment variables

## Installation

```bash
cd scripts/rss-to-posters
npm install
```

## Usage

### Dry Run (Test Mode)

```bash
npm test
# or
node index.js --dry-run
```

This will fetch the RSS feed and show what posters would be created without writing to MongoDB.

### Production Run

```bash
npm start
# or
node index.js
```

This will create/update posters in MongoDB.

## Configuration

Set environment variables to customize behavior:

```bash
# RSS Feed URL
export RSS_FEED_URL="https://chicago.eater.com/rss/index.xml"

# MongoDB Connection
export MONGO_URI="mongodb://admin:asterisk@prod-mongodb-svc.db.svc.cluster.local:27017/admin?replicaSet=prod-mongodb&authSource=admin"
export MONGO_DB="chicago"

# Target City/Region
export TARGET_CITY="chicago"
export TARGET_REGION="chi"

# Poster Settings
export POSTER_LIFESPAN_DAYS="7"  # How long posters stay active
export MAX_ARTICLES="10"          # Max articles to process per run

# Run the script
npm start
```

## Kubernetes Deployment

### Option 1: Manual Run

```bash
# Copy script to a pod
kubectl cp scripts/rss-to-posters prod-mongodb-0:/tmp/ -n db

# Run inside pod
kubectl exec -it prod-mongodb-0 -n db -- bash
cd /tmp/rss-to-posters
npm install
node index.js
```

### Option 2: CronJob (Recommended)

Create a Kubernetes CronJob to run this script every hour:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: rss-to-posters-chicago
  namespace: backend
spec:
  schedule: "0 * * * *"  # Every hour
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: rss-converter
            image: node:18-alpine
            command:
            - /bin/sh
            - -c
            - |
              cd /app
              npm install
              node index.js
            env:
            - name: RSS_FEED_URL
              value: "https://chicago.eater.com/rss/index.xml"
            - name: MONGO_URI
              valueFrom:
                secretKeyRef:
                  name: mongo-pg-replicator-secrets
                  key: MONGO_URI
            - name: MONGO_DB
              value: "chicago"
            - name: TARGET_CITY
              value: "chicago"
            - name: TARGET_REGION
              value: "chi"
            - name: POSTER_LIFESPAN_DAYS
              value: "7"
            - name: MAX_ARTICLES
              value: "10"
            volumeMounts:
            - name: script
              mountPath: /app
          volumes:
          - name: script
            configMap:
              name: rss-to-posters-script
          restartPolicy: OnFailure
```

## Output Example

```
=== RSS to Posters Converter ===
Target: chicago/chi
Dry Run: true
Max Articles: 10

Fetching RSS feed from: https://chicago.eater.com/rss/index.xml
Found 15 entries in RSS feed
Found 0 existing RSS posters in database

[1/10] Processing: Tabletop Trompos Bring the Drama at This Hotel Restaur...
  [DRY RUN] Would create poster:
    ID: https://chicago.eater.com/?p=167910
    Title: Tabletop Trompos Bring the Drama at This Hotel Restaurant in the Loop
    Link: https://chicago.eater.com/restaurant-news/167910/mexican-radio-new-restaurant-opening-chicago-loop-dudley-nieto
    Image: https://platform.chicago.eater.com/wp-content/uploads/sites/17/2026/05/mexican-radio-01784.jpg
    Categories: Chicago Restaurant News, Chicago Restaurant Openings

[2/10] Processing: Chicago's Best Thai Food...
  [DRY RUN] Would create poster:
    ID: https://chicago.eater.com/maps/best-thai-restaurants-chicago
    Title: Chicago's Best Thai Food
    Link: https://chicago.eater.com/maps/best-thai-restaurants-chicago
    Image: https://platform.chicago.eater.com/wp-content/uploads/sites/17/2025/09/666873237_973062575092773_3401881400397483342_n.jpg
    Categories: Dining Out in Chicago, Eater Guides

=== Summary ===
Processed: 10
Created: 10
Skipped: 0
Dry Run: true
```

## Poster Document Structure

Each RSS article is converted to a poster with the following fields:

```javascript
{
  _id: "https://chicago.eater.com/?p=167910",  // RSS entry ID
  posterId: "uuid-v4",
  title: "Article Title",
  status: "ACTIVE",
  city: "chicago",
  region: "chi",
  region_name: "Chi",
  city_name: "Chicago",
  link: "https://chicago.eater.com/...",
  started_at: ISODate("2026-05-05T17:57:23Z"),
  created_at: ISODate("2026-05-05T17:57:23Z"),
  updated_at: ISODate("2026-05-05T18:41:18Z"),
  lifeSpan: 604800000,  // 7 days in milliseconds
  showInLoop: true,
  beaconOnly: false,
  role: "rss-feed",
  created_by: "eater-chicago-bot",
  description: "Article summary text...",
  author: "Brenda Storch",
  categories: ["Chicago Restaurant News", "Restaurant Openings"],
  source: "eater-chicago-rss",
  rss_entry_id: "https://chicago.eater.com/?p=167910",
  section1: "https://platform.chicago.eater.com/.../image.jpg",
  section1MimeType: "image/jpeg",
  broadcastUrl: "https://platform.chicago.eater.com/.../image.jpg",
  broadcastMimeType: "image/jpeg",
  imageCaption: "Image caption text",
  section2: "https://...",  // Optional second image
  section3: "https://...",  // Optional third image
}
```

## Deduplication

The script uses the RSS entry `id` field as the MongoDB `_id` to prevent duplicates. If an article already exists, it will be updated instead of creating a new poster.

## Scheduling Recommendations

- **Hourly**: Good for active feeds with frequent updates
- **Every 6 hours**: Balanced approach for most use cases
- **Daily**: For less active feeds or to reduce load

## Troubleshooting

### Connection Issues

```bash
# Test MongoDB connection
node -e "const {MongoClient} = require('mongodb'); new MongoClient('mongodb://...').connect().then(() => console.log('OK')).catch(console.error)"
```

### Feed Parsing Issues

```bash
# Test feed fetch
curl -I https://chicago.eater.com/rss/index.xml
```

### Dry Run First

Always test with `--dry-run` before running in production:

```bash
node index.js --dry-run
```

## Other RSS Feeds

This script can be adapted for other RSS feeds:

```bash
# New York Eater
export RSS_FEED_URL="https://ny.eater.com/rss/index.xml"
export TARGET_CITY="new-york"
export TARGET_REGION="ny"

# LA Eater
export RSS_FEED_URL="https://la.eater.com/rss/index.xml"
export TARGET_CITY="los-angeles"
export TARGET_REGION="la"

# Generic RSS feed
export RSS_FEED_URL="https://example.com/feed.xml"
export TARGET_CITY="your-city"
export TARGET_REGION="your-region"
```

## License

MIT
