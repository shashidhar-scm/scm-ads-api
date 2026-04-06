-- migrations/0019_osm_poi_features_postgis.up.sql
--
-- NOTE:
-- This migration is intentionally a no-op to avoid blocking application startup in environments
-- where PostGIS is not installed/available.
--
-- Once PostGIS is installed and `CREATE EXTENSION postgis;` succeeds on the target database,
-- replace this migration (or add a later one) to create POI cache tables and functions.

SELECT 1;
