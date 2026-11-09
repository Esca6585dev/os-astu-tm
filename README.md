# os.astu.tm — VPS hosting paneli
https://claude.ai/artifact/XyUSg9BARVDudQtxgKdAVj#artboard-64a8125801c3
os.telecom.tm ýaly VPS hosting paneli. Ulanyjy hasaba alynýar, balansyny görýär, tarif saýlap maşyn (VPS) açýar we oňa SSH bilen girýär. Töleg her sagat balansdan aýrylýar.

## Repo-da näme bar

| Papka / faýl | Näme |
|---|---|
| `api/` | Go 1.23+ · Fiber · PostgreSQL: HTTP API (`cmd/server`) we worker (`cmd/worker`): Proxmox amallary we sagatlaýyn billing |
| `web/` | Next.js 15 paneli: ulanyjy we admin sahypalary |
| `deploy/` | `docker-compose.yml`, `.env.example`, `nginx.conf` |
| `docs/PROXMOX.md` | **Proxmox serwerini taýýarlamak we paneli birikdirmek: ädimme-ädim** |
| `scripts/smoke.sh` | Doly synag (39 barlag): hasaba alyş → maşyn → amallar → billing → suspend → pozmak |
| `test/` | Ilkinji statik dizaýn nusgasy (HTML/CSS/JS) |
| `PROMPT.md` | Taslamanyň doly tehniki tabşyrygy |

## Çalt başlamak

**Production (Proxmox bilen):** [docs/PROXMOX.md](docs/PROXMOX.md) faýlyndaky ädimleri ýerine ýetiriň, soňra:

```bash
cd deploy && cp .env.example .env   # PVE_URL, PVE_TOKEN_ID, PVE_TOKEN_SECRET, parollar
docker compose up -d --build
```

**Ýerli synag (Proxmox-syz):**

```bash
# PostgreSQL işläp durmaly
cd api && HYPERVISOR=fake RUN_WORKER=true DEV_AUTO_VERIFY=true   ADMIN_EMAIL=admin@astu.tm ADMIN_PASSWORD=admin-parol-123   DATABASE_URL=postgres://vps:vps@localhost:5432/vps?sslmode=disable go run ./cmd/server
cd web && npm install && npm run dev          # http://localhost:3000
./scripts/smoke.sh                            # doly synag
```

## Arhitektura gysgaça

- **Billing:** ähli pul teňňede (`BIGINT`). Her sagat `round(aýlyk × n / 720) − tutulan` tutulýar, ýagny aýda aýlyk bahadan köp tutulmaýar. `server_usage (server_id, hour)` açary bir sagadyň iki gezek tutulmagynyň öňüni alýar.
- **Job navbaty:** PostgreSQL-de (`jobs` tablisasy, `FOR UPDATE SKIP LOCKED`), Redis gerek däl. Bir maşynda bir wagtda diňe bir amal ýerine ýetirilýär.
- **Proxmox:** API token; şablony doly klonlamak, cloud-init (IP, root parol, SSH açar), diski ulaltmak, tor tizligi (`rate`), VM firewall (ipfilter, 25-nji port ýapyk), parol qemu-guest-agent arkaly.
- **Balans:** 0-dan aşak düşende maşynlar saklanýar, doldurylanda awtomatik başlaýar, 7 günden soň pozulýar. Email duýduryşlary iberilýär.
- **Howpsuzlyk:** argon2id, httpOnly sessiýa cookie, CSRF (`X-Requested-With`), rate limit, IDOR barlaglary we audit žurnaly.

## Statik dizaýn nusgasy (test/)

`test/index.html` faýlyny brauzerde açyň. Ýa-da ýerli serwer bilen işlediň:

```bash
cd test
python3 -m http.server 8080
# http://localhost:8080
```

Sahypalar:

| Salgy | Sahypa |
|---|---|
| `#/login` | Giriş we hasaba alyş |
| `#/dashboard` | Baş sahypa |
| `#/machines` | Maşynlar |
| `#/new` | Täze maşyn döretmek |
| `#/machine/1` | Maşynyň jikme-jikligi |
| `#/keys` | SSH açarlar |
| `#/plans` | Konfigurasiýalar (tarifler) |
| `#/billing` | Balans we amallar taryhy |
| `#/admin` | Admin paneli (balansy doldurmak) |

Test wersiýasynda maşyn döretmek, durdurmak, pozmak, SSH açar goşmak we admin tarapyndan balans doldurmak işleýär. Ähli maglumatlar `test/js/data.js` faýlynda durýar we sahypa täzelenende başdaky ýagdaýyna gaýdýar.

## Tarifler

| Kod | Aýda (TMT) | ≈ Sagatda | CPU | RAM | HDD | Tizlik |
|---|---|---|---|---|---|---|
| VPS A-2020 | 140.00 | 0.19 | 1 | 1 GB | 20 GB | 8 Mbit/s |
| VPS B-2020 | 175.00 | 0.24 | 1 | 1 GB | 40 GB | 8 Mbit/s |
| VPS C-2020 | 297.50 | 0.41 | 2 | 2 GB | 60 GB | 12 Mbit/s |
| VPS D-2020 | 420.00 | 0.58 | 4 | 4 GB | 80 GB | 24 Mbit/s |
| VPS E-2020 | 595.00 | 0.83 | 6 | 6 GB | 120 GB | 32 Mbit/s |

Her tarifde 1 IP bar. Sagatlaýyn baha = aýlyk baha / 720. Bir aýda aýlyk bahadan köp tutulmaýar.

## Reňkler

| Ady | Reňk |
|---|---|
| Esasy | `#196CFC` |
| Gyzgyn (sidebar, hover) | `#1B44C8` |
| Açyk aksent | `#3E9BFF` |
| Ýumşak fon | `#EEF5FF` |
