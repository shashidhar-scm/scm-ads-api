// MongoDB Region Sync Script
// Run this inside MongoDB shell on the PRIMARY node
// Usage: mongo --eval "var sourceRegion='da'; var targetRegion='cg';" sync-region-mongo.js

var sourceRegion = typeof sourceRegion !== 'undefined' ? sourceRegion : 'da';
var sourceCity = typeof sourceCity !== 'undefined' ? sourceCity : 'da';
var targetRegion = typeof targetRegion !== 'undefined' ? targetRegion : 'cg';
var targetCity = typeof targetCity !== 'undefined' ? targetCity : 'cg';

print('========================================');
print('MongoDB Region Sync');
print('========================================');
print('Source: ' + sourceRegion + ' / ' + sourceCity);
print('Target: ' + targetRegion + ' / ' + targetCity);
print('========================================');

// Function to update region/city fields in a document
function updateLocationFields(doc, targetReg, targetCty) {
    if (doc.region) doc.region = targetReg;
    if (doc.city) doc.city = targetCty;
    if (doc.region_name) doc.region_name = targetReg;
    if (doc.city_name) doc.city_name = targetCty;
    
    // Update nested location fields
    if (doc.location) {
        if (doc.location.region) doc.location.region = targetReg;
        if (doc.location.city) doc.location.city = targetCty;
    }
    
    return doc;
}

// Sync Posters
print('\n--- Syncing Posters ---');
var sourceDB = db.getSiblingDB(sourceRegion);
var targetDB = db.getSiblingDB(targetRegion);

var posterQuery = {
    $or: [
        { status: 'ACTIVE' },
        { status: 'SCHEDULED' }
    ]
};

var postersCursor = sourceDB.posters.find(posterQuery);
var postersCount = 0;
var postersInserted = 0;
var postersUpdated = 0;

print('Syncing posters...');

postersCursor.forEach(function(doc) {
    postersCount++;
    
    // Update location fields
    var newDoc = updateLocationFields(doc, targetRegion, targetCity);
    
    // Remove _id to let MongoDB generate new one or use upsert
    var docId = newDoc._id;
    delete newDoc._id;
    
    // Upsert into target database
    var result = targetDB.posters.updateOne(
        { _id: docId },
        { $set: newDoc },
        { upsert: true }
    );
    
    if (result.upsertedCount > 0) {
        postersInserted++;
    } else if (result.modifiedCount > 0) {
        postersUpdated++;
    }
    
    if (postersCount % 100 === 0) {
        print('Processed ' + postersCount + ' posters...');
    }
});

print('✓ Posters sync complete');
print('  Total: ' + postersCount);
print('  Inserted: ' + postersInserted);
print('  Updated: ' + postersUpdated);

// Sync Ad Posters
print('\n--- Syncing Ad Posters ---');

var adPosterQuery = {
    $or: [
        { status: 'ACTIVE' },
        { status: 'SCHEDULED' }
    ]
};

var adPostersCursor = sourceDB.ad_posters.find(adPosterQuery);
var adPostersCount = 0;
var adPostersInserted = 0;
var adPostersUpdated = 0;

print('Syncing ad_posters...');

adPostersCursor.forEach(function(doc) {
    adPostersCount++;
    
    // Update location fields
    var newDoc = updateLocationFields(doc, targetRegion, targetCity);
    
    // Remove _id to let MongoDB generate new one or use upsert
    var docId = newDoc._id;
    delete newDoc._id;
    
    // Upsert into target database
    var result = targetDB.ad_posters.updateOne(
        { _id: docId },
        { $set: newDoc },
        { upsert: true }
    );
    
    if (result.upsertedCount > 0) {
        adPostersInserted++;
    } else if (result.modifiedCount > 0) {
        adPostersUpdated++;
    }
    
    if (adPostersCount % 100 === 0) {
        print('Processed ' + adPostersCount + ' ad_posters...');
    }
});

print('✓ Ad Posters sync complete');
print('  Total: ' + adPostersCount);
print('  Inserted: ' + adPostersInserted);
print('  Updated: ' + adPostersUpdated);

print('\n========================================');
print('✓ Region sync completed successfully!');
print('========================================');
print('Summary:');
print('  Posters: ' + postersCount + ' (' + postersInserted + ' new, ' + postersUpdated + ' updated)');
print('  Ad Posters: ' + adPostersCount + ' (' + adPostersInserted + ' new, ' + adPostersUpdated + ' updated)');
print('========================================');
