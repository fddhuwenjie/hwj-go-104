package sqlite

// schema 数据库结构：时间统一存 Unix 毫秒（INTEGER），JSON 存 TEXT。
const schema = `
PRAGMA journal_mode=WAL;

CREATE TABLE IF NOT EXISTS product_families (
  id TEXT PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS stations (
  id TEXT PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  capability TEXT NOT NULL,
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS routes (
  id TEXT PRIMARY KEY,
  product_family_id TEXT NOT NULL REFERENCES product_families(id),
  code TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS route_revisions (
  id TEXT PRIMARY KEY,
  route_id TEXT NOT NULL REFERENCES routes(id),
  revision INTEGER NOT NULL,
  status TEXT NOT NULL,
  rework_from_hold_id TEXT NOT NULL DEFAULT '',
  reentry_seq INTEGER NOT NULL DEFAULT 0,
  version INTEGER NOT NULL DEFAULT 1,
  created_at INTEGER NOT NULL,
  UNIQUE(route_id, revision)
);

CREATE TABLE IF NOT EXISTS route_stations (
  id TEXT PRIMARY KEY,
  route_revision_id TEXT NOT NULL REFERENCES route_revisions(id),
  seq INTEGER NOT NULL,
  station_id TEXT NOT NULL REFERENCES stations(id),
  recipe_id TEXT NOT NULL,
  metrology_plan_id TEXT NOT NULL,
  UNIQUE(route_revision_id, seq)
);

CREATE TABLE IF NOT EXISTS recipes (
  id TEXT PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  equipment_family TEXT NOT NULL,
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS recipe_versions (
  id TEXT PRIMARY KEY,
  recipe_id TEXT NOT NULL REFERENCES recipes(id),
  version INTEGER NOT NULL,
  status TEXT NOT NULL,
  params_json TEXT NOT NULL,
  snapshot TEXT NOT NULL DEFAULT '',
  activated_at INTEGER,
  row_version INTEGER NOT NULL DEFAULT 1,
  created_at INTEGER NOT NULL,
  UNIQUE(recipe_id, version)
);

CREATE TABLE IF NOT EXISTS equipment (
  id TEXT PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  family TEXT NOT NULL,
  station_id TEXT NOT NULL REFERENCES stations(id),
  status TEXT NOT NULL,
  version INTEGER NOT NULL DEFAULT 1,
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS chambers (
  id TEXT PRIMARY KEY,
  equipment_id TEXT NOT NULL REFERENCES equipment(id),
  code TEXT NOT NULL,
  capability TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  UNIQUE(equipment_id, code)
);

CREATE TABLE IF NOT EXISTS qualifications (
  id TEXT PRIMARY KEY,
  equipment_id TEXT NOT NULL REFERENCES equipment(id),
  chamber_id TEXT NOT NULL DEFAULT '',
  station_id TEXT NOT NULL REFERENCES stations(id),
  valid_from INTEGER NOT NULL,
  valid_to INTEGER NOT NULL,
  status TEXT NOT NULL,
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS metrology_plans (
  id TEXT PRIMARY KEY,
  code TEXT NOT NULL,
  name TEXT NOT NULL,
  version INTEGER NOT NULL,
  status TEXT NOT NULL,
  sample_positions TEXT NOT NULL,
  min_samples INTEGER NOT NULL,
  pass_limit REAL NOT NULL,
  metric TEXT NOT NULL,
  row_version INTEGER NOT NULL DEFAULT 1,
  created_at INTEGER NOT NULL,
  UNIQUE(code, version)
);

CREATE TABLE IF NOT EXISTS lots (
  id TEXT PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  product_family_id TEXT NOT NULL REFERENCES product_families(id),
  route_id TEXT NOT NULL REFERENCES routes(id),
  status TEXT NOT NULL,
  current_seq INTEGER NOT NULL DEFAULT 0,
  frozen_revision_id TEXT NOT NULL DEFAULT '',
  freeze_snapshot TEXT NOT NULL DEFAULT '',
  frozen_at INTEGER,
  parent_lot_id TEXT NOT NULL DEFAULT '',
  entered_at INTEGER,
  version INTEGER NOT NULL DEFAULT 1,
  created_at INTEGER NOT NULL,
  closed_at INTEGER
);
CREATE INDEX IF NOT EXISTS idx_lots_parent ON lots(parent_lot_id);
CREATE INDEX IF NOT EXISTS idx_lots_status ON lots(status);

CREATE TABLE IF NOT EXISTS wafers (
  id TEXT PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  lot_id TEXT NOT NULL REFERENCES lots(id),
  slot INTEGER NOT NULL,
  status TEXT NOT NULL,
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_wafers_lot ON wafers(lot_id);

CREATE TABLE IF NOT EXISTS wafer_moves (
  id TEXT PRIMARY KEY,
  wafer_id TEXT NOT NULL REFERENCES wafers(id),
  from_lot_id TEXT NOT NULL,
  to_lot_id TEXT NOT NULL,
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_wafer_moves_wafer ON wafer_moves(wafer_id);

CREATE TABLE IF NOT EXISTS runs (
  id TEXT PRIMARY KEY,
  lot_id TEXT NOT NULL REFERENCES lots(id),
  route_revision_id TEXT NOT NULL,
  station_seq INTEGER NOT NULL,
  station_id TEXT NOT NULL,
  equipment_id TEXT NOT NULL,
  chamber_id TEXT NOT NULL,
  recipe_version_id TEXT NOT NULL,
  recipe_snapshot TEXT NOT NULL,
  status TEXT NOT NULL,
  judgment TEXT NOT NULL DEFAULT 'NONE',
  qual_covered INTEGER NOT NULL DEFAULT 1,
  reviewed INTEGER NOT NULL DEFAULT 0,
  started_at INTEGER NOT NULL,
  completed_at INTEGER,
  version INTEGER NOT NULL DEFAULT 1,
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_runs_lot ON runs(lot_id);
CREATE INDEX IF NOT EXISTS idx_runs_status ON runs(status);

CREATE TABLE IF NOT EXISTS run_wafers (
  run_id TEXT NOT NULL REFERENCES runs(id),
  wafer_id TEXT NOT NULL REFERENCES wafers(id),
  PRIMARY KEY (run_id, wafer_id)
);
CREATE INDEX IF NOT EXISTS idx_run_wafers_wafer ON run_wafers(wafer_id);

CREATE TABLE IF NOT EXISTS readings (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES runs(id),
  wafer_id TEXT NOT NULL,
  slot INTEGER NOT NULL,
  metric TEXT NOT NULL,
  value REAL NOT NULL,
  late INTEGER NOT NULL DEFAULT 0,
  sealed INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_readings_run ON readings(run_id);

CREATE TABLE IF NOT EXISTS holds (
  id TEXT PRIMARY KEY,
  lot_id TEXT NOT NULL REFERENCES lots(id),
  run_id TEXT NOT NULL DEFAULT '',
  reason TEXT NOT NULL,
  status TEXT NOT NULL,
  escalated INTEGER NOT NULL DEFAULT 0,
  review_note TEXT NOT NULL DEFAULT '',
  version INTEGER NOT NULL DEFAULT 1,
  created_at INTEGER NOT NULL,
  closed_at INTEGER
);
CREATE INDEX IF NOT EXISTS idx_holds_lot ON holds(lot_id);
CREATE INDEX IF NOT EXISTS idx_holds_status ON holds(status);

CREATE TABLE IF NOT EXISTS rework_records (
  id TEXT PRIMARY KEY,
  lot_id TEXT NOT NULL,
  hold_id TEXT NOT NULL,
  new_revision_id TEXT NOT NULL,
  reentry_seq INTEGER NOT NULL,
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_rework_lot ON rework_records(lot_id);

CREATE TABLE IF NOT EXISTS releases (
  id TEXT PRIMARY KEY,
  lot_id TEXT NOT NULL REFERENCES lots(id),
  from_seq INTEGER NOT NULL,
  to_seq INTEGER NOT NULL,
  kind TEXT NOT NULL,
  note TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_releases_lot ON releases(lot_id);

CREATE TABLE IF NOT EXISTS audit_events (
  id TEXT PRIMARY KEY,
  entity TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  action TEXT NOT NULL,
  detail TEXT NOT NULL DEFAULT '',
  tx_tag TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_entity ON audit_events(entity, entity_id);

CREATE TABLE IF NOT EXISTS idempotency_keys (
  scope TEXT NOT NULL,
  key TEXT NOT NULL,
  response TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  PRIMARY KEY (scope, key)
);

CREATE TABLE IF NOT EXISTS jobs (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  payload TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  attempts INTEGER NOT NULL DEFAULT 0,
  max_attempts INTEGER NOT NULL DEFAULT 3,
  run_at INTEGER NOT NULL,
  last_error TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_jobs_status_runat ON jobs(status, run_at);
`
