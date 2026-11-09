# os.astu.tm — VPS hosting paneli

os.telecom.tm ýaly VPS hosting paneli. Ulanyjy hasaba alynýar, balansyny görýär, tarif saýlap maşyn (VPS) açýar we oňa SSH bilen girýär. Töleg her sagat balansdan aýrylýar.

## Repo-da näme bar

| Faýl / papka | Näme |
|---|---|
| `PROMPT.md` | Doly ulgamy gurmak üçin doly prompt: Go Fiber + Next.js + PostgreSQL + Proxmox, maglumat bazasy, API, billing, howpsuzlyk, tapgyrlar |
| `test/` | Statik test wersiýasy (HTML + CSS + JS). Diňe dizaýny görmek üçin, backend ýok |

## Test wersiýasyny açmak

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
