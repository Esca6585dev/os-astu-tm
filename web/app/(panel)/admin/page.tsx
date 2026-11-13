"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { Badge, PageHead, useCrumbs, usePanel } from "@/components/Shell";
import { Icon } from "@/components/icons";
import { api, ApiError, dt, METHOD, money, ramGB, tmt, TOPUP_STATUS, type Image, type Plan, type Server, type Topup, type User } from "@/lib/api";

type AUser = User & { servers: number; created_at: string };
type Stats = { users: number; running_servers: number; servers: number; free_ips: number; total_ips: number; pending_topups: number;
  nodes: { id: number; name: string; vcpu: [number, number]; ram_mb: [number, number]; disk_gb: [number, number] }[] };
type Node = { id: number; name: string; storage: string; bridge: string; max_vcpu: number; max_ram_mb: number; max_disk_gb: number; is_active: boolean };
type Pool = { id: number; node: string; cidr: string; gateway: string; dns: string; total: number; free: number };
type Audit = { id: number; actor: string; action: string; target: string; meta: Record<string, unknown>; ip: string; created_at: string };

const TABS = ["Doldurmalar", "Ulanyjylar", "Maşynlar", "Node-lar", "IP bloklary", "Obrazlar", "Tarifler", "Audit"] as const;

export default function Admin() {
  useCrumbs("Admin paneli");
  const { me, refresh } = usePanel();
  const [tab, setTab] = useState<(typeof TABS)[number]>("Doldurmalar");
  const [stats, setStats] = useState<Stats | null>(null);
  const loadStats = useCallback(() => { refresh(); return api<Stats>("/admin/stats").then(setStats).catch(() => {}); }, [refresh]);
  useEffect(() => { loadStats(); }, [loadStats]);

  if (me.role !== "admin") return <p className="empty">Diňe administrator üçin.</p>;
  const pct = (a: [number, number]) => (a[1] ? Math.round((a[0] / a[1]) * 100) : 0);

  return (
    <>
      <PageHead title="Admin paneli" sub="Ulanyjylar, balans doldurmak we infrastruktura" />
      <section className="grid grid-stats">
        <div className="card stat"><span className="label">Ulanyjylar</span><span className="value">{stats?.users ?? "—"}</span></div>
        <div className="card stat"><span className="label">Işjeň maşynlar</span><span className="value">{stats?.running_servers ?? "—"} <small>/ {stats?.servers ?? 0}</small></span></div>
        <div className="card stat"><span className="label">Boş IP salgylar</span><span className="value">{stats?.free_ips ?? "—"} <small>/ {stats?.total_ips ?? 0}</small></span></div>
        {stats?.nodes.map((n) => (
          <div key={n.id} className="card stat"><span className="label">{n.name} · paýlanan RAM</span><span className="value">{pct(n.ram_mb)}%</span>
            <div className="meter"><div className="bar"><span className={pct(n.ram_mb) > 80 ? "warn" : ""} style={{ width: `${pct(n.ram_mb)}%` }} /></div></div>
            <span className="hint">CPU {n.vcpu[0]}/{n.vcpu[1]} · Disk {n.disk_gb[0]}/{n.disk_gb[1]} GB</span></div>
        ))}
      </section>
      {stats && (stats.nodes.length === 0 || stats.total_ips === 0) && (
        <div className="alert alert-warn"><Icon name="alert" /><span>Maşyn döretmek üçin azyndan bir Proxmox node, bir IP blok we bir obraz goşuň (docs/PROXMOX.md).</span></div>
      )}
      <div className="tabs" role="tablist">
        {TABS.map((t) => <button key={t} type="button" role="tab" aria-selected={tab === t} onClick={() => setTab(t)}>
          {t}{t === "Doldurmalar" && !!stats?.pending_topups && <span className="badge badge-warn" style={{ marginLeft: 8 }}>{stats.pending_topups}</span>}</button>)}
      </div>
      {tab === "Doldurmalar" && <TopupsTab onChange={loadStats} />}
      {tab === "Ulanyjylar" && <UsersTab onChange={loadStats} />}
      {tab === "Maşynlar" && <ServersTab />}
      {tab === "Node-lar" && <NodesTab onChange={loadStats} />}
      {tab === "IP bloklary" && <PoolsTab onChange={loadStats} />}
      {tab === "Obrazlar" && <ImagesTab />}
      {tab === "Tarifler" && <PlansTab />}
      {tab === "Audit" && <AuditTab />}
    </>
  );
}

function useErr() {
  const { toast } = usePanel();
  return (e: unknown) => toast(e instanceof ApiError ? e.message : "Ýalňyşlyk");
}

function TopupsTab({ onChange }: { onChange: () => void }) {
  const { toast } = usePanel();
  const onErr = useErr();
  const [status, setStatus] = useState("pending");
  const [list, setList] = useState<Topup[] | null>(null);
  const [busy, setBusy] = useState(0);
  const load = useCallback(() => api<Topup[]>(`/admin/topups?limit=100&status=${status}`).then(setList).catch(() => setList([])), [status]);
  useEffect(() => { setList(null); load(); }, [load]);
  // Täze haýyşlar özi görünsin
  useEffect(() => { const i = setInterval(() => { load(); onChange(); }, 30000); return () => clearInterval(i); }, [load, onChange]);

  async function approve(t: Topup) {
    if (!window.confirm(`${t.full_name || t.user_email} (${t.account_no}) balansyna ${money(t.amount)} goşulsynmy?\n\nUsul: ${METHOD[t.method]}\nÇek / töleg belgisi: ${t.reference}\n\nTölegiň hakykatdan gelendigini barladyňyzmy?`)) return;
    setBusy(t.id);
    try { await api(`/admin/topups/${t.id}/approve`, { method: "POST", body: {} }); toast(`${t.account_no}: +${tmt(t.amount)} TMT goşuldy`); load(); onChange(); }
    catch (e) { onErr(e); } finally { setBusy(0); }
  }
  async function reject(t: Topup) {
    const note = window.prompt(`${t.account_no} · ${money(t.amount)} haýyşy ret edilýär.\nSebäbini ýazyň (ulanyjy görer):`, "Töleg tapylmady");
    if (note === null) return;
    setBusy(t.id);
    try { await api(`/admin/topups/${t.id}/reject`, { method: "POST", body: { note } }); toast("Haýyş ret edildi"); load(); onChange(); }
    catch (e) { onErr(e); } finally { setBusy(0); }
  }

  return (
    <section className="card">
      <div className="toolbar">
        <label style={{ display: "flex", alignItems: "center", gap: 8 }}>Ýagdaýy
          <select className="select" value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="pending">Garaşylýan</option><option value="approved">Tassyklanan</option><option value="rejected">Ret edilen</option>
            <option value="cancelled">Ýatyrylan</option><option value="all">Hemmesi</option>
          </select></label>
        <span className="muted">Tölegiň bank hasabyna ýa-da kassa gelendigini barlap, soňra tassyklaň. Her karar audit žurnalyna ýazylýar.</span>
      </div>
      <div className="table-wrap"><table style={{ minWidth: 960 }}>
        <thead><tr><th>Wagty</th><th>Ulanyjy</th><th className="num">Möçberi</th><th>Usul</th><th>Çek / töleg belgisi</th><th>Ýagdaýy</th><th><span className="sr-only">Hereketler</span></th></tr></thead>
        <tbody>{list === null ? <tr><td colSpan={7} className="empty">Ýüklenýär…</td></tr> :
          list.length === 0 ? <tr><td colSpan={7} className="empty">{status === "pending" ? "Garaşylýan haýyş ýok" : "Haýyş ýok"}</td></tr> : list.map((t) => {
            const [label, cls] = TOPUP_STATUS[t.status] ?? [t.status, "badge-off"];
            return (
              <tr key={t.id}>
                <td className="muted" style={{ whiteSpace: "nowrap" }}>{dt(t.created_at)}</td>
                <td><b>{t.full_name || t.user_email}</b><div className="muted" style={{ fontSize: 12 }}>{t.account_no} · {t.user_email} · balans {tmt(t.user_balance ?? 0)}</div></td>
                <td className="num" style={{ fontWeight: 700, fontSize: 15 }}>{tmt(t.amount)}</td>
                <td>{METHOD[t.method]}</td>
                <td><span className="mono">{t.reference}</span>{t.comment && <div className="muted" style={{ fontSize: 12 }}>{t.comment}</div>}</td>
                <td><span className={`badge ${cls}`}>{label}</span>{t.admin_note && <div className="muted" style={{ fontSize: 12 }}>{t.admin_note}</div>}</td>
                <td className="num" style={{ whiteSpace: "nowrap" }}>{t.status === "pending" && <>
                  <button type="button" className="btn" style={{ minHeight: 34 }} disabled={busy === t.id} onClick={() => approve(t)}><Icon name="check" small sw={2.4} />Tassykla</button>{" "}
                  <button type="button" className="btn btn-ghost" style={{ minHeight: 34 }} disabled={busy === t.id} onClick={() => reject(t)}>Ret et</button></>}</td>
              </tr>
            );
          })}</tbody>
      </table></div>
    </section>
  );
}

function UsersTab({ onChange }: { onChange: () => void }) {
  const { toast, refresh } = usePanel();
  const onErr = useErr();
  const [q, setQ] = useState("");
  const [list, setList] = useState<AUser[]>([]);
  const [who, setWho] = useState<AUser | null>(null);
  const [amount, setAmount] = useState("100");
  const [note, setNote] = useState("");
  const load = useCallback(() => api<AUser[]>(`/admin/users?limit=100&q=${encodeURIComponent(q)}`).then((l) => {
    setList(l); setWho((w) => (w ? l.find((x) => x.id === w.id) ?? null : l[0] ?? null));
  }), [q]);
  useEffect(() => { const t = setTimeout(load, 250); return () => clearTimeout(t); }, [load]);

  async function topup() {
    if (!who) return;
    try {
      await api(`/admin/users/${who.id}/topup`, { method: "POST", body: { amount: Number(amount), note } });
      toast(`${who.email}: +${amount} TMT`); setNote(""); load(); refresh(); onChange();
    } catch (e) { onErr(e); }
  }
  async function setStatus(u: AUser, status: string) {
    try { await api(`/admin/users/${u.id}`, { method: "PATCH", body: { status } }); load(); } catch (e) { onErr(e); }
  }
  const newBal = who ? who.balance + Math.round((Number(amount) || 0) * 100) : 0;

  return (
    <div className="split">
      <section className="card main-col">
        <div className="toolbar"><label className="search"><Icon name="search" small /><span className="sr-only">Gözleg</span>
          <input type="search" value={q} onChange={(e) => setQ(e.target.value)} placeholder="Email, at ýa-da U-belgi" /></label></div>
        <div className="table-wrap"><table style={{ minWidth: 700 }}>
          <thead><tr><th>Ulanyjy</th><th>Maşyn</th><th className="num">Balans</th><th>Ýagdaýy</th><th><span className="sr-only">Hereketler</span></th></tr></thead>
          <tbody>{list.map((u) => (
            <tr key={u.id} style={who?.id === u.id ? { background: "var(--primary-soft)" } : undefined}>
              <td><b>{u.full_name || u.email}</b>{u.role === "admin" && <span className="badge badge-info" style={{ marginLeft: 8 }}>ADMIN</span>}
                <div className="muted" style={{ fontSize: 12 }}>{u.account_no} · {u.email}</div></td>
              <td>{u.servers}</td>
              <td className="num" style={{ fontWeight: 600, color: u.balance < 0 ? "var(--danger)" : undefined }}>{tmt(u.balance)}</td>
              <td><Badge status={u.status} /></td>
              <td className="num" style={{ whiteSpace: "nowrap" }}>
                <button type="button" className="btn btn-outline" style={{ minHeight: 34 }} onClick={() => setWho(u)}>Doldur</button>{" "}
                {u.status === "blocked" ? <button type="button" className="btn btn-ghost" style={{ minHeight: 34 }} onClick={() => setStatus(u, "active")}>Aç</button>
                  : <button type="button" className="btn btn-ghost" style={{ minHeight: 34 }} onClick={() => window.confirm(`${u.email} bloklansynmy?`) && setStatus(u, "blocked")}>Blokla</button>}
              </td>
            </tr>))}</tbody>
        </table></div>
      </section>
      <section className="card card-pad side-col" aria-label="Balansy doldurmak" style={{ display: "flex", flexDirection: "column", gap: 14, border: "2px solid var(--primary)" }}>
        <div><h2 className="step-title">Balansy doldur</h2><span className="muted">{who ? `${who.full_name || who.email} · ${who.account_no} · häzir ${money(who.balance)}` : "Ulanyjy saýlaň"}</span></div>
        <label className="field">Möçber (TMT)<input className="input" type="number" min="0.01" step="0.01" value={amount} onChange={(e) => setAmount(e.target.value)} /></label>
        <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>{[50, 100, 200, 500].map((v) => (
          <button key={v} type="button" className="btn btn-ghost" style={{ minHeight: 34, borderRadius: 999 }} onClick={() => setAmount(String(v))}>{v} TMT</button>))}</div>
        <label className="field">Bellik (çek ýa-da töleg belgisi)<textarea className="textarea" rows={2} value={note} onChange={(e) => setNote(e.target.value)} placeholder="mysal: çek №2232, nagt" style={{ fontFamily: "var(--font)" }} /></label>
        <div style={{ display: "flex", justifyContent: "space-between" }}><span className="muted">Täze balans</span><b>{who ? money(newBal) : "—"}</b></div>
        <button type="button" className="btn btn-lg" disabled={!who || !note.trim() || !(Number(amount) > 0)} onClick={topup}>Tassykla we goş</button>
        <span className="hint">Her doldurma audit žurnalyna ýazylýar. Öçürilen serwerler bir minudyň dowamynda awtomatik açylýar.</span>
      </section>
    </div>
  );
}

function ServersTab() {
  const [q, setQ] = useState("");
  const [list, setList] = useState<Server[]>([]);
  useEffect(() => { const t = setTimeout(() => api<Server[]>(`/admin/servers?limit=100&q=${encodeURIComponent(q)}`).then(setList), 250); return () => clearTimeout(t); }, [q]);
  return (
    <section className="card">
      <div className="toolbar"><label className="search"><Icon name="search" small /><span className="sr-only">Gözleg</span>
        <input type="search" value={q} onChange={(e) => setQ(e.target.value)} placeholder="Hostname, email ýa-da IP" /></label></div>
      <div className="table-wrap"><table style={{ minWidth: 900 }}>
        <thead><tr><th>Maşyn</th><th>Eýesi</th><th>IP</th><th>Konfigurasiýa</th><th>Node / VMID</th><th>Ýagdaýy</th><th>Döredildi</th></tr></thead>
        <tbody>{list.length === 0 ? <tr><td colSpan={7} className="empty">Maşyn ýok</td></tr> : list.map((s) => (
          <tr key={s.id}><td><b>{s.hostname}</b><div className="muted" style={{ fontSize: 12 }}>{s.image.slug}</div></td><td>{s.user_email}</td>
            <td className="mono">{s.ip || "—"}</td><td>{s.plan.code}</td><td>{s.node}{s.vmid ? ` / ${s.vmid}` : ""}</td>
            <td><Badge status={s.status} />{s.last_job_error && <div className="form-error" style={{ fontSize: 12 }}>{s.last_job_error}</div>}</td><td className="muted">{dt(s.created_at)}</td></tr>))}</tbody>
      </table></div>
    </section>
  );
}

function NodesTab({ onChange }: { onChange: () => void }) {
  const onErr = useErr();
  const { toast } = usePanel();
  const [list, setList] = useState<Node[]>([]);
  const load = useCallback(() => api<Node[]>("/admin/nodes").then(setList), []);
  useEffect(() => { load(); }, [load]);
  async function add(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = Object.fromEntries(new FormData(e.currentTarget)) as Record<string, string>;
    try {
      await api("/admin/nodes", { method: "POST", body: { name: f.name, storage: f.storage, bridge: f.bridge,
        max_vcpu: +f.max_vcpu, max_ram_mb: Math.round(+f.max_ram_gb * 1024), max_disk_gb: +f.max_disk_gb, is_active: true } });
      toast("Node goşuldy"); (e.target as HTMLFormElement).reset(); load(); onChange();
    } catch (err) { onErr(err); }
  }
  async function toggle(n: Node) {
    try { await api(`/admin/nodes/${n.id}`, { method: "PATCH", body: { ...n, is_active: !n.is_active } }); load(); } catch (e) { onErr(e); }
  }
  return (
    <>
      <section className="card"><div className="table-wrap"><table>
        <thead><tr><th>Proxmox node</th><th>Storage</th><th>Bridge</th><th>CPU</th><th>RAM</th><th>Disk</th><th>Ýagdaýy</th></tr></thead>
        <tbody>{list.length === 0 ? <tr><td colSpan={7} className="empty">Node ýok</td></tr> : list.map((n) => (
          <tr key={n.id}><td><b>{n.name}</b></td><td>{n.storage}</td><td>{n.bridge}</td><td>{n.max_vcpu}</td><td>{ramGB(n.max_ram_mb)}</td><td>{n.max_disk_gb} GB</td>
            <td><button type="button" className="btn btn-ghost" style={{ minHeight: 32 }} onClick={() => toggle(n)}>{n.is_active ? "Işjeň" : "Öçük"}</button></td></tr>))}</tbody>
      </table></div></section>
      <form className="card card-pad" onSubmit={add} style={{ display: "flex", flexDirection: "column", gap: 12 }}>
        <h2 className="step-title">Node goş</h2>
        <p className="hint" style={{ margin: 0 }}>Ady Proxmox-daky node ady bilen birmeňzeş bolmaly (mysal: pve1). Kuwwat — maşynlara paýlap boljak iň köp mukdar.</p>
        <div className="inline-form">
          <label className="field">Ady<input className="input" name="name" required placeholder="pve1" /></label>
          <label className="field">Storage<input className="input" name="storage" defaultValue="local-lvm" /></label>
          <label className="field">Bridge<input className="input" name="bridge" defaultValue="vmbr0" /></label>
          <label className="field">vCPU<input className="input" name="max_vcpu" type="number" min="1" required /></label>
          <label className="field">RAM (GB)<input className="input" name="max_ram_gb" type="number" min="1" required /></label>
          <label className="field">Disk (GB)<input className="input" name="max_disk_gb" type="number" min="10" required /></label>
          <button className="btn"><Icon name="plus" small sw={2.4} />Goş</button>
        </div>
      </form>
    </>
  );
}

function PoolsTab({ onChange }: { onChange: () => void }) {
  const onErr = useErr();
  const { toast } = usePanel();
  const [list, setList] = useState<Pool[]>([]);
  const [nodes, setNodes] = useState<Node[]>([]);
  const load = useCallback(() => api<Pool[]>("/admin/ip-pools").then(setList), []);
  useEffect(() => { load(); api<Node[]>("/admin/nodes").then(setNodes); }, [load]);
  async function add(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = Object.fromEntries(new FormData(e.currentTarget)) as Record<string, string>;
    try {
      const r = await api<{ addresses: number }>("/admin/ip-pools", { method: "POST", body: { node_id: +f.node_id, cidr: f.cidr, gateway: f.gateway,
        dns: f.dns, range_start: f.range_start, range_end: f.range_end, reserved: f.reserved.split(/[\s,]+/).filter(Boolean) } });
      toast(`${r.addresses} IP goşuldy`); (e.target as HTMLFormElement).reset(); load(); onChange();
    } catch (err) { onErr(err); }
  }
  async function del(p: Pool) {
    if (!window.confirm(`${p.cidr} pozulsynmy?`)) return;
    try { await api(`/admin/ip-pools/${p.id}`, { method: "DELETE" }); load(); onChange(); } catch (e) { onErr(e); }
  }
  return (
    <>
      <section className="card"><div className="table-wrap"><table>
        <thead><tr><th>CIDR</th><th>Node</th><th>Şlýuz</th><th>DNS</th><th className="num">Boş / jemi</th><th><span className="sr-only">Hereketler</span></th></tr></thead>
        <tbody>{list.length === 0 ? <tr><td colSpan={6} className="empty">IP blok ýok</td></tr> : list.map((p) => (
          <tr key={p.id}><td className="mono">{p.cidr}</td><td>{p.node}</td><td className="mono">{p.gateway}</td><td>{p.dns}</td><td className="num">{p.free} / {p.total}</td>
            <td className="num"><button type="button" className="btn btn-danger" style={{ minHeight: 32 }} onClick={() => del(p)}>Poz</button></td></tr>))}</tbody>
      </table></div></section>
      <form className="card card-pad" onSubmit={add} style={{ display: "flex", flexDirection: "column", gap: 12 }}>
        <h2 className="step-title">IP blok goş</h2>
        <p className="hint" style={{ margin: 0 }}>Tor, broadcast we şlýuz salgylary awtomatik aýrylýar. Aralyk berilse, diňe şol aralykdaky IP-ler ulanylýar.</p>
        <div className="inline-form">
          <label className="field">Node<select className="select" name="node_id" required>{nodes.map((n) => <option key={n.id} value={n.id}>{n.name}</option>)}</select></label>
          <label className="field">CIDR<input className="input mono" name="cidr" required placeholder="216.250.11.0/24" /></label>
          <label className="field">Şlýuz<input className="input mono" name="gateway" required placeholder="216.250.11.1" /></label>
          <label className="field">DNS<input className="input" name="dns" defaultValue="8.8.8.8 1.1.1.1" /></label>
          <label className="field">Aralygyň başy<input className="input mono" name="range_start" placeholder="(islege görä)" /></label>
          <label className="field">Aralygyň soňy<input className="input mono" name="range_end" placeholder="(islege görä)" /></label>
          <label className="field">Bron (ulanylmaýan)<input className="input mono" name="reserved" placeholder="IP, IP…" /></label>
          <button className="btn" disabled={nodes.length === 0}><Icon name="plus" small sw={2.4} />Goş</button>
        </div>
      </form>
    </>
  );
}

function ImagesTab() {
  const onErr = useErr();
  const { toast } = usePanel();
  const [list, setList] = useState<Image[]>([]);
  const load = useCallback(() => api<Image[]>("/admin/images").then(setList), []);
  useEffect(() => { load(); }, [load]);
  async function add(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = Object.fromEntries(new FormData(e.currentTarget)) as Record<string, string>;
    try {
      await api("/admin/images", { method: "POST", body: { slug: f.slug, name: f.name, version: f.version, proxmox_template_vmid: +f.vmid, is_active: true, sort: +f.sort || 0 } });
      toast("Obraz goşuldy"); (e.target as HTMLFormElement).reset(); load();
    } catch (err) { onErr(err); }
  }
  async function toggle(i: Image) {
    try { await api(`/admin/images/${i.id}`, { method: "PATCH", body: { ...i, is_active: !i.is_active } }); load(); } catch (e) { onErr(e); }
  }
  return (
    <>
      <section className="card"><div className="table-wrap"><table>
        <thead><tr><th>Ady</th><th>Wersiýa</th><th>Kod</th><th>Proxmox şablon VMID</th><th>Ýagdaýy</th></tr></thead>
        <tbody>{list.length === 0 ? <tr><td colSpan={5} className="empty">Obraz ýok</td></tr> : list.map((i) => (
          <tr key={i.id}><td><b>{i.name}</b></td><td>{i.version}</td><td className="mono">{i.slug}</td><td className="mono">{i.proxmox_template_vmid}</td>
            <td><button type="button" className="btn btn-ghost" style={{ minHeight: 32 }} onClick={() => toggle(i)}>{i.is_active ? "Işjeň" : "Öçük"}</button></td></tr>))}</tbody>
      </table></div></section>
      <form className="card card-pad" onSubmit={add} style={{ display: "flex", flexDirection: "column", gap: 12 }}>
        <h2 className="step-title">Obraz goş</h2>
        <p className="hint" style={{ margin: 0 }}>Şablon her node-da şol bir VMID bilen bolmaly (docs/PROXMOX.md, 4-nji ädim).</p>
        <div className="inline-form">
          <label className="field">Ady<input className="input" name="name" required placeholder="Ubuntu" /></label>
          <label className="field">Wersiýa<input className="input" name="version" required placeholder="24.04 LTS" /></label>
          <label className="field">Kod (slug)<input className="input mono" name="slug" required placeholder="ubuntu-24_04_2025-11" /></label>
          <label className="field">Şablon VMID<input className="input mono" name="vmid" type="number" min="100" required placeholder="9000" /></label>
          <label className="field">Tertip<input className="input" name="sort" type="number" defaultValue="0" /></label>
          <button className="btn"><Icon name="plus" small sw={2.4} />Goş</button>
        </div>
      </form>
    </>
  );
}

function PlansTab() {
  const onErr = useErr();
  const { toast } = usePanel();
  const [list, setList] = useState<Plan[]>([]);
  const load = useCallback(() => api<Plan[]>("/admin/plans").then(setList), []);
  useEffect(() => { load(); }, [load]);
  async function save(p: Plan, priceTMT: string, active: boolean) {
    try {
      await api(`/admin/plans/${p.id}`, { method: "PATCH", body: { ...p, price_monthly: Math.round(Number(priceTMT) * 100), is_active: active } });
      toast(`${p.code} ýatda saklandy`); load();
    } catch (e) { onErr(e); }
  }
  return (
    <section className="card"><div className="table-wrap"><table style={{ minWidth: 820 }}>
      <thead><tr><th>Kod</th><th>CPU</th><th>RAM</th><th>HDD</th><th>Tizlik</th><th>Aýlyk baha (TMT)</th><th>Ýagdaýy</th></tr></thead>
      <tbody>{list.map((p) => <PlanRow key={p.id} p={p} onSave={save} />)}</tbody>
    </table></div>
      <p className="hint" style={{ padding: "0 16px 16px", margin: 0 }}>Bahanyň üýtgemegi täze sagatlardan başlap hasaplanýar. Bar bolan maşynlaryň konfigurasiýasy üýtgemeýär.</p>
    </section>
  );
}

function PlanRow({ p, onSave }: { p: Plan; onSave: (p: Plan, price: string, active: boolean) => void }) {
  const [price, setPrice] = useState((p.price_monthly / 100).toFixed(2));
  return (
    <tr><td><b>{p.code}</b></td><td>{p.vcpu}</td><td>{ramGB(p.ram_mb)}</td><td>{p.disk_gb} GB</td><td>{p.bandwidth_mbit} Mbit/s</td>
      <td><span style={{ display: "flex", gap: 8 }}><input className="input" style={{ width: 120 }} type="number" step="0.01" value={price} onChange={(e) => setPrice(e.target.value)} aria-label={`${p.code} bahasy`} />
        <button type="button" className="btn btn-ghost" style={{ minHeight: 40 }} onClick={() => onSave(p, price, p.is_active)}>Ýatda sakla</button></span></td>
      <td><button type="button" className="btn btn-ghost" style={{ minHeight: 34 }} onClick={() => onSave(p, price, !p.is_active)}>{p.is_active ? "Işjeň" : "Öçük"}</button></td></tr>
  );
}

function AuditTab() {
  const [list, setList] = useState<Audit[]>([]);
  useEffect(() => { api<Audit[]>("/admin/audit?limit=100").then(setList); }, []);
  return (
    <section className="card"><div className="table-wrap"><table style={{ minWidth: 800 }}>
      <thead><tr><th>Wagty</th><th>Kim</th><th>Hereket</th><th>Obýekt</th><th>Jikme-jiklik</th><th>IP</th></tr></thead>
      <tbody>{list.map((l) => (
        <tr key={l.id}><td className="muted">{dt(l.created_at)}</td><td>{l.actor || "—"}</td><td className="mono">{l.action}</td><td>{l.target}</td>
          <td className="mono" style={{ fontSize: 12 }}>{Object.keys(l.meta).length ? JSON.stringify(l.meta) : ""}</td><td className="mono">{l.ip}</td></tr>))}</tbody>
    </table></div>
      <p className="hint" style={{ padding: "0 16px 16px", margin: 0 }}><Link href="/admin">Ähli amallar</Link> audit žurnalynda saklanýar.</p>
    </section>
  );
}
