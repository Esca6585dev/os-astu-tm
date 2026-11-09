#!/usr/bin/env bash
# Doly synag: API-ni HYPERVISOR=fake RUN_WORKER=true DEV_AUTO_VERIFY=true bilen işledip, şu skripti işlediň.
#   API=http://localhost:8080 PSQL="psql postgres://vps:vps@localhost:5432/vps" ./scripts/smoke.sh
set -euo pipefail
API=${API:-http://localhost:8080}
PSQL=${PSQL:-psql postgres://vps:vps@localhost:5432/vps}
ADMIN_EMAIL=${ADMIN_EMAIL:-admin@astu.tm}
ADMIN_PASSWORD=${ADMIN_PASSWORD:-admin-parol-123}
TMP=$(mktemp -d); trap 'rm -rf "$TMP"' EXIT
pass=0
ok()   { pass=$((pass+1)); echo "  ✓ $*"; }
fail() { echo "  ✗ $*"; exit 1; }
# req JAR METHOD PATH [JSON] → body in $TMP/body, code in $code
req() {
  local jar=$1 m=$2 p=$3 d=${4:-}
  code=$(curl -s -o "$TMP/body" -w '%{http_code}' -b "$TMP/$jar" -c "$TMP/$jar" -X "$m" \
    -H 'Content-Type: application/json' -H 'X-Requested-With: fetch' ${d:+--data "$d"} "$API/api/v1$p")
}
body() { cat "$TMP/body"; }
expect() { [ "$code" = "$1" ] || fail "$2 → HTTP $code: $(body)"; ok "$2"; }
jq_() { python3 -c "import json,sys; d=json.load(open('$TMP/body')); print($1)"; }
wait_status() { # id status
  for _ in $(seq 1 40); do
    req user GET "/servers/$1"; s=$(jq_ "d['status']"); b=$(jq_ "d['busy']")
    [ "$s" = "$2" ] && [ "$b" = "False" ] && return 0; sleep 1
  done; fail "server $1 status=$s busy=$b, garaşylýan $2"
}

echo "== Admin sazlamalary"
req admin POST /auth/login "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}"; expect 200 "admin login"
req admin POST /admin/nodes '{"name":"pve1","storage":"local-lvm","bridge":"vmbr0","max_vcpu":64,"max_ram_mb":131072,"max_disk_gb":4000,"is_active":true}'; expect 200 "node goşmak"
NODE=$(jq_ "d['id']")
req admin POST /admin/ip-pools "{\"node_id\":$NODE,\"cidr\":\"203.0.113.0/28\",\"gateway\":\"203.0.113.1\"}"; expect 201 "IP blok (/28)"
[ "$(jq_ "d['addresses']")" = "13" ] && ok "13 IP döredildi (tor, broadcast, şlýuz aýryldy)" || fail "IP sany $(body)"
req admin POST /admin/images '{"slug":"ubuntu-24_04","name":"Ubuntu","version":"24.04 LTS","proxmox_template_vmid":9000,"is_active":true}'; expect 200 "obraz goşmak"
IMG=$(jq_ "d['id']")

echo "== Ulanyjy"
EMAIL="test$RANDOM@mysal.tm"
req user POST /auth/register "{\"email\":\"$EMAIL\",\"password\":\"ulanyjy-parol-1\",\"full_name\":\"Test Ulanyjy\"}"; expect 201 "hasaba alyş"
req user POST /auth/login "{\"email\":\"$EMAIL\",\"password\":\"ulanyjy-parol-1\"}"; expect 200 "login"
req user GET /me; expect 200 "/me"; UID_=$(jq_ "d['id']")
curl -s -o /dev/null -w '%{http_code}' -b "$TMP/user" -X POST "$API/api/v1/ssh-keys" -H 'Content-Type: application/json' --data '{}' | grep -q 403 && ok "CSRF: X-Requested-With bolmasa 403"
req user GET /plans; expect 200 "tarifler"; PLAN=$(jq_ "[p['id'] for p in d if p['code']=='VPS C-2020'][0]")
req user POST /servers "{\"plan_id\":$PLAN,\"image_id\":$IMG,\"hostname\":\"web-01\"}"; expect 402 "balans ýok → 402"

req admin POST /admin/users/$UID_/topup '{"amount":500,"note":"çek №1"}'; expect 200 "admin balans +500 TMT"
req user POST /ssh-keys '{"name":"noutbuk","public_key":"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGvXk2ZJr8q3vH9cK2b5c3XzQ8n1QpZ6YzJp0aXxqv2B a@b"}'; expect 201 "SSH açar"
KEY=$(jq_ "d['id']")
req user POST /servers "{\"plan_id\":$PLAN,\"image_id\":$IMG,\"hostname\":\"Bad_Name\"}"; expect 422 "nädogry hostname → 422"
req user POST /servers "{\"plan_id\":$PLAN,\"image_id\":$IMG,\"hostname\":\"web-01\",\"ssh_key_ids\":[$KEY]}"; expect 202 "maşyn döretmek"
SID=$(jq_ "d['id']")
wait_status "$SID" running && ok "maşyn running"
req user GET "/servers/$SID"; [ "$(jq_ "d['ip']")" = "203.0.113.2" ] && ok "IP berildi: 203.0.113.2" || fail "IP $(body)"
[ "$(jq_ "d['usage']['status']")" = "running" ] && ok "Proxmox ulanyşy gelýär"

echo "== Amallar"
req user POST "/servers/$SID/stop"; expect 202 "durdurmak"
req user POST "/servers/$SID/start"; expect 409 "busy wagtynda başga amal → 409"
wait_status "$SID" stopped && ok "stopped"
req user POST "/servers/$SID/start"; expect 202 "başlatmak"; wait_status "$SID" running && ok "running"
req user POST "/servers/$SID/reset-password"; expect 202 "root parol"; PW=$(jq_ "d['root_password']"); [ ${#PW} -eq 16 ] && ok "täze parol 16 simwol"
wait_status "$SID" running
req user POST "/servers/$SID/reinstall" "{\"image_id\":$IMG}"; expect 202 "OS täzeden gurmak"; wait_status "$SID" running && ok "täzeden guruldy"

echo "== Başga ulanyjy (IDOR)"
E2="other$RANDOM@mysal.tm"
req other POST /auth/register "{\"email\":\"$E2\",\"password\":\"ulanyjy-parol-2\"}"; req other POST /auth/login "{\"email\":\"$E2\",\"password\":\"ulanyjy-parol-2\"}"
req other GET "/servers/$SID"; expect 404 "başga ulanyjynyň maşyny görünmeýär"
req other POST "/servers/$SID/stop"; expect 404 "başga ulanyjynyň maşynyny durdurup bolmaýar"

echo "== Billing (sagatlar yza süýşürilýär, worker her minutda hasaplaýar)"
$PSQL -qtAc "UPDATE servers SET created_at = date_trunc('hour', now()) - interval '3 hours' WHERE id=$SID" >/dev/null
for _ in $(seq 1 75); do
  n=$($PSQL -qtAc "SELECT count(*) FROM server_usage WHERE server_id=$SID"); [ "$n" -ge 3 ] && break; sleep 1
done
[ "$n" -ge 3 ] && ok "3 sagat hasaplandy" || fail "server_usage=$n"
req user GET /me/balance; BAL=$(jq_ "d['balance']")
[ "$BAL" = "49876" ] && ok "balans 500.00 − round(297.50×3/720) = 498.76 TMT" || fail "balans $BAL (garaşylýan 49876)"

echo "== Suspend / unsuspend"
$PSQL -qtAc "UPDATE users SET balance=-100 WHERE id=$UID_" >/dev/null
for _ in $(seq 1 75); do req user GET "/servers/$SID"; [ "$(jq_ "d['status']")" = "suspended" ] && break; sleep 1; done
[ "$(jq_ "d['status']")" = "suspended" ] && ok "balans < 0 → suspended" || fail "status $(jq_ "d['status']")"
req user POST "/servers/$SID/start"; expect 402 "saklanan maşyny başladyp bolmaýar"
req admin POST /admin/users/$UID_/topup '{"amount":100,"note":"çek №2"}'; expect 200 "balans doldurmak"
for _ in $(seq 1 75); do req user GET "/servers/$SID"; [ "$(jq_ "d['status']")" = "running" ] && break; sleep 1; done
[ "$(jq_ "d['status']")" = "running" ] && ok "balans > 0 → awtomatik running" || fail "status $(jq_ "d['status']")"

echo "== Pozmak"
wait_status "$SID" running
req user DELETE "/servers/$SID"; expect 202 "pozmak"
for _ in $(seq 1 30); do s=$($PSQL -qtAc "SELECT status FROM servers WHERE id=$SID"); [ "$s" = "deleted" ] && break; sleep 1; done
[ "$s" = "deleted" ] && ok "deleted" || fail "status $s"
[ "$($PSQL -qtAc "SELECT count(*) FROM ip_addresses WHERE address='203.0.113.2' AND server_id IS NULL")" = "1" ] && ok "IP boşadyldy"
req admin GET /admin/audit; [ "$(jq_ "len(d)")" -gt 5 ] && ok "audit žurnaly ýazylýar"

echo; echo "Ähli $pass barlag geçdi."
