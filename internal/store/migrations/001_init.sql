CREATE TABLE IF NOT EXISTS accounts (
  id           BIGSERIAL PRIMARY KEY,
  username     TEXT UNIQUE NOT NULL,
  pass_hash    TEXT NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  banned_until TIMESTAMPTZ,
  ban_reason   TEXT NOT NULL DEFAULT '',
  muted_until  TIMESTAMPTZ,
  is_admin     BOOLEAN NOT NULL DEFAULT false
);
CREATE TABLE IF NOT EXISTS characters (
  id         BIGSERIAL PRIMARY KEY,
  account_id BIGINT UNIQUE NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  name       TEXT NOT NULL,
  outfit     JSONB NOT NULL,
  data       JSONB NOT NULL,
  money      INT NOT NULL DEFAULT 0,
  rep        INT NOT NULL DEFAULT 0,
  points     INT NOT NULL DEFAULT 0,
  escapes    INT NOT NULL DEFAULT 0,
  releases   INT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS characters_rep_idx    ON characters (rep DESC);
CREATE INDEX IF NOT EXISTS characters_money_idx  ON characters (money DESC);
CREATE INDEX IF NOT EXISTS characters_points_idx ON characters (points DESC);
CREATE TABLE IF NOT EXISTS reports (
  id         BIGSERIAL PRIMARY KEY,
  reporter   BIGINT NOT NULL,
  target     BIGINT NOT NULL,
  reason     TEXT NOT NULL,
  handled    BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS characters_name_lower_idx ON characters (lower(name));
