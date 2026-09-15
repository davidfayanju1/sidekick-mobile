-- Phase 3 — discovery indexes (§1). IF NOT EXISTS so reruns are safe.
-- Geo uses earthdistance (Phase 0 decision: lighter than PostGIS).
-- Search uses both FTS (ranking) and trigrams (partial/fuzzy match).

CREATE INDEX IF NOT EXISTS idx_tasks_earth ON tasks
  USING gist (ll_to_earth(location_lat, location_lng))
  WHERE location_lat IS NOT NULL AND location_lng IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_tasks_fts ON tasks
  USING gin (to_tsvector('english', title || ' ' || description));

CREATE INDEX IF NOT EXISTS idx_tasks_title_trgm ON tasks
  USING gin (title gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_tasks_desc_trgm ON tasks
  USING gin (description gin_trgm_ops);
