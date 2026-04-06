DROP FUNCTION IF EXISTS refresh_device_poi_feature_values(text, text);
DROP FUNCTION IF EXISTS rebuild_osm_poi_points();

DROP TABLE IF EXISTS device_poi_feature_values;
DROP TABLE IF EXISTS poi_points;
DROP TABLE IF EXISTS poi_categories;
