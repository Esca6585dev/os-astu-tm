-- Bekaplar, ulanyjy firewall-y we balans doldurma haýyşlary.
-- Mundan beýläk balans gutaranda maşyn diňe öçürilýär (suspended), awtomatik pozulmaýar.

ALTER TABLE servers DROP CONSTRAINT servers_status_check;
ALTER TABLE servers ADD CONSTRAINT servers_status_check
  CHECK (status IN ('provisioning','running','stopped','suspended','reinstalling','restoring','deleting','deleted','error'));
ALTER TABLE servers DROP COLUMN delete_warned_at;
ALTER TABLE servers ADD COLUMN fw_policy_in TEXT NOT NULL DEFAULT 'ACCEPT' CHECK (fw_policy_in IN ('ACCEPT','DROP'));

-- Ulanyjynyň firewall düzgünleri. Proxmox-a pos tertibinde (ýokardan aşak) ýazylýar.
-- addr: "in" düzgünde çeşme (source), "out" düzgünde barýan ýeri (dest).
CREATE TABLE firewall_rules (
  id        BIGSERIAL PRIMARY KEY,
  server_id BIGINT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  pos       INT NOT NULL,
  direction TEXT NOT NULL CHECK (direction IN ('in','out')),
  action    TEXT NOT NULL CHECK (action IN ('ACCEPT','DROP')),
  proto     TEXT NOT NULL DEFAULT '' CHECK (proto IN ('','tcp','udp','icmp')),
  dport     TEXT NOT NULL DEFAULT '',
  addr      TEXT NOT NULL DEFAULT '',
  comment   TEXT NOT NULL DEFAULT ''
);
CREATE INDEX firewall_rules_server_idx ON firewall_rules(server_id, pos);

-- Proxmox vzdump bekaplary. volid: "local:backup/vzdump-qemu-1002-2026_11_13-10_00_00.vma.zst"
CREATE TABLE backups (
  id          BIGSERIAL PRIMARY KEY,
  server_id   BIGINT NOT NULL REFERENCES servers(id),
  image_id    BIGINT NOT NULL REFERENCES images(id),
  note        TEXT NOT NULL DEFAULT '',
  status      TEXT NOT NULL DEFAULT 'creating'
              CHECK (status IN ('creating','ready','restoring','deleting','deleted','error')),
  storage     TEXT NOT NULL,
  volid       TEXT,
  size_bytes  BIGINT NOT NULL DEFAULT 0,
  error       TEXT NOT NULL DEFAULT '',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  finished_at TIMESTAMPTZ
);
CREATE INDEX backups_server_idx ON backups(server_id) WHERE status <> 'deleted';

-- Ulanyjy tölegi geçirýär we haýyş iberýär; administrator tassyklanda balansa goşulýar.
CREATE TABLE topup_requests (
  id             BIGSERIAL PRIMARY KEY,
  user_id        BIGINT NOT NULL REFERENCES users(id),
  amount         BIGINT NOT NULL CHECK (amount > 0),
  method         TEXT NOT NULL CHECK (method IN ('bank','cash','card')),
  reference      TEXT NOT NULL,
  comment        TEXT NOT NULL DEFAULT '',
  status         TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected','cancelled')),
  admin_id       BIGINT REFERENCES users(id),
  admin_note     TEXT NOT NULL DEFAULT '',
  transaction_id BIGINT REFERENCES transactions(id),
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  decided_at     TIMESTAMPTZ
);
CREATE INDEX topup_requests_status_idx ON topup_requests(status, created_at);
CREATE INDEX topup_requests_user_idx ON topup_requests(user_id, created_at DESC);
