// os.astu.tm — statik test wersiýasy. Backend ýok: ähli hereketler brauzerde ýerine ýetirilýär.
(function () {
  "use strict";

  const D = window.DATA;
  const $ = (sel, root = document) => root.querySelector(sel);
  const fmt = (n) => Number(n).toFixed(2);
  const money = (n) => fmt(n) + " TMT";
  const esc = (s) => String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  const planOf = (id) => D.plans.find((p) => p.id === id);
  const hourly = (p) => p.monthly / 720;
  const totalHourly = () => D.servers.filter((s) => s.status !== "suspended").reduce((a, s) => a + hourly(planOf(s.plan)), 0);
  const daysLeft = () => { const h = totalHourly(); return h > 0 ? Math.floor(D.balance / (h * 24)) : "∞"; };

  // ---------- Ikonlar ----------
  const svg = (d, cls = "icon", sw = 2) =>
    `<svg class="${cls}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="${sw}" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${d}</svg>`;
  const IC = {
    home: '<path d="M3 11l9-7 9 7"/><path d="M5 10v10h14V10"/><path d="M10 20v-6h4v6"/>',
    cpu: '<rect x="6" y="6" width="12" height="12" rx="2"/><path d="M9 2v4M15 2v4M9 18v4M15 18v4M2 9h4M2 15h4M18 9h4M18 15h4"/>',
    server: '<rect x="3" y="4" width="18" height="7" rx="2"/><rect x="3" y="13" width="18" height="7" rx="2"/><path d="M7 7.5h.01M7 16.5h.01"/>',
    key: '<circle cx="8" cy="15" r="4"/><path d="M10.8 12.2L20 3M16 7l3 3"/>',
    disk: '<ellipse cx="12" cy="6" rx="8" ry="3"/><path d="M4 6v12c0 1.7 3.6 3 8 3s8-1.3 8-3V6"/><path d="M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3"/>',
    sliders: '<path d="M4 6h10M18 6h2M4 12h4M12 12h8M4 18h12"/><circle cx="16" cy="6" r="2"/><circle cx="10" cy="12" r="2"/><circle cx="18" cy="18" r="2"/>',
    backup: '<path d="M3 12a9 9 0 1 0 3-6.7"/><path d="M3 4v5h5"/><path d="M12 8v4l3 2"/>',
    net: '<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3c2.5 2.7 3.8 5.7 3.8 9s-1.3 6.3-3.8 9c-2.5-2.7-3.8-5.7-3.8-9S9.5 5.7 12 3z"/>',
    shield: '<path d="M12 3l8 3v6c0 4.5-3.4 8-8 9-4.6-1-8-4.5-8-9V6z"/>',
    wallet: '<rect x="3" y="6" width="18" height="13" rx="2"/><path d="M3 10h18M16 15h2"/>',
    list: '<path d="M4 7h16M4 12h16M4 17h10"/>',
    chev: '<path d="M6 9l6 6 6-6"/>',
    plus: '<path d="M12 5v14M5 12h14"/>',
    refresh: '<path d="M20 12a8 8 0 1 1-2.3-5.6"/><path d="M20 4v5h-5"/>',
    copy: '<rect x="9" y="9" width="11" height="11" rx="2"/><path d="M5 15V5a1 1 0 0 1 1-1h9"/>',
    power: '<path d="M12 3v9"/><path d="M6.3 7.3a8 8 0 1 0 11.4 0"/>',
    terminal: '<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M7 9l3 3-3 3M13 15h4"/>',
    search: '<circle cx="11" cy="11" r="7"/><path d="M20 20l-3.5-3.5"/>',
    check: '<path d="M5 12l4.5 4.5L19 7"/>',
    trash: '<path d="M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3"/>',
    alert: '<path d="M12 3l9.5 17h-19z"/><path d="M12 10v4M12 17h.01"/>',
    info: '<circle cx="12" cy="12" r="9"/><path d="M12 11v5M12 8h.01"/>'
  };

  // ---------- Menýu ----------
  const NAV = [
    { route: "dashboard", label: "Baş sahypa", icon: "home" },
    { group: "Hasaplaýyş", icon: "cpu", open: true, items: [
      { route: "machines", label: "Maşynlar", icon: "server" },
      { route: "keys", label: "SSH açarlar", icon: "key" },
      { route: "images", label: "Obrazlar", icon: "disk" },
      { route: "plans", label: "Konfigurasiýalar", icon: "sliders" },
      { route: "backups", label: "Bekaplar", icon: "backup" }
    ]},
    { group: "Tor", icon: "net", items: [
      { route: "ips", label: "IP salgylar", icon: "net" },
      { route: "firewall", label: "Firewall", icon: "shield" }
    ]},
    { group: "Maliýe", icon: "wallet", items: [
      { route: "billing", label: "Balans", icon: "wallet" },
      { route: "billing", label: "Amallar taryhy", icon: "list" }
    ]},
    { route: "dns", label: "DNS", icon: "list" }
  ];

  function renderNav(active) {
    const link = (it, sub) =>
      `<a href="#/${it.route}" class="${sub ? "sub " : ""}${it.route === active ? "active" : ""}" ${it.route === active ? 'aria-current="page"' : ""}>${svg(IC[it.icon], sub ? "icon-sm" : "icon")}${it.label}</a>`;
    $("#nav").innerHTML = NAV.map((n) => {
      if (!n.group) return link(n, false);
      const open = n.open || n.items.some((i) => i.route === active);
      return `<details ${open ? "open" : ""}><summary><span style="display:flex;align-items:center;gap:12px">${svg(IC[n.icon])}${n.group}</span><span class="chev" style="display:flex">${svg(IC.chev, "icon-sm")}</span></summary><div class="group-items">${n.items.map((i) => link(i, true)).join("")}</div></details>`;
    }).join("");
    document.querySelectorAll(".sidebar-foot a[data-route]").forEach((a) => a.classList.toggle("active", a.dataset.route === active));
  }

  function crumbs(parts) {
    $("#crumbs").innerHTML = parts.map((p, i) =>
      i === parts.length - 1 ? `<span class="current">${esc(p)}</span>` : `<span>${esc(p)}</span><span class="sep" aria-hidden="true">/</span>`
    ).join("");
  }

  function toast(msg) {
    const t = $("#toast");
    t.textContent = msg;
    t.classList.add("show");
    clearTimeout(toast._t);
    toast._t = setTimeout(() => t.classList.remove("show"), 2200);
  }

  function copy(text) {
    (navigator.clipboard ? navigator.clipboard.writeText(text) : Promise.reject()).then(
      () => toast("Göçürildi: " + text),
      () => toast(text)
    );
  }

  const badge = (status) => ({
    active: '<span class="badge badge-ok">IŞJEŇ</span>',
    stopped: '<span class="badge badge-off">DURDURYLAN</span>',
    suspended: '<span class="badge badge-warn">SAKLANAN</span>',
    provisioning: '<span class="badge badge-info">DÖREDILÝÄR</span>',
    pending: '<span class="badge badge-off">TASSYKLANMADYK</span>'
  }[status] || status);

  const head = (title, sub, actions = "") =>
    `<div class="page-head"><div><h1>${title}</h1>${sub ? `<p>${sub}</p>` : ""}</div><div class="row-actions">${actions}</div></div>`;

  // ---------- Sahypalar ----------
  const views = {};

  views.dashboard = () => {
    crumbs(["Baş sahypa"]);
    const active = D.servers.filter((s) => s.status === "active").length;
    const cap = D.servers.reduce((a, s) => a + planOf(s.plan).monthly, 0);
    const used = D.servers.reduce((a, s) => a + s.monthUsed, 0);
    return `
      ${head("Hoş geldiňiz, Aman", "Maşynlaryňyzyň we balansyňyzyň gysgaça ýagdaýy",
        `<a class="btn" href="#/new">${svg(IC.plus, "icon-sm", 2.4)}Täze maşyn</a>`)}
      <section class="grid grid-stats" aria-label="Görkezijiler">
        <div class="card stat"><span class="label">Balans</span><span class="value">${money(D.balance)}</span><a href="#/billing">Doldurmak →</a></div>
        <div class="card stat"><span class="label">Işjeň maşynlar</span><span class="value">${active} <small>/ ${D.servers.length}</small></span><span class="muted">${D.servers.length - active} durdurylan</span></div>
        <div class="card stat"><span class="label">Häzirki tutum</span><span class="value">${fmt(totalHourly())} <small>TMT / sag</small></span><span class="muted">Aýlyk çäk ${money(cap)}</span></div>
        <div class="card stat"><span class="label">Şu aý tutulan</span><span class="value">${money(used)}</span><span class="muted">Balans ≈ ${daysLeft()} gün ýeter</span></div>
      </section>
      <div class="alert alert-warn">${svg(IC.alert)}<span>Balans 0 bolanda maşynlar awtomatik durdurylýar. 7 günden soň olar pozulýar.</span><a href="#/billing"><b>Balansy gör</b></a></div>
      <div class="split">
        <section class="card main-col">
          <div class="card-head"><h2>Maşynlarym</h2><a href="#/machines">Hemmesi →</a></div>
          <div class="table-wrap"><table>
            <thead><tr><th>Ady</th><th>IP salgy</th><th>Konfigurasiýa</th><th>Ýagdaýy</th></tr></thead>
            <tbody>${D.servers.slice(0, 4).map((s) => `<tr>
              <td><a href="#/machine/${s.id}"><b>${esc(s.name)}</b></a><div class="muted" style="font-size:12px">${esc(s.image)}</div></td>
              <td class="mono">${s.ip}</td><td>${planOf(s.plan).code}</td><td>${badge(s.status)}</td></tr>`).join("")}</tbody>
          </table></div>
        </section>
        <section class="card side-col">
          <div class="card-head"><h2>Soňky amallar</h2><a href="#/billing">Taryh →</a></div>
          ${D.transactions.slice(0, 4).map((t) => `<div style="display:flex;justify-content:space-between;gap:12px;padding:12px 20px;border-top:1px solid var(--border-soft)">
            <div><div style="font-weight:500">${esc(t.note)}</div><div class="muted" style="font-size:12px">${t.at}</div></div>
            <b style="color:${t.amount > 0 ? "var(--ok)" : "inherit"}">${t.amount > 0 ? "+" : "−"}${fmt(Math.abs(t.amount))}</b></div>`).join("")}
        </section>
      </div>`;
  };

  views.machines = () => {
    crumbs(["Hasaplaýyş", "Maşynlar"]);
    const active = D.servers.filter((s) => s.status === "active").length;
    return `
      ${head("Maşynlar", `${D.servers.length} maşyn · ${active} işjeň · jemi ${fmt(totalHourly())} TMT / sag`,
        `<button type="button" class="btn btn-ghost btn-icon" id="refresh" aria-label="Täzele">${svg(IC.refresh)}</button><a class="btn" href="#/new">${svg(IC.plus, "icon-sm", 2.4)}Täze maşyn</a>`)}
      <section class="card">
        <div class="toolbar">
          <label class="search">${svg(IC.search, "icon-sm")}<span class="sr-only">Gözleg</span><input type="search" id="q" placeholder="Ady ýa-da IP boýunça gözle"></label>
          <label style="display:flex;align-items:center;gap:8px">Ýagdaýy
            <select class="select" id="statusFilter"><option value="">Hemmesi</option><option value="active">Işjeň</option><option value="stopped">Durdurylan</option><option value="suspended">Saklanan</option></select>
          </label>
        </div>
        <div class="table-wrap"><table style="min-width:920px">
          <thead><tr><th>Ady</th><th>Obraz</th><th>IP salgy</th><th>Konfigurasiýa</th><th>Ýagdaýy</th><th>Döredildi</th><th><span class="sr-only">Hereketler</span></th></tr></thead>
          <tbody id="rows"></tbody>
        </table></div>
        <div class="pager">
          <label style="display:flex;align-items:center;gap:8px">Görkez <select class="select"><option>10</option><option>25</option><option>50</option></select></label>
          <span id="count"></span>
        </div>
      </section>
      <p class="muted" style="margin:0">Durdurylan maşyn hem tölegli: disk we IP salgy siz üçin saklanýar. Tutumy togtatmak üçin maşyny pozuň.</p>
      <a class="fab" href="#/new" aria-label="Täze maşyn döret">${svg(IC.plus, "icon", 2.4)}</a>`;
  };
  views.machines.mount = () => {
    const draw = () => {
      const q = $("#q").value.trim().toLowerCase();
      const st = $("#statusFilter").value;
      const list = D.servers.filter((s) => (!q || s.name.includes(q) || s.ip.includes(q)) && (!st || s.status === st));
      $("#rows").innerHTML = list.map((s) => `<tr>
        <td><a href="#/machine/${s.id}"><b>${esc(s.name)}</b></a></td>
        <td>${esc(s.image)}</td>
        <td><span style="display:inline-flex;align-items:center;gap:6px"><span class="mono">${s.ip}</span><button type="button" class="icon-btn" data-copy="${s.ip}" aria-label="${s.ip} göçür">${svg(IC.copy, "icon-sm")}</button></span></td>
        <td><a href="#/plans">${planOf(s.plan).code}</a></td>
        <td>${badge(s.status)}</td>
        <td class="muted" style="font-variant-numeric:tabular-nums">${s.created}</td>
        <td class="num"><a class="btn btn-ghost" style="min-height:34px" href="#/machine/${s.id}">Dolandyr</a></td></tr>`).join("") ||
        `<tr><td colspan="7" class="muted" style="text-align:center;padding:32px">Hiç zat tapylmady</td></tr>`;
      $("#count").textContent = `1–${list.length} / ${list.length}`;
    };
    $("#q").addEventListener("input", draw);
    $("#statusFilter").addEventListener("change", draw);
    $("#refresh").addEventListener("click", () => { draw(); toast("Sanaw täzelendi"); });
    draw();
  };

  views.machine = (id) => {
    const s = D.servers.find((x) => x.id === Number(id));
    if (!s) return views.notfound();
    const p = planOf(s.plan);
    crumbs(["Hasaplaýyş", "Maşynlar", s.name]);
    const meter = (label, val, pct) => `<div class="meter"><div class="top"><b>${label}</b><span class="muted">${val}</span></div><div class="bar"><span style="width:${pct}%"></span></div></div>`;
    const on = s.status === "active";
    return `
      <div class="page-head">
        <div>
          <a href="#/machines">← Maşynlar</a>
          <div style="display:flex;align-items:center;gap:12px;margin-top:6px"><h1>${esc(s.name)}</h1>${badge(s.status)}</div>
          <p>${esc(s.image)} · ${p.code} · ${p.cpu} CPU · ${p.ram} GB RAM · ${p.disk} GB HDD · ${p.speed} Mbit/s</p>
        </div>
        <div class="row-actions">
          <button type="button" class="btn btn-ghost" id="power" ${s.status === "provisioning" ? "disabled" : ""}>${svg(IC.power, "icon-sm")}${on ? "Durdur" : "Başlat"}</button>
          <button type="button" class="btn btn-ghost" id="reboot">${svg(IC.refresh, "icon-sm")}Täzeden ýükle</button>
          <button type="button" class="btn btn-ghost" id="console">${svg(IC.terminal, "icon-sm")}Konsol</button>
        </div>
      </div>
      <section class="ssh-box">
        <div><div class="label">SSH bilen birikmek</div><code>ssh root@${s.ip}</code></div>
        <button type="button" class="btn btn-ghost" data-copy="ssh root@${s.ip}">${svg(IC.copy, "icon-sm")}Göçür</button>
      </section>
      <div class="split">
        <section class="card card-pad main-col" style="flex:1 1 360px">
          <h2 class="step-title" style="margin-bottom:16px">Maglumat</h2>
          <dl class="kv">
            <dt>IPv4</dt><dd class="mono">${s.ip}</dd>
            <dt>Şlýuz</dt><dd class="mono">203.0.113.1</dd>
            <dt>Ulanyjy</dt><dd>root</dd>
            <dt>Node</dt><dd>${s.node}</dd>
            <dt>Döredildi</dt><dd>${s.created}</dd>
            <dt>Baha</dt><dd>${money(p.monthly)} / aý · ≈ ${fmt(hourly(p))} TMT / sag</dd>
            <dt>Şu aý tutulan</dt><dd>${money(s.monthUsed)}</dd>
          </dl>
        </section>
        <section class="card card-pad side-col" style="flex:1 1 360px;display:flex;flex-direction:column;gap:18px">
          <h2 class="step-title">Häzirki ulanyş</h2>
          ${meter("CPU", on ? "18%" : "0%", on ? 18 : 0)}
          ${meter("RAM", on ? `${(p.ram * 0.65).toFixed(1)} / ${p.ram} GB` : `0 / ${p.ram} GB`, on ? 65 : 0)}
          ${meter("Disk", `${Math.round(p.disk * 0.35)} / ${p.disk} GB`, 35)}
          ${meter("Tor tizligi", on ? `${(p.speed * 0.26).toFixed(1)} / ${p.speed} Mbit/s` : `0 / ${p.speed} Mbit/s`, on ? 26 : 0)}
        </section>
      </div>
      <div class="split">
        <section class="card card-pad" style="flex:1 1 360px;display:flex;flex-direction:column;gap:14px">
          <h2 class="step-title">OS-i täzeden gurmak</h2>
          <p class="muted" style="margin:0">Diskdäki ähli maglumatlar pozular. IP salgy üýtgemeýär.</p>
          <label class="field">Obraz<select class="select" id="reimg">${D.images.map((i) => `<option value="${i.slug}">${i.name} ${i.version}</option>`).join("")}</select></label>
          <button type="button" class="btn btn-ghost" id="reinstall" style="align-self:flex-start">Täzeden gur</button>
        </section>
        <section class="card card-pad" style="flex:1 1 360px;display:flex;flex-direction:column;gap:14px">
          <h2 class="step-title">Root paroly täzele</h2>
          <p class="muted" style="margin:0">Täze parol döredilip, diňe bir gezek görkeziler.</p>
          <button type="button" class="btn btn-ghost" id="resetpw" style="align-self:flex-start">Täze parol döret</button>
          <div id="pwOut"></div>
          <hr style="border:0;border-top:1px solid var(--border-soft);width:100%;margin:4px 0">
          <h2 class="step-title" style="color:var(--danger)">Maşyny pozmak</h2>
          <p class="muted" style="margin:0">Maşyn we diski yzyna gaýtarylmazdan pozulýar. Tutum şu sagatdan soň togtaýar.</p>
          <button type="button" class="btn btn-danger" id="del" style="align-self:flex-start">${svg(IC.trash, "icon-sm")}Maşyny poz</button>
        </section>
      </div>`;
  };
  views.machine.mount = (id) => {
    const s = D.servers.find((x) => x.id === Number(id));
    if (!s) return;
    $("#power") && $("#power").addEventListener("click", () => {
      s.status = s.status === "active" ? "stopped" : "active";
      toast(s.status === "active" ? "Maşyn başladyldy" : "Maşyn durduryldy");
      route();
    });
    $("#reboot").addEventListener("click", () => toast("Maşyn täzeden ýüklenýär…"));
    $("#console").addEventListener("click", () => toast("noVNC konsoly hakyky ulgamda açylar"));
    $("#reinstall").addEventListener("click", () => {
      s.image = $("#reimg").value;
      toast("OS täzeden gurulýar: " + s.image);
      route();
    });
    $("#resetpw").addEventListener("click", () => {
      const chars = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789";
      const arr = new Uint32Array(16);
      crypto.getRandomValues(arr);
      const pw = Array.from(arr, (n) => chars[n % chars.length]).join("");
      $("#pwOut").innerHTML = `<div class="alert alert-info"><span>Täze root parol: <b class="mono">${pw}</b></span><button type="button" class="btn btn-ghost" data-copy="${pw}">${svg(IC.copy, "icon-sm")}Göçür</button></div>`;
    });
    $("#del").addEventListener("click", () => {
      const ok = window.confirm(`"${s.name}" pozulsynmy? Bu hereketi yzyna gaýtaryp bolmaýar.`);
      if (!ok) return;
      D.servers = D.servers.filter((x) => x !== s);
      toast("Maşyn pozuldy");
      location.hash = "#/machines";
    });
  };

  // Täze maşyn
  const draft = { image: "u24", plan: "c", auth: "key", keys: [1], hostname: "ubuntu-2gb-01", password: "" };
  views.new = () => {
    crumbs(["Hasaplaýyş", "Maşynlar", "Täze maşyn"]);
    return `
      <div class="split">
        <div class="main-col" style="display:flex;flex-direction:column;gap:20px">
          ${head("Täze maşyn döret", "Obraz, konfigurasiýa we giriş usulyny saýlaň")}
          <section class="card card-pad" style="display:flex;flex-direction:column;gap:16px">
            <h2 class="step-title"><b>1.</b> Obraz</h2>
            <div class="choices choices-os" role="radiogroup" aria-label="Obraz" id="osList"></div>
          </section>
          <section class="card card-pad" style="display:flex;flex-direction:column;gap:16px">
            <h2 class="step-title"><b>2.</b> Konfigurasiýa</h2>
            <div class="choices choices-plan" role="radiogroup" aria-label="Konfigurasiýa" id="planList"></div>
          </section>
          <section class="card card-pad" style="display:flex;flex-direction:column;gap:16px">
            <h2 class="step-title"><b>3.</b> Giriş usuly</h2>
            <div class="segmented" role="radiogroup" aria-label="Giriş usuly">
              <button type="button" role="radio" data-auth="key">SSH açar</button>
              <button type="button" role="radio" data-auth="pass">Root parol</button>
            </div>
            <div id="authBody"></div>
          </section>
          <section class="card card-pad" style="display:flex;flex-direction:column;gap:12px">
            <h2 class="step-title"><b>4.</b> Maşynyň ady</h2>
            <label class="field" style="max-width:420px">Hostname<input class="input mono" id="hostname" value="${esc(draft.hostname)}" pattern="[a-z0-9-]+" maxlength="63"></label>
            <span class="hint" id="hostHint">Diňe kiçi latyn harplary, sanlar we “-” belgisi.</span>
          </section>
        </div>
        <aside class="card card-pad side-col summary sticky" aria-label="Sargyt jemi" style="display:flex;flex-direction:column;gap:16px">
          <h2 class="step-title">Jemi</h2>
          <dl id="sum"></dl>
          <hr style="border:0;border-top:1px solid var(--border-soft);width:100%;margin:0">
          <div style="display:flex;align-items:baseline;justify-content:space-between"><span class="muted">Aýda iň köp</span><span class="total" id="sumMonthly"></span></div>
          <div style="display:flex;justify-content:space-between"><span class="muted">Sagatda</span><b id="sumHourly"></b></div>
          <div style="display:flex;justify-content:space-between"><span class="muted">Balans</span><b>${money(D.balance)}</b></div>
          <button type="button" class="btn btn-lg btn-block" id="create">Maşyny döret</button>
          <p class="hint" style="margin:0" id="createHint">Töleg her sagat balansdan aýrylýar (aýlyk baha / 720). Maşyn açmak üçin balans azyndan 24 sagatlyk tutuma ýetmeli.</p>
        </aside>
      </div>`;
  };
  views.new.mount = () => {
    const draw = () => {
      $("#osList").innerHTML = D.images.map((i) => `<button type="button" class="choice" role="radio" aria-checked="${i.id === draft.image}" data-os="${i.id}"><span class="t">${i.name}</span><span class="s">${i.version}</span></button>`).join("");
      $("#planList").innerHTML = D.plans.map((p) => `<button type="button" class="choice" role="radio" aria-checked="${p.id === draft.plan}" data-plan="${p.id}">
        <span class="s" style="font-weight:700;color:var(--primary-dark)">${p.code}</span>
        <span class="price">${fmt(p.monthly)} <small>TMT / aý</small></span>
        <span class="s">≈ ${fmt(hourly(p))} TMT / sag</span>
        <span class="specs">${p.cpu} CPU\n${p.ram} GB RAM\n${p.disk} GB HDD\n${p.speed} Mbit/s · ${p.ip} IP</span></button>`).join("");
      document.querySelectorAll("[data-auth]").forEach((b) => b.setAttribute("aria-checked", String(b.dataset.auth === draft.auth)));
      $("#authBody").innerHTML = draft.auth === "key"
        ? D.sshKeys.map((k) => `<label style="display:flex;align-items:center;gap:12px;padding:12px 14px;border:1px solid #D0D5DD;border-radius:8px;margin-bottom:8px"><input type="checkbox" data-key="${k.id}" ${draft.keys.includes(k.id) ? "checked" : ""} style="width:18px;height:18px"><span><b>${esc(k.name)}</b><span class="mono muted" style="display:block;font-size:12px">${k.fp} · ${k.type}</span></span></label>`).join("") + `<a href="#/keys">+ Täze SSH açar goş</a>`
        : `<label class="field" style="max-width:420px">Root parol<input class="input" type="password" id="pw" placeholder="Azyndan 12 simwol" value="${esc(draft.password)}"></label><span class="hint">Boş goýsaňyz, parol awtomatik döredilip, bir gezek görkeziler.</span>`;
      const p = planOf(draft.plan);
      const img = D.images.find((i) => i.id === draft.image);
      $("#sum").innerHTML = `<dt>Obraz</dt><dd>${img.name} ${img.version}</dd><dt>Konfigurasiýa</dt><dd>${p.code}</dd><dt>Resurslar</dt><dd>${p.cpu} CPU · ${p.ram} GB · ${p.disk} GB</dd><dt>Tizlik</dt><dd>${p.speed} Mbit/s</dd><dt>Giriş</dt><dd>${draft.auth === "key" ? "SSH açar" : "Root parol"}</dd>`;
      $("#sumMonthly").textContent = money(p.monthly);
      $("#sumHourly").textContent = "≈ " + fmt(hourly(p)) + " TMT";
      const enough = D.balance >= hourly(p) * 24;
      $("#create").disabled = !enough;
      $("#createHint").style.color = enough ? "" : "var(--danger)";
    };
    $("#view").addEventListener("click", (e) => {
      const os = e.target.closest("[data-os]"); if (os) { draft.image = os.dataset.os; draw(); }
      const pl = e.target.closest("[data-plan]"); if (pl) { draft.plan = pl.dataset.plan; draw(); }
      const au = e.target.closest("[data-auth]"); if (au) { draft.auth = au.dataset.auth; draw(); }
    });
    $("#view").addEventListener("change", (e) => {
      if (e.target.dataset.key) {
        const id = Number(e.target.dataset.key);
        draft.keys = e.target.checked ? [...draft.keys, id] : draft.keys.filter((k) => k !== id);
      }
      if (e.target.id === "pw") draft.password = e.target.value;
    });
    $("#hostname").addEventListener("input", (e) => {
      draft.hostname = e.target.value;
      const ok = /^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$/.test(draft.hostname);
      $("#hostHint").style.color = ok ? "" : "var(--danger)";
      $("#create").disabled = !ok;
    });
    $("#create").addEventListener("click", () => {
      if (draft.auth === "key" && draft.keys.length === 0) return toast("Azyndan bir SSH açar saýlaň");
      const id = Math.max(0, ...D.servers.map((s) => s.id)) + 1;
      const used = new Set(D.servers.map((s) => s.ip));
      let n = 50; while (used.has("203.0.113." + n)) n++;
      const img = D.images.find((i) => i.id === draft.image);
      const now = new Date();
      const s = { id, name: draft.hostname, image: img.slug, ip: "203.0.113." + n, plan: draft.plan, status: "provisioning",
        created: now.toISOString().slice(0, 16).replace("T", " "), node: "node-1", monthUsed: 0 };
      D.servers.unshift(s);
      toast("Maşyn döredilýär…");
      location.hash = "#/machine/" + id;
      setTimeout(() => { s.status = "active"; if (location.hash === "#/machine/" + id) route(); toast("Maşyn taýýar: " + s.ip); }, 2500);
    });
    draw();
  };

  views.keys = () => {
    crumbs(["Hasaplaýyş", "SSH açarlar"]);
    return `
      ${head("SSH açarlar", "Täze maşyn döredilende saýlanan açarlar root ulanyjysyna goşulýar")}
      <div class="split">
        <section class="card main-col"><div class="table-wrap"><table style="min-width:600px">
          <thead><tr><th>Ady</th><th>Görnüşi</th><th>Barmak yzy</th><th>Goşuldy</th><th><span class="sr-only">Hereketler</span></th></tr></thead>
          <tbody>${D.sshKeys.map((k) => `<tr><td><b>${esc(k.name)}</b></td><td>${esc(k.type)}</td><td class="mono">${esc(k.fp)}</td><td class="muted">${k.added}</td>
            <td class="num"><button type="button" class="btn btn-danger" style="min-height:34px" data-delkey="${k.id}">Poz</button></td></tr>`).join("")}</tbody>
        </table></div></section>
        <section class="card card-pad side-col" style="display:flex;flex-direction:column;gap:14px">
          <h2 class="step-title">Täze açar goş</h2>
          <label class="field">Ady<input class="input" id="kname" placeholder="mysal: is-noutbuk"></label>
          <label class="field">Açyk açar<textarea class="textarea" id="kval" rows="5" placeholder="ssh-ed25519 AAAAC3Nza… siz@kompýuter"></textarea></label>
          <span class="hint">Açary almak üçin: <code class="mono">cat ~/.ssh/id_ed25519.pub</code></span>
          <button type="button" class="btn" id="addKey">${svg(IC.plus, "icon-sm", 2.4)}Açary goş</button>
        </section>
      </div>`;
  };
  views.keys.mount = () => {
    $("#addKey").addEventListener("click", () => {
      const name = $("#kname").value.trim();
      const val = $("#kval").value.trim();
      const m = val.match(/^(ssh-ed25519|ssh-rsa|ecdsa-sha2-nistp256)\s+([A-Za-z0-9+/=]{20,})/);
      if (!name || !m) return toast("Ady we dogry açyk açary giriziň");
      D.sshKeys.push({ id: Date.now(), name, type: m[1] === "ssh-rsa" ? "rsa" : m[1].replace("ssh-", ""),
        fp: "SHA256:" + m[2].slice(0, 6) + "…" + m[2].slice(-4), added: new Date().toISOString().slice(0, 10) });
      toast("Açar goşuldy");
      route();
    });
    $("#view").addEventListener("click", (e) => {
      const b = e.target.closest("[data-delkey]");
      if (!b) return;
      D.sshKeys = D.sshKeys.filter((k) => k.id !== Number(b.dataset.delkey));
      toast("Açar pozuldy");
      route();
    });
  };

  views.plans = () => {
    crumbs(["Hasaplaýyş", "Konfigurasiýalar"]);
    const li = (k, v) => `<li><span>${k}</span><span>${v}</span></li>`;
    return `
      ${head("Konfigurasiýalar", "Wirtual hususy serweriň (VPS Hosting) nyrhnamasy")}
      <div class="grid plans">${D.plans.map((p) => `
        <article class="card plan ${p.popular ? "popular" : ""}">
          ${p.popular ? '<span class="tag">Köp saýlanýan</span>' : ""}
          <h2>${p.code}</h2>
          <div><div class="price">${fmt(p.monthly)} <small>TMT / aý</small></div><div class="muted">≈ ${fmt(hourly(p))} TMT sagatda</div></div>
          <ul>${li("CPU", p.cpu)}${li("RAM", p.ram + " GB")}${li("HDD", p.disk + " GB")}${li("IP", p.ip)}${li("Tizlik", p.speed + " Mbit/s")}</ul>
          <a class="btn ${p.popular ? "" : "btn-outline"}" href="#/new" data-pick="${p.id}">Saýla</a>
        </article>`).join("")}
      </div>
      <div class="alert alert-info">${svg(IC.info)}<span>Töleg her sagat balansdan aýrylýar (aýlyk baha / 720). Bir aýda tarifiň aýlyk bahasyndan köp tutulmaýar.</span></div>`;
  };
  views.plans.mount = () => {
    $("#view").addEventListener("click", (e) => { const b = e.target.closest("[data-pick]"); if (b) draft.plan = b.dataset.pick; });
  };

  views.billing = () => {
    crumbs(["Maliýe", "Balans"]);
    const typeLbl = { charge: "Tutum", topup: '<span class="badge badge-ok">Doldurmak</span>', refund: '<span class="badge badge-info">Gaýtarmak</span>' };
    const cap = D.servers.reduce((a, s) => a + planOf(s.plan).monthly, 0);
    const used = D.servers.reduce((a, s) => a + s.monthUsed, 0);
    return `
      ${head("Balans", "Töleg her sagat balansdan awtomatik aýrylýar")}
      <div class="split">
        <section class="card card-pad" style="flex:1 1 340px;background:var(--primary-dark);border-color:var(--primary-dark);color:#fff;display:flex;flex-direction:column;gap:18px">
          <span style="color:#C7D7FF">Häzirki balans</span>
          <span style="font:600 44px/1 var(--font-head)">${fmt(D.balance)} <span style="font-size:20px;color:#DCE6FF">TMT</span></span>
          <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px"><div><div style="color:#C7D7FF">Sagatlaýyn tutum</div><b>${fmt(totalHourly())} TMT</b></div><div><div style="color:#C7D7FF">Ýeter</div><b>≈ ${daysLeft()} gün</b></div></div>
        </section>
        <section class="card card-pad" style="flex:1 1 380px;display:flex;flex-direction:column;gap:14px">
          <h2 class="step-title">Balansy nädip doldurmaly</h2>
          <ol style="margin:0;padding-left:20px;display:flex;flex-direction:column;gap:8px">
            <li>Administratora hasap belgiňizi we möçberi aýdyň.</li>
            <li>Tölegi [BANK / TÖLEG USULY] arkaly geçiriň.</li>
            <li>Administrator tassyklandan soň balans şu ýerde görüner.</li>
          </ol>
          <div style="display:flex;flex-wrap:wrap;align-items:center;justify-content:space-between;gap:12px;background:#F9FAFB;border:1px solid var(--border-soft);border-radius:8px;padding:12px 14px">
            <div><div class="muted" style="font-size:12px">Hasap belgiňiz</div><b class="mono" style="font-size:16px">U-10482</b></div>
            <button type="button" class="btn btn-ghost" data-copy="U-10482">${svg(IC.copy, "icon-sm")}Göçür</button>
          </div>
        </section>
      </div>
      <section class="card card-pad">
        <div style="display:flex;flex-wrap:wrap;justify-content:space-between;gap:8px;margin-bottom:16px"><h2 class="step-title">Şu aý boýunça tutum</h2><span class="muted">Jemi <b style="color:var(--ink)">${money(used)}</b> · aýlyk çäk ${money(cap)}</span></div>
        <div class="grid" style="grid-template-columns:repeat(auto-fit,minmax(200px,1fr));gap:12px">${D.servers.map((s) => {
          const p = planOf(s.plan); const pct = Math.min(100, Math.round(s.monthUsed / p.monthly * 100));
          return `<div style="border:1px solid var(--border-soft);border-radius:10px;padding:14px 16px" class="meter"><div class="top"><b>${esc(s.name)} · ${p.code.replace("-2020", "")}</b><span>${fmt(s.monthUsed)}</span></div><div class="bar"><span style="width:${pct}%"></span></div><span class="hint">çäk ${money(p.monthly)}</span></div>`;
        }).join("")}</div>
      </section>
      <section class="card">
        <div class="card-head"><h2>Amallar taryhy</h2></div>
        <div class="table-wrap"><table style="min-width:720px">
          <thead><tr><th>Wagty</th><th>Görnüşi</th><th>Düşündiriş</th><th class="num">Möçberi</th><th class="num">Galyndy</th></tr></thead>
          <tbody>${D.transactions.map((t) => `<tr><td class="muted">${t.at}</td><td>${typeLbl[t.type]}</td><td>${esc(t.note)}</td>
            <td class="num" style="font-weight:600;color:${t.amount > 0 ? "var(--ok)" : "inherit"}">${t.amount > 0 ? "+" : "−"}${fmt(Math.abs(t.amount))}</td><td class="num muted">${fmt(t.after)}</td></tr>`).join("")}</tbody>
        </table></div>
      </section>`;
  };

  views.admin = () => {
    crumbs(["Admin paneli"]);
    const st = { active: '<span class="badge badge-ok">Işjeň</span>', suspended: '<span class="badge badge-warn">Saklanan</span>', pending: '<span class="badge badge-off">Tassyklanmadyk</span>' };
    return `
      ${head("Admin paneli", "Ulanyjylar, balans doldurmak we infrastruktura")}
      <section class="grid grid-stats">
        <div class="card stat"><span class="label">Ulanyjylar</span><span class="value">128</span></div>
        <div class="card stat"><span class="label">Işjeň maşynlar</span><span class="value">74</span></div>
        <div class="card stat"><span class="label">Boş IP salgylar</span><span class="value">11 <small>/ 90</small></span></div>
        <div class="card stat"><span class="label">node-1 RAM</span><span class="value">78%</span><div class="meter"><div class="bar"><span class="warn" style="width:78%"></span></div></div></div>
      </section>
      <div class="split">
        <section class="card main-col">
          <div class="toolbar"><label class="search">${svg(IC.search, "icon-sm")}<span class="sr-only">Ulanyjy gözle</span><input type="search" placeholder="Email, at ýa-da U-belgi"></label></div>
          <div class="table-wrap"><table style="min-width:680px">
            <thead><tr><th>Ulanyjy</th><th>Maşyn</th><th class="num">Balans</th><th>Ýagdaýy</th><th><span class="sr-only">Hereketler</span></th></tr></thead>
            <tbody>${D.users.map((u) => `<tr><td><b>${esc(u.name)}</b><div class="muted" style="font-size:12px">${u.id} · ${esc(u.email)}</div></td><td>${u.servers}</td>
              <td class="num" style="font-weight:600;color:${u.balance < 0 ? "var(--danger)" : "inherit"}">${fmt(u.balance)}</td><td>${st[u.status]}</td>
              <td class="num"><button type="button" class="btn btn-outline" style="min-height:34px" data-topup="${u.id}">Doldur</button></td></tr>`).join("")}</tbody>
          </table></div>
        </section>
        <section class="card card-pad side-col" style="display:flex;flex-direction:column;gap:14px;border:2px solid var(--primary)" aria-label="Balansy doldurmak">
          <div><h2 class="step-title">Balansy doldur</h2><span class="muted" id="tuWho"></span></div>
          <label class="field">Möçber (TMT)<input class="input" type="number" min="1" step="0.01" id="tuAmt" value="100"></label>
          <div style="display:flex;flex-wrap:wrap;gap:8px">${[50, 100, 200, 500].map((v) => `<button type="button" class="btn btn-ghost" style="min-height:34px;border-radius:999px" data-preset="${v}">${v} TMT</button>`).join("")}</div>
          <label class="field">Bellik (çek ýa-da töleg belgisi)<textarea class="textarea" rows="2" id="tuNote" placeholder="mysal: çek №2232, nagt" style="font-family:var(--font)"></textarea></label>
          <div style="display:flex;justify-content:space-between"><span class="muted">Täze balans</span><b id="tuNew"></b></div>
          <button type="button" class="btn btn-lg" id="tuGo">Tassykla we goş</button>
          <span class="hint">Her doldurma audit žurnalyna ýazylýar.</span>
        </section>
      </div>`;
  };
  views.admin.mount = () => {
    let who = D.users[0];
    const draw = () => {
      $("#tuWho").textContent = `${who.name} · ${who.id} · häzir ${money(who.balance)}`;
      $("#tuNew").textContent = money(who.balance + (Number($("#tuAmt").value) || 0));
    };
    $("#view").addEventListener("click", (e) => {
      const u = e.target.closest("[data-topup]"); if (u) { who = D.users.find((x) => x.id === u.dataset.topup); draw(); $("#tuAmt").focus(); }
      const p = e.target.closest("[data-preset]"); if (p) { $("#tuAmt").value = p.dataset.preset; draw(); }
    });
    $("#tuAmt").addEventListener("input", draw);
    $("#tuGo").addEventListener("click", () => {
      const amt = Number($("#tuAmt").value);
      if (!(amt > 0)) return toast("Dogry möçber giriziň");
      who.balance += amt;
      if (who.id === "U-10482") {
        D.balance += amt;
        D.transactions.unshift({ at: new Date().toISOString().slice(0, 16).replace("T", " "), type: "topup", note: "Administrator · " + ($("#tuNote").value.trim() || "bellik ýok"), amount: amt, after: D.balance });
      }
      if (who.status === "suspended" && who.balance > 0) who.status = "active";
      toast(`${who.name}: +${fmt(amt)} TMT goşuldy`);
      route();
    });
    draw();
  };

  views.notfound = () => {
    crumbs(["Taýýarlanýar"]);
    return `${head("Bu bölüm taýýarlanýar", "Test wersiýasynda bu sahypa entek ýok.")}<a class="btn" href="#/dashboard" style="align-self:flex-start">Baş sahypa</a>`;
  };

  // ---------- Giriş ----------
  let authTab = "login";
  function renderAuth() {
    const tabBtn = (id, label) => `<button type="button" role="tab" aria-selected="${authTab === id}" aria-checked="${authTab === id}" data-tab="${id}">${label}</button>`;
    const login = `
      <form class="auth-card" id="loginForm" novalidate>
        <div><h2>Hasabyňyza giriň</h2><p class="muted" style="margin:4px 0 0">Email we parolyňyzy ýazyň</p></div>
        <label class="field">Email<input class="input" type="email" required placeholder="siz@mysal.tm" autocomplete="username"></label>
        <label class="field">Parol<input class="input" type="password" required placeholder="••••••••" autocomplete="current-password"></label>
        <div style="display:flex;justify-content:space-between;align-items:center;gap:12px"><label style="display:flex;align-items:center;gap:8px"><input type="checkbox" style="width:18px;height:18px">Ýatda sakla</label><a href="#/login">Paroly unutdyňyzmy?</a></div>
        <button class="btn btn-lg btn-block" type="submit">Giriş</button>
      </form>`;
    const reg = `
      <form class="auth-card" id="regForm" novalidate>
        <div><h2>Täze hasap açyň</h2><p class="muted" style="margin:4px 0 0">Emailiňize tassyklama salgysy iberiler</p></div>
        <label class="field">Doly adyňyz<input class="input" placeholder="Aman Amanow" autocomplete="name"></label>
        <label class="field">Email<input class="input" type="email" placeholder="siz@mysal.tm" autocomplete="email"></label>
        <label class="field">Telefon<input class="input" type="tel" placeholder="+993 6X XXXXXX" autocomplete="tel"></label>
        <label class="field">Parol<input class="input" type="password" placeholder="Azyndan 10 simwol" autocomplete="new-password"></label>
        <label style="display:flex;gap:8px;align-items:flex-start"><input type="checkbox" style="width:18px;height:18px;margin-top:2px">Ulanyş şertlerini we gizlinlik syýasatyny kabul edýärin</label>
        <button class="btn btn-lg btn-block" type="submit">Hasaby döret</button>
      </form>`;
    $("#auth").innerHTML = `
      <section class="auth-hero">
        <a class="brand" href="#/login"><span class="brand-logo"><img src="assets/logo.svg" alt="ASTU"></span><span class="brand-name">VPS</span></a>
        <div style="display:flex;flex-direction:column;gap:20px">
          <h1>Ýerli VPS serwerler, sagatlaýyn töleg bilen</h1>
          <p>Konfigurasiýa saýlaň, maşynyňyzy açyň we SSH bilen giriň. Diňe ulanan sagadyňyz üçin tölärsiňiz.</p>
          <ul>
            <li>${svg(IC.check, "icon", 2.4)}Ubuntu, Debian, AlmaLinux we Rocky Linux obrazlary</li>
            <li>${svg(IC.check, "icon", 2.4)}Root parol ýa-da SSH açar bilen giriş</li>
            <li>${svg(IC.check, "icon", 2.4)}Aýlyk çäk: tarif bahasyndan köp tutulmaýar</li>
          </ul>
        </div>
        <p style="margin:0;color:#C7D7FF;font-size:13px">© 2026 ASTU</p>
      </section>
      <section class="auth-form">
        <div style="width:100%;max-width:400px;display:flex;flex-direction:column;gap:24px">
          <div class="segmented" role="tablist" aria-label="Giriş görnüşi" style="max-width:none">${tabBtn("login", "Giriş")}${tabBtn("register", "Hasaba alyş")}</div>
          ${authTab === "login" ? login : reg}
        </div>
      </section>`;
    $("#auth").querySelectorAll("[data-tab]").forEach((b) => b.addEventListener("click", () => { authTab = b.dataset.tab; renderAuth(); }));
    const f = $("#loginForm") || $("#regForm");
    f.addEventListener("submit", (e) => {
      e.preventDefault();
      if (f.id === "regForm") { authTab = "login"; renderAuth(); toast("Hasap döredildi. Emailiňizi tassyklaň."); return; }
      location.hash = "#/dashboard";
    });
  }

  // ---------- Router ----------
  function route() {
    const [name, arg] = (location.hash.replace(/^#\//, "") || "login").split("/");
    const isAuth = name === "login";
    $("#auth").hidden = !isAuth;
    $("#app").hidden = isAuth;
    if (isAuth) { renderAuth(); return; }

    const view = views[name] || views.notfound;
    const navKey = name === "machine" || name === "new" ? "machines" : name;
    renderNav(navKey);
    $("#topBalance").textContent = money(D.balance);

    // Öňki diňleýjileri arassalamak üçin main-i täzeden döredýäris
    const old = $("#view");
    const fresh = old.cloneNode(false);
    old.replaceWith(fresh);
    fresh.innerHTML = view(arg);
    if (view.mount) view.mount(arg);
    fresh.addEventListener("click", (e) => { const c = e.target.closest("[data-copy]"); if (c) copy(c.dataset.copy); });
    $("#sidebar").classList.remove("open");
    window.scrollTo(0, 0);
  }

  $("#menuToggle").addEventListener("click", () => $("#sidebar").classList.toggle("open"));
  window.addEventListener("hashchange", route);
  route();
})();
