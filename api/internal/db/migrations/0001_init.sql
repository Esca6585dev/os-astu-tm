-- os.astu.tm ilkinji shema. Ähli pul mukdarlary teňňede (1 TMT = 100 teňňe).
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE users (
  id                BIGSERIAL PRIMARY KEY,
  email             CITEXT UNIQUE NOT NULL,
  password_hash     TEXT NOT NULL,
  full_name         TEXT NOT NULL DEFAULT '',
  phone             TEXT NOT NULL DEFAULT '',
  role              TEXT NOT NULL DEFAULT 'user'    CHECK (role IN ('user','admin')),
  status            TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','active','blocked')),
  balance           BIGINT NOT NULL DEFAULT 0,
  low_balance_notified_at TIMESTAMPTZ,
  email_verified_at TIMESTAMPTZ,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE email_tokens (
  id         BIGSERIAL PRIMARY KEY,
  user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  purpose    TEXT NOT NULL CHECK (purpose IN ('verify','reset')),
  token_hash TEXT UNIQUE NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  used_at    TIMESTAMPTZ
);

CREATE TABLE sessions (
  id         BIGSERIAL PRIMARY KEY,
  user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash TEXT UNIQUE NOT NULL,
  ip         TEXT NOT NULL DEFAULT '',
  user_agent TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX sessions_user_idx ON sessions(user_id);

CREATE TABLE ssh_keys (
  id          BIGSERIAL PRIMARY KEY,
  user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name        TEXT NOT NULL,
  public_key  TEXT NOT NULL,
  key_type    TEXT NOT NULL,
  fingerprint TEXT NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, fingerprint)
);

CREATE TABLE plans (
  id             BIGSERIAL PRIMARY KEY,
  code           TEXT UNIQUE NOT NULL,
  vcpu           INT NOT NULL,
  ram_mb         INT NOT NULL,
  disk_gb        INT NOT NULL,
  ip_count       INT NOT NULL DEFAULT 1,
  bandwidth_mbit INT NOT NULL,
  price_monthly  BIGINT NOT NULL,
  is_popular     BOOLEAN NOT NULL DEFAULT false,
  is_active      BOOLEAN NOT NULL DEFAULT true,
  sort           INT NOT NULL DEFAULT 0
);

CREATE TABLE images (
  id                    BIGSERIAL PRIMARY KEY,
  slug                  TEXT UNIQUE NOT NULL,
  name                  TEXT NOT NULL,
  version               TEXT NOT NULL,
  proxmox_template_vmid INT NOT NULL,
  is_active             BOOLEAN NOT NULL DEFAULT true,
  sort                  INT NOT NULL DEFAULT 0
);

CREATE TABLE nodes (
  id          BIGSERIAL PRIMARY KEY,
  name        TEXT UNIQUE NOT NULL,
  storage     TEXT NOT NULL DEFAULT 'local-lvm',
  bridge      TEXT NOT NULL DEFAULT 'vmbr0',
  max_vcpu    INT NOT NULL,
  max_ram_mb  INT NOT NULL,
  max_disk_gb INT NOT NULL,
  is_active   BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE ip_pools (
  id       BIGSERIAL PRIMARY KEY,
  node_id  BIGINT NOT NULL REFERENCES nodes(id),
  cidr     TEXT NOT NULL,
  gateway  TEXT NOT NULL,
  dns      TEXT NOT NULL DEFAULT '8.8.8.8 1.1.1.1'
);

CREATE TABLE servers (
  id           BIGSERIAL PRIMARY KEY,
  user_id      BIGINT NOT NULL REFERENCES users(id),
  plan_id      BIGINT NOT NULL REFERENCES plans(id),
  image_id     BIGINT NOT NULL REFERENCES images(id),
  node_id      BIGINT NOT NULL REFERENCES nodes(id),
  hostname     TEXT NOT NULL,
  proxmox_vmid INT UNIQUE,
  status       TEXT NOT NULL DEFAULT 'provisioning'
               CHECK (status IN ('provisioning','running','stopped','suspended','reinstalling','deleting','deleted','error')),
  busy         BOOLEAN NOT NULL DEFAULT false,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  suspended_at TIMESTAMPTZ,
  delete_warned_at TIMESTAMPTZ,
  deleted_at   TIMESTAMPTZ
);
CREATE INDEX servers_user_idx ON servers(user_id) WHERE deleted_at IS NULL;

CREATE TABLE ip_addresses (
  id        BIGSERIAL PRIMARY KEY,
  pool_id   BIGINT NOT NULL REFERENCES ip_pools(id) ON DELETE CASCADE,
  address   TEXT UNIQUE NOT NULL,
  prefix    INT NOT NULL,
  server_id BIGINT REFERENCES servers(id),
  reserved  BOOLEAN NOT NULL DEFAULT false
);

CREATE TABLE server_ssh_keys (
  server_id  BIGINT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  ssh_key_id BIGINT NOT NULL REFERENCES ssh_keys(id) ON DELETE CASCADE,
  PRIMARY KEY (server_id, ssh_key_id)
);

CREATE TABLE transactions (
  id            BIGSERIAL PRIMARY KEY,
  user_id       BIGINT NOT NULL REFERENCES users(id),
  type          TEXT NOT NULL CHECK (type IN ('topup','charge','refund','adjustment')),
  amount        BIGINT NOT NULL,
  balance_after BIGINT NOT NULL,
  server_id     BIGINT REFERENCES servers(id),
  admin_id      BIGINT REFERENCES users(id),
  note          TEXT NOT NULL DEFAULT '',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX transactions_user_idx ON transactions(user_id, created_at DESC);

CREATE TABLE server_usage (
  server_id      BIGINT NOT NULL REFERENCES servers(id),
  hour           TIMESTAMPTZ NOT NULL,
  amount         BIGINT NOT NULL,
  transaction_id BIGINT REFERENCES transactions(id),
  PRIMARY KEY (server_id, hour)
);

-- Job navbaty (Redis gerek däl): worker setirleri FOR UPDATE SKIP LOCKED bilen alýar.
CREATE TABLE jobs (
  id          BIGSERIAL PRIMARY KEY,
  server_id   BIGINT REFERENCES servers(id),
  action      TEXT NOT NULL,
  payload     JSONB NOT NULL DEFAULT '{}',
  status      TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','running','done','failed')),
  attempts    INT NOT NULL DEFAULT 0,
  error       TEXT NOT NULL DEFAULT '',
  run_after   TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  finished_at TIMESTAMPTZ
);
CREATE INDEX jobs_queue_idx ON jobs(run_after) WHERE status = 'queued';

CREATE TABLE audit_log (
  id         BIGSERIAL PRIMARY KEY,
  actor_id   BIGINT REFERENCES users(id),
  action     TEXT NOT NULL,
  target     TEXT NOT NULL DEFAULT '',
  meta       JSONB NOT NULL DEFAULT '{}',
  ip         TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- VPS Hosting nyrhnamasy (teňňede)
INSERT INTO plans (code, vcpu, ram_mb, disk_gb, ip_count, bandwidth_mbit, price_monthly, is_popular, sort) VALUES
  ('VPS A-2020', 1, 1024,  20, 1,  8, 14000, false, 1),
  ('VPS B-2020', 1, 1024,  40, 1,  8, 17500, false, 2),
  ('VPS C-2020', 2, 2048,  60, 1, 12, 29750, true,  3),
  ('VPS D-2020', 4, 4096,  80, 1, 24, 42000, false, 4),
  ('VPS E-2020', 6, 6144, 120, 1, 32, 59500, false, 5);
