#!/usr/bin/env node

const { XMLParser } = require('fast-xml-parser');
const cheerio = require('cheerio');
const { MongoClient } = require('mongodb');
const { v4: uuidv4 } = require('uuid');
const fetch = require('node-fetch');

// Configuration
const CONFIG = {
  RSS_FEED_URL: process.env.RSS_FEED_URL || 'https://chicago.eater.com/rss/index.xml',
  MONGO_URI: process.env.MONGO_URI || 'mongodb://admin:asterisk@prod-mongodb-svc.db.svc.cluster.local:27017/admin?replicaSet=prod-mongodb&authSource=admin',
  MONGO_DB: process.env.MONGO_DB || 'chicago',
  TARGET_CITY: process.env.TARGET_CITY || 'chicago',
  TARGET_REGION: process.env.TARGET_REGION || 'chi',
  POSTER_LIFESPAN_DAYS: parseInt(process.env.POSTER_LIFESPAN_DAYS || '7'),
  DRY_RUN: process.argv.includes('--dry-run'),
  MAX_ARTICLES: parseInt(process.env.MAX_ARTICLES || '10'),
};

// RSS Feed Parser
class RSSFeedParser {
  constructor() {
    this.parser = new XMLParser({
      ignoreAttributes: false,
      attributeNamePrefix: '@_',
    });
  }

  async fetchFeed(url) {
    console.log(`Fetching RSS feed from: ${url}`);
    const response = await fetch(url);
    if (!response.ok) {
      throw new Error(`Failed to fetch RSS feed: ${response.statusText}`);
    }
    return await response.text();
  }

  parseFeed(xmlContent) {
    const result = this.parser.parse(xmlContent);
    return result.feed?.entry || [];
  }

  extractMainImage(htmlContent) {
    const $ = cheerio.load(htmlContent);
    const firstImage = $('img').first();
    
    if (firstImage.length) {
      return {
        url: firstImage.attr('src'),
        alt: firstImage.attr('alt') || '',
        caption: firstImage.attr('data-caption') || $('figcaption').first().text().trim() || '',
      };
    }
    return null;
  }

  extractAllImages(htmlContent) {
    const $ = cheerio.load(htmlContent);
    const images = [];
    
    $('img').each((i, elem) => {
      const url = $(elem).attr('src');
      if (url && url.startsWith('http')) {
        images.push({
          url,
          alt: $(elem).attr('alt') || '',
          caption: $(elem).attr('data-caption') || '',
        });
      }
    });
    
    return images;
  }

  stripHtml(html) {
    const $ = cheerio.load(html);
    return $.text().trim();
  }
}

// Poster Generator
class PosterGenerator {
  constructor(config) {
    this.config = config;
  }

  generatePosterId() {
    return uuidv4();
  }

  createMultiLangContent(text) {
    return [
      { lang: 'en', data: text },
      { lang: 'es', data: text },
      { lang: 'fa', data: text },
      { lang: 'ar', data: text },
      { lang: 'ko', data: text },
      { lang: 'ur', data: text },
      { lang: 'hi', data: text },
      { lang: 'vi', data: text },
      { lang: 'zh-CN', data: text }
    ];
  }

  createPosterFromEntry(entry, existingIds = new Set()) {
    // Extract entry ID from RSS
    const entryId = entry.id || entry.link?.['@_href'] || '';
    
    // Check if already processed
    if (existingIds.has(entryId)) {
      console.log(`Skipping duplicate entry: ${entryId}`);
      return null;
    }

    // Parse dates
    const published = new Date(entry.published);
    const updated = new Date(entry.updated);
    const endDate = new Date(published.getTime() + (this.config.POSTER_LIFESPAN_DAYS * 24 * 60 * 60 * 1000));
    
    // Extract content
    const htmlContent = entry.content?.['#text'] || entry.content || '';
    const summary = entry.summary?.['#text'] || entry.summary || '';
    const title = this.stripCData(entry.title?.['#text'] || entry.title || 'Untitled');
    const description = this.stripHtml(summary);
    const link = entry.link?.['@_href'] || entry.link || '';
    
    // Extract images
    const mainImage = new RSSFeedParser().extractMainImage(htmlContent);
    const allImages = new RSSFeedParser().extractAllImages(htmlContent);
    
    // Extract categories
    const categories = Array.isArray(entry.category) 
      ? entry.category.map(c => c['@_term'] || c).filter(Boolean)
      : entry.category ? [entry.category['@_term'] || entry.category] : [];
    
    // Determine poster group based on categories
    const posterGroup = categories.some(c => c.toLowerCase().includes('dining') || c.toLowerCase().includes('restaurant')) 
      ? ['dining'] 
      : ['news'];
    
    // Build poster document matching the schema
    const poster = {
      posterId: this.generatePosterId(),
      title: title,
      status: 'ACTIVE',
      city: this.config.TARGET_CITY,
      region: this.config.TARGET_REGION,
      started_at: published.toISOString(),
      start_date: published,
      end_date: endDate,
      created_at: published,
      updated_at: updated,
      lifeSpan: this.config.POSTER_LIFESPAN_DAYS * 24 * 60 * 60 * 1000,
      showInLoop: true,
      beaconOnly: false,
      role: 'rss-feed',
      created_by: 'eater-chicago-rss',
      amount: 0.0,
      posterGroup: posterGroup,
      posterType: ['rss_restaurant'],
      ad_agency: [],
      addressDetails: {
        address: '',
        state: '',
        zipCode: '',
        phoneNumber: ''
      },
      event_start_date: null,
      event_end_date: null,
      kiosksId: null,
      link: link || null,
      misc: {
        isLanding: 'false',
        playTime: '5',
        templateType: 'rss-article',
        rssSource: 'eater-chicago',
        rssEntryId: entryId,
        categories: categories.join(', ')
      },
      paymentDetails: null,
      placeIds: null,
      section1: {
        fileName: '',
        content: this.createMultiLangContent(title),
        link: link || ''
      },
      section2: {
        content: this.createMultiLangContent(description),
        action: link || ''
      },
      section4: null,
      section5: {
        content: this.createMultiLangContent('Eater Chicago')
      }
    };

    // Add main image to section3 if available
    if (mainImage && mainImage.url) {
      poster.section3 = {
        fileName: mainImage.url.split('/').pop().split('?')[0],
        fileUrl: mainImage.url,
        mimetype: 'image',
        mobileUrl: mainImage.url
      };
    } else {
      poster.section3 = null;
    }

    return poster;
  }

  stripCData(text) {
    if (!text) return '';
    return text.replace(/<!\[CDATA\[(.*?)\]\]>/g, '$1').trim();
  }

  stripHtml(html) {
    const $ = cheerio.load(html);
    return $.text().trim().substring(0, 500); // Limit to 500 chars
  }

  capitalizeFirst(str) {
    return str.charAt(0).toUpperCase() + str.slice(1);
  }

  getMimeTypeFromUrl(url) {
    if (!url) return 'image/jpeg';
    const ext = url.split('.').pop().split('?')[0].toLowerCase();
    const mimeTypes = {
      'jpg': 'image/jpeg',
      'jpeg': 'image/jpeg',
      'png': 'image/png',
      'gif': 'image/gif',
      'webp': 'image/webp',
    };
    return mimeTypes[ext] || 'image/jpeg';
  }
}

// MongoDB Manager
class MongoDBManager {
  constructor(uri, dbName) {
    this.uri = uri;
    this.dbName = dbName;
    this.client = null;
    this.db = null;
  }

  async connect() {
    console.log(`Connecting to MongoDB: ${this.dbName}`);
    this.client = new MongoClient(this.uri);
    await this.client.connect();
    this.db = this.client.db(this.dbName);
    console.log('Connected to MongoDB');
  }

  async getExistingRSSEntryIds() {
    const collection = this.db.collection('posters');
    const existingPosters = await collection
      .find({ 'misc.rssSource': 'eater-chicago' }, { projection: { 'misc.rssEntryId': 1 } })
      .toArray();
    return new Set(existingPosters.map(p => p.misc?.rssEntryId).filter(Boolean));
  }

  async upsertPoster(poster) {
    const collection = this.db.collection('posters');
    const result = await collection.updateOne(
      { 'misc.rssEntryId': poster.misc.rssEntryId },
      { $set: poster },
      { upsert: true }
    );
    return result;
  }

  async close() {
    if (this.client) {
      await this.client.close();
      console.log('MongoDB connection closed');
    }
  }
}

// Main Application
class RSSToPostersApp {
  constructor(config) {
    this.config = config;
    this.parser = new RSSFeedParser();
    this.generator = new PosterGenerator(config);
    this.db = new MongoDBManager(config.MONGO_URI, config.MONGO_DB);
  }

  async run() {
    console.log('=== RSS to Posters Converter ===');
    console.log(`Target: ${this.config.TARGET_CITY}/${this.config.TARGET_REGION}`);
    console.log(`Dry Run: ${this.config.DRY_RUN}`);
    console.log(`Max Articles: ${this.config.MAX_ARTICLES}`);
    console.log('');

    try {
      // Fetch and parse RSS feed
      const xmlContent = await this.parser.fetchFeed(this.config.RSS_FEED_URL);
      const entries = this.parser.parseFeed(xmlContent);
      console.log(`Found ${entries.length} entries in RSS feed`);

      // Connect to MongoDB
      if (!this.config.DRY_RUN) {
        await this.db.connect();
      }

      // Get existing RSS entry IDs to avoid duplicates
      const existingIds = this.config.DRY_RUN 
        ? new Set() 
        : await this.db.getExistingRSSEntryIds();
      console.log(`Found ${existingIds.size} existing RSS posters in database`);
      console.log('');

      // Process entries
      let processed = 0;
      let created = 0;
      let skipped = 0;

      for (const entry of entries.slice(0, this.config.MAX_ARTICLES)) {
        processed++;
        
        const poster = this.generator.createPosterFromEntry(entry, existingIds);
        
        if (!poster) {
          skipped++;
          continue;
        }

        console.log(`[${processed}/${this.config.MAX_ARTICLES}] Processing: ${poster.title.substring(0, 60)}...`);
        
        if (this.config.DRY_RUN) {
          console.log('  [DRY RUN] Would create poster:');
          console.log(`    ID: ${poster._id}`);
          console.log(`    Title: ${poster.title}`);
          console.log(`    Link: ${poster.link}`);
          console.log(`    Image: ${poster.section1 || 'None'}`);
          console.log(`    Categories: ${poster.categories.join(', ')}`);
          console.log('');
        } else {
          const result = await this.db.upsertPoster(poster);
          if (result.upsertedCount > 0) {
            created++;
            console.log(`  ✓ Created new poster`);
          } else {
            console.log(`  ↻ Updated existing poster`);
          }
        }
      }

      console.log('');
      console.log('=== Summary ===');
      console.log(`Processed: ${processed}`);
      console.log(`Created: ${created}`);
      console.log(`Skipped: ${skipped}`);
      console.log(`Dry Run: ${this.config.DRY_RUN}`);

    } catch (error) {
      console.error('Error:', error.message);
      throw error;
    } finally {
      if (!this.config.DRY_RUN) {
        await this.db.close();
      }
    }
  }
}

// Run the application
if (require.main === module) {
  const app = new RSSToPostersApp(CONFIG);
  app.run().catch(error => {
    console.error('Fatal error:', error);
    process.exit(1);
  });
}

module.exports = { RSSToPostersApp, RSSFeedParser, PosterGenerator };
