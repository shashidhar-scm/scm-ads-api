CREATE OR REPLACE FUNCTION rebuild_osm_poi_cache()
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
    TRUNCATE TABLE poi_colleges;
    TRUNCATE TABLE poi_dorms;
    TRUNCATE TABLE poi_hostels;

    INSERT INTO poi_colleges (osm_id, geom)
    SELECT osm_id, ST_Transform(way, 4326)::geometry(Point, 4326)
    FROM planet_osm_point
    WHERE amenity IN ('university', 'college')
      AND way IS NOT NULL;

    INSERT INTO poi_colleges (osm_id, geom)
    SELECT osm_id, ST_Transform(ST_Centroid(way), 4326)::geometry(Point, 4326)
    FROM planet_osm_polygon
    WHERE amenity IN ('university', 'college')
      AND way IS NOT NULL
    ON CONFLICT (osm_id) DO NOTHING;

    INSERT INTO poi_dorms (osm_id, geom)
    SELECT osm_id, ST_Transform(way, 4326)::geometry(Point, 4326)
    FROM planet_osm_point
    WHERE building = 'dormitory'
      AND way IS NOT NULL;

    INSERT INTO poi_dorms (osm_id, geom)
    SELECT osm_id, ST_Transform(ST_Centroid(way), 4326)::geometry(Point, 4326)
    FROM planet_osm_polygon
    WHERE building = 'dormitory'
      AND way IS NOT NULL
    ON CONFLICT (osm_id) DO NOTHING;

    INSERT INTO poi_hostels (osm_id, geom)
    SELECT osm_id, ST_Transform(way, 4326)::geometry(Point, 4326)
    FROM planet_osm_point
    WHERE tourism = 'hostel'
      AND way IS NOT NULL;

    INSERT INTO poi_hostels (osm_id, geom)
    SELECT osm_id, ST_Transform(ST_Centroid(way), 4326)::geometry(Point, 4326)
    FROM planet_osm_polygon
    WHERE tourism = 'hostel'
      AND way IS NOT NULL
    ON CONFLICT (osm_id) DO NOTHING;
END;
$$;
