CREATE TABLE IF NOT EXISTS poi_categories (
    category TEXT PRIMARY KEY,
    point_where TEXT NOT NULL,
    polygon_where TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS poi_points (
    category TEXT NOT NULL,
    osm_id BIGINT NOT NULL,
    geom geometry(Point, 4326) NOT NULL,
    PRIMARY KEY (category, osm_id)
);

CREATE INDEX IF NOT EXISTS idx_poi_points_geom ON poi_points USING GIST (geom);
CREATE INDEX IF NOT EXISTS idx_poi_points_category ON poi_points (category);

CREATE TABLE IF NOT EXISTS device_poi_feature_values (
    host_name TEXT NOT NULL,
    city TEXT NOT NULL,
    region_code TEXT NOT NULL,
    category TEXT NOT NULL,
    radius_m INTEGER NOT NULL,
    poi_count INTEGER NOT NULL,
    nearest_distance_km DOUBLE PRECISION,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC'),
    PRIMARY KEY (host_name, category, radius_m)
);

CREATE INDEX IF NOT EXISTS idx_device_poi_feature_values_city_region ON device_poi_feature_values (city, region_code);
CREATE INDEX IF NOT EXISTS idx_device_poi_feature_values_category_radius ON device_poi_feature_values (category, radius_m);

INSERT INTO poi_categories (category, point_where, polygon_where)
VALUES
    ('college', 'amenity IN (''university'',''college'')', 'amenity IN (''university'',''college'')'),
    ('dorm', 'building = ''dormitory''', 'building = ''dormitory'''),
    ('hostel', 'tourism = ''hostel''', 'tourism = ''hostel'''),
    ('restaurant', 'amenity = ''restaurant''', 'amenity = ''restaurant'''),
    ('pub_bar', 'amenity IN (''pub'',''bar'')', 'amenity IN (''pub'',''bar'')'),
    ('hotel', 'tourism IN (''hotel'',''motel'',''guest_house'',''hostel'')', 'tourism IN (''hotel'',''motel'',''guest_house'',''hostel'')'),
    ('mobile_shop', 'shop IN (''mobile_phone'',''electronics'')', 'shop IN (''mobile_phone'',''electronics'')')
ON CONFLICT (category) DO UPDATE SET
    point_where = EXCLUDED.point_where,
    polygon_where = EXCLUDED.polygon_where;

CREATE OR REPLACE FUNCTION rebuild_osm_poi_points()
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
    c RECORD;
BEGIN
    TRUNCATE TABLE poi_points;

    FOR c IN (SELECT category, point_where, polygon_where FROM poi_categories WHERE enabled = TRUE) LOOP
        EXECUTE format(
            'INSERT INTO poi_points (category, osm_id, geom)
             SELECT %L, osm_id, ST_Transform(way, 4326)::geometry(Point, 4326)
             FROM planet_osm_point
             WHERE %s AND way IS NOT NULL',
            c.category,
            c.point_where
        );

        EXECUTE format(
            'INSERT INTO poi_points (category, osm_id, geom)
             SELECT %L, osm_id, ST_Transform(ST_Centroid(way), 4326)::geometry(Point, 4326)
             FROM planet_osm_polygon
             WHERE %s AND way IS NOT NULL
             ON CONFLICT (category, osm_id) DO NOTHING',
            c.category,
            c.polygon_where
        );
    END LOOP;
END;
$$;

CREATE OR REPLACE FUNCTION refresh_device_poi_feature_values(p_city text, p_region_code text)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
    c RECORD;
    r INTEGER;
BEGIN
    FOR c IN (SELECT category FROM poi_categories WHERE enabled = TRUE) LOOP
        FOREACH r IN ARRAY ARRAY[1000, 2000] LOOP
            WITH device_points AS (
                SELECT
                    d.host_name,
                    d.device_config->>'city' AS city,
                    d.region->>'code' AS region_code,
                    CASE
                        WHEN jsonb_typeof(d.device_config->'LongLat') = 'array'
                            AND jsonb_array_length(d.device_config->'LongLat') = 2
                        THEN ST_SetSRID(
                                ST_MakePoint(
                                    (d.device_config->'LongLat'->>0)::double precision,
                                    (d.device_config->'LongLat'->>1)::double precision
                                ),
                                4326
                            )::geometry(Point, 4326)

                        WHEN NULLIF(d.device_config->'position'->>'longitude','') IS NOT NULL
                            AND NULLIF(d.device_config->'position'->>'latitude','') IS NOT NULL
                        THEN ST_SetSRID(
                                ST_MakePoint(
                                    NULLIF(d.device_config->'position'->>'longitude','')::double precision,
                                    NULLIF(d.device_config->'position'->>'latitude','')::double precision
                                ),
                                4326
                            )::geometry(Point, 4326)

                        ELSE NULL
                    END AS geom
                FROM devices d
                WHERE d.device_config->>'city' = p_city
                  AND d.region->>'code' = p_region_code
                  AND (d.device_config->>'test' IS NULL OR d.device_config->>'test' <> 'true')
            ),
            features AS (
                SELECT
                    dp.host_name,
                    dp.city,
                    dp.region_code,
                    (SELECT COUNT(*)
                     FROM poi_points pp
                     WHERE pp.category = c.category
                       AND dp.geom IS NOT NULL
                       AND ST_DWithin(pp.geom::geography, dp.geom::geography, r)
                    ) AS poi_count,
                    (SELECT MIN(ST_Distance(pp.geom::geography, dp.geom::geography) / 1000.0)
                     FROM poi_points pp
                     WHERE pp.category = c.category
                       AND dp.geom IS NOT NULL
                    ) AS nearest_distance_km
                FROM device_points dp
                WHERE dp.geom IS NOT NULL
            )
            INSERT INTO device_poi_feature_values (
                host_name, city, region_code,
                category, radius_m, poi_count, nearest_distance_km,
                updated_at
            )
            SELECT
                host_name, city, region_code,
                c.category, r, poi_count, nearest_distance_km,
                NOW() AT TIME ZONE 'UTC'
            FROM features
            ON CONFLICT (host_name, category, radius_m) DO UPDATE SET
                city = EXCLUDED.city,
                region_code = EXCLUDED.region_code,
                poi_count = EXCLUDED.poi_count,
                nearest_distance_km = EXCLUDED.nearest_distance_km,
                updated_at = EXCLUDED.updated_at;
        END LOOP;
    END LOOP;
END;
$$;
