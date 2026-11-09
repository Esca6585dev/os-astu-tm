# Proxmox VE-ni os.astu.tm paneli bilen sazlamak

Bu gollanma Proxmox serwerini taýýarlamagy we paneli oňa birikdirmegi ädimme-ädim düşündirýär. Ähli buýruklar Proxmox node-unda `root` hökmünde işledilýär.

```text
Ulanyjy ──HTTPS──> nginx ──> web (Next.js) / api (Go)
                                     │
                              worker (Go) ──HTTPS 8006, API token──> Proxmox VE
                                     │                                    │
                                PostgreSQL                         VM-ler (cloud-init)
Ulanyjy ─────────────────────── SSH (22) ────────────────────────────────┘
```

## 1. Proxmox VE-ni gurnamak

1. [Proxmox VE 8 ýa-da 9](https://www.proxmox.com/en/downloads) ISO-dan gurnaň.
2. Storage: `local-lvm` (LVM-thin) ýa-da ZFS ulanyp bolýar. Paneliň node sazlamasyndaky `storage` meýdany şunuň ady bilen birmeňzeş bolmaly.
3. Ulgamy täzeläň: `apt update && apt full-upgrade -y`.

## 2. Tor: jemgyýetçilik IP-ler üçin bridge

Maşynlar jemgyýetçilik IP salgylaryny göni `vmbr0` bridge-den alýar. Telekomdan alnan IP blogy node-yň fiziki portuna gelmeli.

`/etc/network/interfaces` mysaly (IP-ler mysal üçin):

```text
auto lo
iface lo inet loopback

iface eno1 inet manual

auto vmbr0
iface vmbr0 inet static
    address 216.250.11.10/24      # node-yň öz IP-si
    gateway 216.250.11.1          # Telekomuň şlýuzy
    bridge-ports eno1
    bridge-stp off
    bridge-fd 0
```

`ifreload -a` bilen işjeňleşdiriň. Şol /24 blokdaky beýleki salgylar (node-yň we şlýuzyň salgysyndan başgalary) panelde IP blok hökmünde goşulýar.

> Proxmox API-ni (8006-njy port) internete açmaň. Ony diňe panel serweriniň içki IP-sine açyň (Datacenter → Firewall ýa-da daşky firewall).

## 3. Firewall

Panel her VM-e IP/MAC filtrini we çykýan 25-nji port (SMTP) bloklaýan düzgüni goýýar (`PVE_FIREWALL=true`). Bu düzgünler diňe Datacenter firewall açyk bolanda işleýär:

1. Datacenter → Firewall → **Options**: `Firewall = Yes`.
2. Öňünden **8006 we 22 portlaryna admin IP-lerden** rugsat beriji düzgün goşuň, ýogsa öz-özüňizi bloklarsyňyz.
3. Node → Firewall → Options-da hem firewall açyk bolmaly.

## 4. Cloud-init şablonlary (obrazlar)

Her obraz Proxmox-da bir şablon VM bolýar. Panel şony doly klonlaýar, cloud-init bilen IP-ni, root parolyny we SSH açarlary goýýar, diski tarife çenli ulaldýar.

Şablonda **qemu-guest-agent** bolmaly (panel root parolyny şonuň üsti bilen çalyşýar) we **root bilen SSH girişi** açyk bolmaly.

```bash
apt install -y libguestfs-tools
cd /var/lib/vz/template/iso

# Ubuntu 24.04
wget https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img
virt-customize -a noble-server-cloudimg-amd64.img \
  --install qemu-guest-agent \
  --run-command 'systemctl enable qemu-guest-agent' \
  --write '/etc/cloud/cloud.cfg.d/99-panel.cfg:disable_root: false
ssh_pwauth: true
' \
  --write '/etc/ssh/sshd_config.d/99-panel.conf:PermitRootLogin yes
PasswordAuthentication yes
' \
  --truncate /etc/machine-id

VMID=9000
qm create $VMID --name tpl-ubuntu-2404 --memory 1024 --cores 1 --cpu host \
  --net0 virtio,bridge=vmbr0 --scsihw virtio-scsi-pci --ostype l26
qm set $VMID --scsi0 local-lvm:0,import-from=/var/lib/vz/template/iso/noble-server-cloudimg-amd64.img
qm set $VMID --ide2 local-lvm:cloudinit --boot order=scsi0 --serial0 socket --vga serial0 --agent enabled=1
qm template $VMID
```

Debian 12 üçin şol ädimleri `debian-12-genericcloud-amd64.qcow2` faýly (https://cloud.debian.org/images/cloud/bookworm/latest/) we `VMID=9001` bilen gaýtalaň.

Bellikler:
- Esasy disk `scsi0` bolmaly. Başga ady ulansaňyz, `.env`-de `PVE_DISK` sazlamasyny üýtgediň.
- Şablonyň diski iň kiçi tarifiň diskinden (20 GB) **kiçi** bolmaly. Cloud şekilleri 2–3 GB bolýar, ýagny bu şert ýerine ýetýär. Disk diňe ulaldylýar.
- **Bir node:** şablonlar `local-lvm`-de durup biler.
- **Klaster (birnäçe node):** VMID klasterde gaýtalanmaýar. Şablonlary paýlaşylýan storage-da (Ceph, NFS) saklaň. Panel şablonyň node-unu özi tapýar we klony saýlanan node-a (`target`) iberýär.

## 5. Panel üçin API ulanyjysy, roly we tokeni

`root@pam` ulanmaň. Diňe gerekli hukuklary bolan aýratyn ulanyjy dörediň.

**Proxmox VE 8:**

```bash
pveum role add PanelVM -privs "VM.Allocate VM.Audit VM.Clone VM.Config.CDROM VM.Config.CPU VM.Config.Cloudinit VM.Config.Disk VM.Config.HWType VM.Config.Memory VM.Config.Network VM.Config.Options VM.PowerMgmt VM.Monitor Datastore.AllocateSpace Datastore.Audit SDN.Use Sys.Audit"
```

**Proxmox VE 9** (`VM.Monitor` aýryldy, guest-agent arkaly parol çalyşmak üçin `VM.GuestAgent.Unrestricted` gerek):

```bash
pveum role add PanelVM -privs "VM.Allocate VM.Audit VM.Clone VM.Config.CDROM VM.Config.CPU VM.Config.Cloudinit VM.Config.Disk VM.Config.HWType VM.Config.Memory VM.Config.Network VM.Config.Options VM.PowerMgmt VM.GuestAgent.Unrestricted Datastore.AllocateSpace Datastore.Audit SDN.Use Sys.Audit"
```

Soňra:

```bash
pveum user add panel@pve --comment "os.astu.tm paneli"
pveum acl modify / --users panel@pve --roles PanelVM
pveum user token add panel@pve api --privsep 0
```

Soňky buýruk `full-tokenid` (`panel@pve!api`) we `value` (gizlin açar) çykarýar. Gizlin açar diňe bir gezek görkezilýär, ony `.env`-e ýazyň.

**TLS:** Proxmox-yň CA sertifikatyny panel serwerine göçüriň:

```bash
scp root@PVE:/etc/pve/pve-root-ca.pem deploy/certs/pve-root-ca.pem
```

Sertifikatyň içinde node-yň IP-si ýa-da ady bolmaly. `PVE_URL`-da şol ady ulanyň. Bu mümkin bolmasa, diňe içki torda `PVE_INSECURE_TLS=true` goýup bilersiňiz.

## 6. Paneli işletmek

```bash
git clone https://github.com/Esca6585dev/os-astu-tm && cd os-astu-tm/deploy
cp .env.example .env        # PVE_URL, PVE_TOKEN_ID, PVE_TOKEN_SECRET, parollar
mkdir -p certs && cp /ýol/pve-root-ca.pem certs/
docker compose up -d --build
docker compose logs -f api worker
```

nginx üçin `deploy/nginx.conf` faýlyny `/etc/nginx/sites-available/` papkasyna göçüriň, `certbot --nginx -d os.astu.tm` buýrugyny işlediň we `nginx -s reload` ediň.

## 7. Paneliň içinde infrastrukturany goşmak

`ADMIN_EMAIL` / `ADMIN_PASSWORD` bilen giriň we **Admin paneli** sahypasyna geçiň:

1. **Node-lar:** ady Proxmox-daky bilen birmeňzeş bolmaly (`pve1`). Storage `local-lvm`, bridge `vmbr0`. Maşynlara paýlap boljak iň köp CPU, RAM we disk mukdaryny ýazyň (overcommit edip bilersiňiz, mysal üçin CPU × 2).
2. **IP bloklary:** node, CIDR (`216.250.11.0/24`) we şlýuz (`216.250.11.1`) giriziň. Node-yň öz IP-sini we beýleki ulanylýan salgylary **Bron** meýdanyna ýazyň ýa-da aralygy (başy we soňy) görkeziň.
3. **Obrazlar:** `Ubuntu`, `24.04 LTS`, kod `ubuntu-24_04`, şablon VMID `9000`.
4. **Tarifler:** VPS A–E-2020 tarifleri eýýäm goşulan. Bahany şu ýerde üýtgedip bolýar.

## 8. Barlag

1. Synag ulanyjysyny hasaba alyň, admin panelden oňa balans goşuň.
2. Maşyn dörediň. Ýagdaýy 1–3 minutda **IŞJEŇ** bolmaly, Proxmox-da täze VM peýda bolar.
3. `ssh root@IP` bilen giriň.
4. Ýalňyşlyk bolsa: `docker compose logs worker` buýrugy bilen loglary we admin paneldäki **Maşynlar** sanawyndaky ýalňyşlyk tekstini görüň.

| Ýalňyşlyk | Sebäbi |
|---|---|
| `401 ... authentication failure` | Token ID ýa-da gizlin açar nädogry |
| `403 Permission check failed` | Rolda hukuk ýetmeýär (5-nji ädim), ýa-da ACL `/`-e berilmedik |
| `şablon VM 9000 tapylmady` | Obrazyň VMID-i nädogry ýa-da şablon ýok |
| `x509: certificate` | `PVE_CA_FILE` ýok ýa-da `PVE_URL`-daky at sertifikatdaky bilen gabat gelmeýär |
| `qemu-guest-agent ... not running` | Şablonda guest agent ýok (4-nji ädim), parol çalşyp bolmaýar |
| `storage ... does not exist` | Node sazlamasyndaky storage ady Proxmox-daky bilen deň däl |
| Maşyn işleýär, ýöne tor ýok | Datacenter firewall-da `ipfilter` açyk, ýöne IP blogy dogry bridge-de däl, ýa-da Telekom blogy node-a gönükdirilmedik |

## 9. Proxmox-syz synag

Proxmox serweri entek ýok bolsa:

```bash
HYPERVISOR=fake RUN_WORKER=true DEV_AUTO_VERIFY=true ADMIN_EMAIL=admin@astu.tm ADMIN_PASSWORD=admin-parol-123 \
DATABASE_URL=postgres://vps:vps@localhost:5432/vps?sslmode=disable go run ./cmd/server
```

Bu režimde maşynlar diňe ýatda "döredilýär". Ähli prosesi `scripts/smoke.sh` bilen barlap bolýar.
