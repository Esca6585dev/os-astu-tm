# os.astu.tm — VPS hosting paneli: doly prompt

Bu prompt AI kod kömekçisine (Claude Code we ş.m.) ýa-da developere berlende, şu repoda doly işleýän VPS hosting panelini gurmak üçin ýazyldy. `test/` papkasyndaky statik wersiýa dizaýnyň nusgasydyr: sahypalar, reňkler, tekstler we hereketler şondan alynýar.

---

## 1. Maksat

ASTU-nyň serwerlerinde **os.telecom.tm** ýaly VPS hosting panelini gur. Ol `os.astu.tm` salgysynda işlär.

Ulanyjy:
1. hasaba alynýar, emailini tassyklaýar we girýär;
2. balansyny görýär (balansy administrator doldurýar);
3. obraz (OS) we konfigurasiýa (tarif) saýlap, maşyn (VPS) döredýär;
4. maşyna root parol ýa-da SSH açar bilen SSH arkaly girýär;
5. maşyny başladýar, durdurýar, täzeden ýükleýär, OS-i täzeden gurýar, root paroly täzeleýär we pozýar.

Pul her sagat balansdan aýrylýar. Balans gutarsa, maşynlar saklanýar (suspend).

## 2. Tehnologiýalar

| Gatlak | Tehnologiýa |
|---|---|
| Backend API | Go 1.23+, Fiber v2, pgx v5, goose (migrasiýalar) |
| Job navbaty / worker | asynq (Redis) |
| Maglumat bazasy | PostgreSQL 16 (`citext` giňeltmesi) |
| Keş / navbat | Redis 7 |
| Frontend | Next.js 15 (App Router, TypeScript), Tailwind CSS |
| Wirtualizasiýa | Proxmox VE 8, REST API, API token |
| Deploy | Docker Compose, nginx (TLS) |

## 3. Repo gurluşy

```text
os-astu-tm/
  api/
    cmd/server/main.go        # Fiber HTTP API
    cmd/worker/main.go        # asynq worker + sagatlaýyn billing cron
    internal/
      config/                 # env okamak
      db/                     # pgx pool
      auth/                   # argon2id, JWT, email tokenleri
      user/ plan/ image/ sshkey/ server/ billing/ admin/ ippool/
      proxmox/                # Hypervisor interfeýsi + Proxmox adapteri
      jobs/                   # asynq task görnüşleri
      middleware/             # auth, rate limit, audit, CORS
    migrations/               # goose SQL faýllary
  web/                        # Next.js
    app/(auth)/login, register, verify, forgot
    app/(panel)/dashboard, machines, machines/new, machines/[id],
                ssh-keys, images, plans, billing
    app/admin/users, servers, plans, images, nodes, ip-pools, audit
    components/  lib/api.ts
    public/logo.svg           # test/assets/logo.svg şu ýere göçürilýär
  deploy/
    docker-compose.yml        # api, worker, web, postgres, redis, nginx
    nginx.conf
  test/                       # statik dizaýn nusgasy (şu prompt bilen gelýär)
  PROMPT.md  README.md
```

`Hypervisor` interfeýsi bolmaly. Şonda soňra OpenStack ýa-da başga platforma üçin diňe täze adapter ýazmak ýeterlik bolar.

## 4. Tarifler (VPS Hosting nyrhnamasy)

| Kod | Aýlyk töleg (TMT) | CPU | RAM | HDD | IP | Tizlik |
|---|---|---|---|---|---|---|
| VPS A-2020 | 140.00 | 1 | 1 GB | 20 GB | 1 | 8 Mbit/s |
| VPS B-2020 | 175.00 | 1 | 1 GB | 40 GB | 1 | 8 Mbit/s |
| VPS C-2020 | 297.50 | 2 | 2 GB | 60 GB | 1 | 12 Mbit/s |
| VPS D-2020 | 420.00 | 4 | 4 GB | 80 GB | 1 | 24 Mbit/s |
| VPS E-2020 | 595.00 | 6 | 6 GB | 120 GB | 1 | 32 Mbit/s |

Bu tarifler ilkinji migrasiýada `plans` tablisasyna seed edilýär. Admin paneli arkaly olary üýtgedip bolýar.

## 5. Billing düzgünleri

- Ähli pul mukdarlary **teňňede** (`BIGINT`) saklanýar: 1 TMT = 100 teňňe. Float ulanylmaýar.
- Bir aý = 720 sagat diýlip hasaplanýar. Sagatlaýyn baha = `price_monthly / 720`.
- Tegelekleme ýalňyşy ýygnanmaz ýaly, her sagat şu tutulýar:
  `charge = round(price_monthly × şu aýda hasaplanan sagat / 720) − şu aý eýýäm tutulan`.
  Netijede bir aýda tutulan jemi hiç haçan `price_monthly`-dan geçmeýär.
- `running` we `stopped` maşynlar üçin pul tutulýar, sebäbi disk we IP saklanýar. `suspended` maşyndan tutulmaýar.
- Her sagadyň başynda worker şuny bir DB tranzaksiýasynda edýär: `server_usage (server_id, hour)` setiri `ON CONFLICT DO NOTHING` bilen goşulýar. Setir täze bolsa, balans azaldylýar we `transactions`-a `charge` ýazgysy goşulýar. Şeýlelikde bir sagat iki gezek tutulmaýar.
- Maşyn döretmek üçin balans azyndan 24 sagatlyk tutuma ýetmeli, ýetmese API `402` gaýtarýar.
- Balans 24 sagatlyk tutumdan az galanda email duýduryşy iberilýär.
- Balans 0-dan aşak düşende ulanyjynyň ähli maşynlary durdurylyp `suspended` ýagdaýyna geçýär. Balans doldurylanda olar awtomatik başlaýar.
- 7 gün `suspended` bolup galan maşyn diski bilen pozulýar we IP boşadylýar. Mundan 2 gün öň duýduryş iberilýär.
- MVP-de balansy diňe administrator elde doldurýar (bellik bilen, `audit_log`-a ýazylýar). Bank töleg şlýuzy soňky tapgyrda goşulýar.

## 6. Maglumat bazasy

```sql
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE users (
  id BIGSERIAL PRIMARY KEY,
  account_no TEXT UNIQUE NOT NULL,                -- U-10482
  email CITEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,                    -- argon2id
  full_name TEXT, phone TEXT,
  role TEXT NOT NULL DEFAULT 'user',              -- user | admin
  status TEXT NOT NULL DEFAULT 'pending',         -- pending | active | suspended | blocked
  balance BIGINT NOT NULL DEFAULT 0,              -- teňňe
  email_verified_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE email_tokens (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  purpose TEXT NOT NULL,                          -- verify | reset
  token_hash TEXT UNIQUE NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL, used_at TIMESTAMPTZ
);
CREATE TABLE sessions (
  id UUID PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  refresh_hash TEXT NOT NULL, ip INET, user_agent TEXT,
  expires_at TIMESTAMPTZ NOT NULL, revoked_at TIMESTAMPTZ
);
CREATE TABLE ssh_keys (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL, public_key TEXT NOT NULL,
  key_type TEXT NOT NULL, fingerprint TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, fingerprint)
);
CREATE TABLE plans (
  id BIGSERIAL PRIMARY KEY,
  code TEXT UNIQUE NOT NULL,                      -- VPS A-2020
  vcpu INT NOT NULL, ram_mb INT NOT NULL, disk_gb INT NOT NULL,
  ip_count INT NOT NULL DEFAULT 1,
  bandwidth_mbit INT NOT NULL,
  price_monthly BIGINT NOT NULL,                  -- teňňe
  is_popular BOOLEAN NOT NULL DEFAULT false,
  is_active BOOLEAN NOT NULL DEFAULT true,
  sort INT NOT NULL DEFAULT 0
);
CREATE TABLE images (
  id BIGSERIAL PRIMARY KEY,
  slug TEXT UNIQUE NOT NULL,                      -- ubuntu-24_04_2025-10
  name TEXT NOT NULL, version TEXT NOT NULL,
  proxmox_template_vmid INT NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT true
);
CREATE TABLE nodes (
  id BIGSERIAL PRIMARY KEY,
  name TEXT UNIQUE NOT NULL, api_url TEXT NOT NULL,
  storage TEXT NOT NULL, bridge TEXT NOT NULL DEFAULT 'vmbr0',
  max_vcpu INT NOT NULL, max_ram_mb INT NOT NULL, max_disk_gb INT NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT true
);
CREATE TABLE ip_pools (
  id BIGSERIAL PRIMARY KEY,
  node_id BIGINT REFERENCES nodes(id),
  cidr CIDR NOT NULL, gateway INET NOT NULL,
  dns TEXT NOT NULL DEFAULT '8.8.8.8 1.1.1.1'
);
CREATE TABLE ip_addresses (
  id BIGSERIAL PRIMARY KEY,
  pool_id BIGINT NOT NULL REFERENCES ip_pools(id),
  address INET UNIQUE NOT NULL,
  server_id BIGINT,
  reserved BOOLEAN NOT NULL DEFAULT false
);
CREATE TABLE servers (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id),
  plan_id BIGINT NOT NULL REFERENCES plans(id),
  image_id BIGINT NOT NULL REFERENCES images(id),
  node_id BIGINT NOT NULL REFERENCES nodes(id),
  hostname TEXT NOT NULL,
  proxmox_vmid INT UNIQUE,
  ip_id BIGINT REFERENCES ip_addresses(id),
  status TEXT NOT NULL DEFAULT 'provisioning',
    -- provisioning | running | stopped | suspended | deleting | deleted | error
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  suspended_at TIMESTAMPTZ, deleted_at TIMESTAMPTZ
);
ALTER TABLE ip_addresses ADD FOREIGN KEY (server_id) REFERENCES servers(id);
CREATE TABLE server_ssh_keys (
  server_id BIGINT REFERENCES servers(id) ON DELETE CASCADE,
  ssh_key_id BIGINT REFERENCES ssh_keys(id) ON DELETE CASCADE,
  PRIMARY KEY (server_id, ssh_key_id)
);
CREATE TABLE transactions (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id),
  type TEXT NOT NULL,                             -- topup | charge | refund | adjustment
  amount BIGINT NOT NULL,                         -- + goşmak, − aýyrmak
  balance_after BIGINT NOT NULL,
  server_id BIGINT REFERENCES servers(id),
  admin_id BIGINT REFERENCES users(id),
  note TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE server_usage (
  server_id BIGINT NOT NULL REFERENCES servers(id),
  hour TIMESTAMPTZ NOT NULL,
  amount BIGINT NOT NULL,
  transaction_id BIGINT REFERENCES transactions(id),
  PRIMARY KEY (server_id, hour)
);
CREATE TABLE jobs_log (
  id BIGSERIAL PRIMARY KEY,
  server_id BIGINT REFERENCES servers(id),
  action TEXT NOT NULL,     -- create | start | stop | reboot | reinstall | reset_password | delete | suspend | unsuspend
  status TEXT NOT NULL,     -- queued | running | done | failed
  proxmox_upid TEXT, error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), finished_at TIMESTAMPTZ
);
CREATE TABLE audit_log (
  id BIGSERIAL PRIMARY KEY,
  actor_id BIGINT REFERENCES users(id),
  action TEXT NOT NULL, target TEXT, meta JSONB, ip INET,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

## 7. API (`/api/v1`)

Access token 15 minutlyk, refresh token 30 günlik bolýar. Ikisi hem `httpOnly`, `Secure`, `SameSite=Lax` cookie-de saklanýar. Ýalňyşlyklar `{ "error": { "code": "...", "message": "..." } }` görnüşinde gaýtarylýar.

| Method | Path | Wezipesi |
|---|---|---|
| POST | `/auth/register` | Hasaba alyş, tassyklama emaili |
| POST | `/auth/verify` | Email tokenini tassyklamak |
| POST | `/auth/login`, `/auth/refresh`, `/auth/logout` | Sessiýa |
| POST | `/auth/forgot`, `/auth/reset` | Paroly dikeltmek |
| GET / PATCH | `/me` | Profil, parol üýtgetmek |
| GET | `/me/balance` | Balans, sagatlaýyn tutum, näçe gün ýetjegi |
| GET | `/me/transactions` | Amallar taryhy (`?type=&page=`) |
| GET / POST / DELETE | `/ssh-keys` | SSH açarlar (açyk açar we fingerprint barlanýar) |
| GET | `/plans`, `/images` | Konfigurasiýalar we obrazlar |
| GET / POST | `/servers` | Sanaw / täze maşyn: `plan_id`, `image_id`, `hostname`, `ssh_key_ids[]` ýa-da `password` |
| GET / DELETE | `/servers/:id` | Jikme-jiklik (CPU/RAM/disk/tor ulanyşy bilen) / pozmak |
| POST | `/servers/:id/start`, `/stop`, `/reboot` | Power amallary → `202` + job id |
| POST | `/servers/:id/reinstall` | `image_id` bilen OS-i täzeden gurmak |
| POST | `/servers/:id/reset-password` | Täze root parol (diňe jogapda bir gezek gaýtarylýar) |
| GET | `/servers/:id/jobs` | Amallaryň ýagdaýy (frontend polling) |
| GET | `/servers/:id/console` | noVNC üçin wagtlaýyn bilet (MVP-den soň) |
| GET / PATCH | `/admin/users`, `/admin/users/:id` | Ulanyjylar, bloklamak |
| POST | `/admin/users/:id/topup` | Balansy elde doldurmak: `amount`, `note` |
| CRUD | `/admin/plans`, `/admin/images`, `/admin/nodes`, `/admin/ip-pools` | Infrastruktura |
| GET | `/admin/servers`, `/admin/transactions`, `/admin/audit`, `/admin/stats` | Gözegçilik |

Rate limit: `/auth/*` üçin bir IP-den minutda 10 soraga, `POST /servers` üçin bir ulanyjydan sagatda 10 soraga rugsat berilýär.

## 8. Maşyn döretmegiň prosesi (worker)

1. **API:** balansy barlaýar, bir DB tranzaksiýasynda resursy ýeterlik node saýlaýar we boş IP-ni `SELECT … FOR UPDATE SKIP LOCKED` bilen bron edýär. `servers` setiri `provisioning` ýagdaýynda döredilýär, `create` job navbata goşulýar we `202` gaýtarylýar.
2. **Worker:** `POST /nodes/{node}/qemu/{template_vmid}/clone` (`newid`, `full=1`, `storage`, `name=hostname`) çagyrýar we UPID task tamamlanýança garaşýar.
3. `PUT /nodes/{node}/qemu/{vmid}/config` arkaly `cores`, `memory`, `net0=virtio,bridge=vmbr0,rate=<Mbit/8 MB/s>,firewall=1`, `ipconfig0=ip=X/NN,gw=G`, `nameserver`, `ciuser=root`, `cipassword` ýa-da `sshkeys` (URL-encoded) goýulýar.
4. `PUT …/resize` (`disk=scsi0`, `size=<disk_gb>G`) çagyrylýar, soňra `POST …/status/start`.
5. Status `running` bolýar we billing başlaýar. Parol DB-de saklanmaýar.
6. Ýalňyşlyk bolsa VM pozulýar, IP boşadylýar, status `error` bolýar we pul tutulmaýar.

## 9. Frontend (Next.js)

`test/` papkasyndaky dizaýny takyk gaýtala:

- **Reňkler:** esasy `#196CFC`, gyzgyn `#1B44C8` (sidebar we hover), açyk aksent `#3E9BFF`, ýumşak fon `#EEF5FF`, sahypa fony `#F6F7F9`. Olary Tailwind temasyna `primary`, `primary-dark`, `primary-light`, `primary-soft` atlary bilen goş.
- **Şriftler:** Inter (tekst), Space Grotesk (sözbaşylar).
- **Logo:** `public/logo.svg`. Gök sidebarda ak gutynyň içinde görkezilýär, sebäbi logonyň ýazgylary gara.
- **Gurluş:** çepde menýu. Onuň bölümleri: Baş sahypa; Hasaplaýyş (Maşynlar, SSH açarlar, Obrazlar, Konfigurasiýalar, Bekaplar); Tor (IP salgylar, Firewall); Maliýe (Balans, Amallar taryhy); DNS; aşakda Admin paneli we Çykmak. Ýokarky zolakda ýol (breadcrumb), balans (+ düwmesi bilen), dil saýlamak (TM/RU/EN) we ulanyjy menýusy durýar.
- **Maşynlar tablisasy:** Ady · Obraz · IP salgy (göçür düwmesi bilen) · Konfigurasiýa · Ýagdaýy · Döredildi · hereketler. Gözleg, ýagdaý boýunça filtr we sahypalama bar. Sag aşakda "+" FAB düwmesi.
- **Ýagdaý nyşanlary:** IŞJEŇ (ýaşyl), DURDURYLAN (çal), SAKLANAN (mämişi), DÖREDILÝÄR (gök).
- **Täze maşyn:** 1) obraz, 2) konfigurasiýa kartoçkalary (aýlyk baha uly, sagatlyk baha kiçi), 3) SSH açar ýa-da root parol, 4) hostname (`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`). Sagda "Jemi" paneli; balans ýetmese düwme ýapyk bolýar.
- **Maşyn sahypasy:** power düwmeleri, `ssh root@IP` gutusy, maglumat, ulanyş zolaklary, OS-i täzeden gurmak, paroly täzelemek we pozmak (tassyklama bilen).
- **Dil:** UI tekstleriniň hemmesi türkmen dilinde. `next-intl` bilen RU we EN goşmaga taýýar bolsun.
- **Elýeterlilik:** her bir input-yň `<label>`-y bolmaly. Diňe ikonly düwmelerde `aria-label` bolmaly. Kontrast azyndan 4.5:1. Telefon ekranynda menýu gizlenýär we düwme bilen açylýar.
- Maşynyň ýagdaýy üýtgäp durka (provisioning, reinstall), sahypa `/servers/:id/jobs`-y her 3 sekuntda soraýar.

## 10. Howpsuzlyk

- Parollar argon2id bilen heşlenýär. Tokenler DB-de diňe heş görnüşinde saklanýar.
- Proxmox-a `root@pam` bilen däl-de, aýratyn rol bilen döredilen API token (`panel@pve!api`) arkaly birikilýär. Token diňe worker-iň env-inde bolýar.
- Proxmox API (8006) diňe içki torda elýeterli bolmaly.
- Her endpoint serweriň `user_id`-ini barlaýar (IDOR-dan gorag).
- Her VM-de Proxmox firewall `ipfilter` we MAC filtri bilen açyk bolmaly. 25-nji port ilkibada ýapyk.
- Admin hereketleriniň hemmesi `audit_log`-a ýazylýar.
- CSRF: cookie + `SameSite=Lax` bolup, üýtgeşme girizýän soraglarda `X-CSRF-Token` hem barlanýar.
- Gizlin maglumatlar diňe `.env` faýlynda saklanýar, olar repo-da bolmaly däl.

## 11. Tapgyrlar

1. Esas: monorepo, docker-compose, migrasiýalar we seed (tarifler), auth, Next.js giriş we baş sahypa.
2. Proxmox adapteri: clone, config, resize, start/stop, delete we task garaşmak; test node bilen integrasiýa testi.
3. Maşyn lifecycle: IP pool, node saýlamak, maşynyň ähli amallary we sahypalary.
4. Billing: ledger, sagatlaýyn cron, aýlyk çäk, suspend/unsuspend, emailler, balans sahypasy.
5. Admin paneli: ulanyjylar, balansy doldurmak, tarifler, obrazlar, node-lar, IP bloklary, audit.
6. Taýýarlyk: rate limit, monitoring (Prometheus + Grafana), DB backup, Telegram duýduryşlary.

**MVP-den soň:** bank töleg şlýuzy, noVNC konsol, snapshot/backup, firewall paneli, DNS, ulanyjy API tokenleri.

## 12. Kabul ediş şertleri

- Täze ulanyjy hasaba alynýar, emailini tassyklaýar we girýär.
- Admin balans goşandan soň ulanyjy VPS C-2020 maşynyny döredýär. Maşyn bir minudyň dowamynda `running` bolýar we `ssh root@IP` işleýär.
- Bir sagatdan soň balansdan `round(29750 × 1 / 720)` = 41 teňňe aýrylýar we `transactions`-da görünýär.
- Balans 0-dan aşak düşende maşyn durdurylýar. Balans doldurylanda awtomatik başlaýar.
- Bir ulanyjy başga ulanyjynyň maşynyny görüp ýa-da dolandyryp bilmeýär (`404`).
- Go üçin `go test ./...` we frontend üçin `npm run lint && npm run build` ýalňyşsyz geçýär.
