# Revision-Based Sync API Guide

## Overview

Version 0.0.82 introduces **revision-based synchronization** across all legacy endpoints, inspired by CouchDB/Sync Gateway patterns. This dramatically reduces bandwidth usage for kiosks by returning lightweight "no changes" responses when content hasn't been modified.

## Key Concepts

### Revision ID Format
- **Format:** `1-{8-char-hash}`
- **Example:** `1-e688e4e7`
- **Generation:** SHA256 hash of document content (first 8 characters)
- **Deterministic:** Same content always produces same revision

### Benefits
- ✅ **99.9%+ bandwidth savings** when content unchanged
- ✅ **No clock drift issues** (hash-based, not timestamp-based)
- ✅ **Atomic changes** (revision changes only when content changes)
- ✅ **Standard pattern** (compatible with CouchDB/PouchDB clients)

---

## New RESTful Endpoint

### `GET /theme/{device}`

**Description:** Get loop configuration for a specific device with CouchDB-style response.

**URL Pattern:** `https://scm-ads-api.citypost.us/theme/{device}`

**Example:** `https://scm-ads-api.citypost.us/theme/U696843`

#### Request Headers
- `If-None-Match: {revision}` (optional) - For conditional fetch

#### Response (200 OK)
```json
{
  "_id": "U696843",
  "_rev": "1-e688e4e7",
  "cards": [
    "e56a23b8-f8d2-4538-b583-aa8a74562398",
    "14a4d4da-5731-40f1-bae1-c4b4bc1108e3",
    "ad_poster_default_au_0",
    ...
  ],
  "city": "au",
  "device_code": "int_code",
  "device_type": "Interactive Kiosk",
  "loopPosterId": "U696843",
  "region": "au",
  "created_at": {"$date": "2019-08-21T11:15:28.82Z"},
  "updated_at": {"$date": "2026-04-26T11:44:46.49Z"}
}
```

#### Response (304 Not Modified)
When `If-None-Match` header matches current revision, returns empty body with 304 status.

#### Response Headers
- `ETag: {revision}` - Current document revision
- `Content-Type: application/json`

#### Example Usage
```bash
# First request - get full data
curl -i https://scm-ads-api.citypost.us/theme/U696843

# Response includes: ETag: 1-e688e4e7

# Subsequent request - conditional fetch
curl -i -H "If-None-Match: 1-e688e4e7" \
  https://scm-ads-api.citypost.us/theme/U696843

# Returns: HTTP/2 304 (no body, saves bandwidth)
```

---

## Updated Legacy Endpoints

All legacy endpoints now support revision-based sync via `rev` query parameter.

### 1. `GET /scm-api/theme`

**Description:** Get theme configuration with revision support.

**Parameters:**
- `theme_id` (required) - Theme identifier (e.g., `jc_jct_kiosk_6.0`)
- `rev` (optional) - Client revision for conditional fetch

**Response (Full Data):**
```json
{
  "status": "ok",
  "rev": "1-8ea2e690",
  "theme": [
    {
      "theme_id": "jc_jct_kiosk_6.0",
      "primaryColor": "#00AEEF",
      ...
    }
  ]
}
```

**Response (No Changes):**
```json
{
  "status": "no_changes",
  "rev": "1-8ea2e690"
}
```

**Example:**
```bash
# First request
curl "https://scm-ads-api.citypost.us/scm-api/theme?theme_id=jc_jct_kiosk_6.0"

# Subsequent request with revision
curl "https://scm-ads-api.citypost.us/scm-api/theme?theme_id=jc_jct_kiosk_6.0&rev=1-8ea2e690"
```

---

### 2. `GET /scm-api/getContent`

**Description:** Get all posters and ad_posters for a city/region with revision support.

**Parameters:**
- `city` (required) - City code (e.g., `jc`)
- `region` (required) - Region code (e.g., `jct`)
- `rev` (optional) - Client revision for conditional fetch

**Response (Full Data):**
```json
{
  "status": "ok",
  "rev": "1-01dcc336",
  "posters": [...],      // Array of 1747 posters
  "ad_posters": [...]    // Array of 31 ad_posters
}
```

**Response (No Changes):**
```json
{
  "status": "no_changes",
  "rev": "1-01dcc336"
}
```

**Data Savings:**
- Full response: ~11 MB
- No-changes response: 43 bytes
- **Savings: 99.9996%**

**Example:**
```bash
# First request
curl "https://scm-ads-api.citypost.us/scm-api/getContent?city=jc&region=jct"

# Subsequent request with revision
curl "https://scm-ads-api.citypost.us/scm-api/getContent?city=jc&region=jct&rev=1-01dcc336"
```

---

### 3. `GET /scm-api/getLoopPostersWeb`

**Description:** Get resolved loop poster objects for a device with revision support.

**Parameters:**
- `city` (required) - City code (e.g., `jc`)
- `device` (required) - Device identifier
- `rev` (optional) - Client revision for conditional fetch

**Response (Full Data):**
```json
{
  "status": "ok",
  "rev": "1-15195f0c",
  "loop_poster": [...]   // Array of resolved poster objects
}
```

**Response (No Changes):**
```json
{
  "status": "no_changes",
  "rev": "1-15195f0c"
}
```

**Example:**
```bash
# First request
curl "https://scm-ads-api.citypost.us/scm-api/getLoopPostersWeb?city=jc&device=umkc-update-test-2"

# Subsequent request with revision
curl "https://scm-ads-api.citypost.us/scm-api/getLoopPostersWeb?city=jc&device=umkc-update-test-2&rev=1-15195f0c"
```

---

## Client Implementation Guide

### Flutter/Dart Example

```dart
class ApiClient {
  String? _themeRev;
  String? _contentRev;
  String? _loopRev;

  Future<Map<String, dynamic>> fetchTheme(String themeId) async {
    var url = 'https://scm-ads-api.citypost.us/scm-api/theme?theme_id=$themeId';
    
    // Add revision if we have one
    if (_themeRev != null) {
      url += '&rev=$_themeRev';
    }
    
    final response = await http.get(Uri.parse(url));
    final data = jsonDecode(response.body);
    
    // Check if content changed
    if (data['status'] == 'no_changes') {
      print('Theme unchanged, using cached data');
      return null; // Use cached data
    }
    
    // Update stored revision
    _themeRev = data['rev'];
    return data;
  }
}
```

### JavaScript/HTTP Example

```javascript
class ApiClient {
  constructor() {
    this.loopRev = null;
  }

  async fetchLoop(device) {
    const headers = {};
    
    // Use If-None-Match header for RESTful endpoint
    if (this.loopRev) {
      headers['If-None-Match'] = this.loopRev;
    }
    
    const response = await fetch(
      `https://scm-ads-api.citypost.us/theme/${device}`,
      { headers }
    );
    
    // Check for 304 Not Modified
    if (response.status === 304) {
      console.log('Loop unchanged, using cached data');
      return null;
    }
    
    // Update stored revision from ETag
    this.loopRev = response.headers.get('ETag');
    return await response.json();
  }
}
```

---

## Migration Guide

### For Existing Clients

1. **Add revision storage** to your client state
2. **Send revision parameter** on subsequent requests
3. **Handle no_changes response** by using cached data
4. **Update stored revision** from server response

### Backward Compatibility

All endpoints remain **100% backward compatible**:
- Omitting `rev` parameter returns full data (as before)
- Response includes both `status` and data fields
- Old clients continue to work without changes

---

## Performance Metrics

### Typical Kiosk Usage Pattern

| Endpoint | Call Frequency | Full Size | No-Change Size | Daily Savings |
|----------|---------------|-----------|----------------|---------------|
| **getContent** | 1x/day | 11 MB | 43 bytes | ~11 MB |
| **theme** | 1x/day | 60 KB | 43 bytes | ~60 KB |
| **getLoopPostersWeb** | Every 3 min | 50 KB | 43 bytes | ~23 MB |
| **Total** | - | - | - | **~34 MB/day** |

### Monthly Savings (1000 kiosks)
- **Before:** ~1 TB/month
- **After:** ~10 GB/month
- **Savings:** ~99% reduction

---

## Swagger Documentation

Full API documentation available at:
- **Swagger UI:** `https://scm-ads-api.citypost.us/swagger/index.html`
- **OpenAPI Spec:** `https://scm-ads-api.citypost.us/swagger/doc.json`

---

## Version History

### v0.0.82 (April 27, 2026)
- ✅ Added `/theme/{device}` RESTful endpoint with ETag support
- ✅ Added revision support to `/scm-api/theme`
- ✅ Added revision support to `/scm-api/getContent`
- ✅ Added revision support to `/scm-api/getLoopPostersWeb`
- ✅ Implemented CouchDB-style revision ID generation
- ✅ Added comprehensive Swagger documentation

---

## Support

For questions or issues:
- Check Swagger docs: `/swagger/index.html`
- Review test examples in this guide
- Contact: API team
