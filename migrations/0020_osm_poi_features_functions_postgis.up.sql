CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE IF NOT EXISTS poi_colleges (
    osm_id BIGINT PRIMARY KEY,
    geom geometry(Point, 4326) NOT NULL
);

CREATE TABLE IF NOT EXISTS poi_dorms (
    osm_id BIGINT PRIMARY KEY,
    geom geometry(Point, 4326) NOT NULL
);

CREATE TABLE IF NOT EXISTS poi_hostels (
    osm_id BIGINT PRIMARY KEY,
    geom geometry(Point, 4326) NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_poi_colleges_geom ON poi_colleges USING GIST (geom);
CREATE INDEX IF NOT EXISTS idx_poi_dorms_geom ON poi_dorms USING GIST (geom);
CREATE INDEX IF NOT EXISTS idx_poi_hostels_geom ON poi_hostels USING GIST (geom);

CREATE OR REPLACE FUNCTION rebuild_osm_poi_cache()
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
    TRUNCATE TABLE poi_colleges;
    TRUNCATE TABLE poi_dorms;
    TRUNCATE TABLE poi_hostels;

    INSERT INTO poi_colleges (osm_id, geom)
    SELECT osm_id, way::geometry(Point, 4326)
    FROM planet_osm_point
    WHERE amenity IN ('university', 'college')
      AND way IS NOT NULL;

    INSERT INTO poi_colleges (osm_id, geom)
    SELECT osm_id, ST_Centroid(way)::geometry(Point, 4326)
    FROM planet_osm_polygon
    WHERE amenity IN ('university', 'college')
      AND way IS NOT NULL
    ON CONFLICT (osm_id) DO NOTHING;

    INSERT INTO poi_dorms (osm_id, geom)
    SELECT osm_id, way::geometry(Point, 4326)
    FROM planet_osm_point
    WHERE building = 'dormitory'
      AND way IS NOT NULL;

    INSERT INTO poi_dorms (osm_id, geom)
    SELECT osm_id, ST_Centroid(way)::geometry(Point, 4326)
    FROM planet_osm_polygon
    WHERE building = 'dormitory'
      AND way IS NOT NULL
    ON CONFLICT (osm_id) DO NOTHING;

    INSERT INTO poi_hostels (osm_id, geom)
    SELECT osm_id, way::geometry(Point, 4326)
    FROM planet_osm_point
    WHERE tourism = 'hostel'
      AND way IS NOT NULL;

    INSERT INTO poi_hostels (osm_id, geom)
    SELECT osm_id, ST_Centroid(way)::geometry(Point, 4326)
    FROM planet_osm_polygon
    WHERE tourism = 'hostel'
      AND way IS NOT NULL
    ON CONFLICT (osm_id) DO NOTHING;
END;
$$;

CREATE OR REPLACE FUNCTION refresh_device_poi_features(p_city text, p_region_code text)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
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
            (SELECT COUNT(*) FROM poi_colleges pc
              WHERE dp.geom IS NOT NULL
                AND ST_DWithin(pc.geom::geography, dp.geom::geography, 1000)
            ) AS college_count_1km,
            (SELECT COUNT(*) FROM poi_colleges pc
              WHERE dp.geom IS NOT NULL
                AND ST_DWithin(pc.geom::geography, dp.geom::geography, 2000)
            ) AS college_count_2km,
            (SELECT COUNT(*) FROM poi_dorms pd
              WHERE dp.geom IS NOT NULL
                AND ST_DWithin(pd.geom::geography, dp.geom::geography, 1000)
            ) AS dorm_count_1km,
            (SELECT COUNT(*) FROM poi_hostels ph
              WHERE dp.geom IS NOT NULL
                AND ST_DWithin(ph.geom::geography, dp.geom::geography, 1000)
            ) AS hostel_count_1km,
            (SELECT MIN(ST_Distance(pc.geom::geography, dp.geom::geography) / 1000.0)
              FROM poi_colleges pc
              WHERE dp.geom IS NOT NULL
            ) AS nearest_college_distance_km
        FROM device_points dp
        WHERE dp.geom IS NOT NULL
    )
    INSERT INTO device_poi_features (
        host_name, city, region_code,
        college_count_1km, college_count_2km,
        dorm_count_1km, hostel_count_1km,
        nearest_college_distance_km, updated_at
    )
    SELECT
        host_name, city, region_code,
        college_count_1km, college_count_2km,
        dorm_count_1km, hostel_count_1km,
        nearest_college_distance_km,
        NOW() AT TIME ZONE 'UTC'
    FROM features
    ON CONFLICT (host_name) DO UPDATE SET
        city = EXCLUDED.city,
        region_code = EXCLUDED.region_code,
        college_count_1km = EXCLUDED.college_count_1km,
        college_count_2km = EXCLUDED.college_count_2km,
        dorm_count_1km = EXCLUDED.dorm_count_1km,
        hostel_count_1km = EXCLUDED.hostel_count_1km,
        nearest_college_distance_km = EXCLUDED.nearest_college_distance_km,
        updated_at = EXCLUDED.updated_at;
END;
$$;
