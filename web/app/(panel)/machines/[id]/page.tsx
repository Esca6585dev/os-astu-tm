"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { Badge, useCopy, useCrumbs, usePanel } from "@/components/Shell";
import { Icon } from "@/components/icons";
import { api, ApiError, bytes, dt, hourly, money, ramGB, type Image, type Server } from "@/lib/api";

export default function MachinePage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const { toast, refresh: refreshMe } = usePanel();
  const copy = useCopy();
  const [s, setS] = useState<Server | null>(null);
  const [notFound, setNotFound] = useState(false);
  const [images, setImages] = useState<Image[]>([]);
  const [reimg, setReimg] = useState(0);
  const [pw, setPw] = useState("");
  const [confirmName, setConfirmName] = useState("");
  useCrumbs("Hasaplaýyş", "Maşynlar", s?.hostname ?? "…");

  const load = useCallback(() =>
    api<Server>(`/servers/${id}`).then(setS).catch((e) => { if (e instanceof ApiError && e.status === 404) setNotFound(true); }), [id]);
  useEffect(() => { load(); api<Image[]>("/images").then((l) => { setImages(l); if (l[0]) setReimg(l[0].id); }); }, [load]);
  // Amal dowam edýärkä ýagdaýy her 3 sekuntda soraýarys
  useEffect(() => {
    if (!s) return;
    const i = setInterval(load, s.busy || ["provisioning", "reinstalling", "deleting"].includes(s.status) ? 3000 : 15000);
    return () => clearInterval(i);
  }, [s, load]);

  async function act(action: string, body?: unknown, okMsg?: string) {
    try {
      const r = await api<{ root_password?: string }>(`/servers/${id}/${action}`, { method: "POST", body: body ?? {} });
      if (r.root_password) setPw(r.root_password);
      toast(okMsg || "Amal başlandy");
      load();
    } catch (e) {
      toast(e instanceof ApiError ? e.message : "Ýalňyşlyk");
    }
  }

  async function remove() {
    try {
      await api(`/servers/${id}`, { method: "DELETE" });
      toast("Maşyn pozulýar");
      refreshMe();
      router.push("/machines");
    } catch (e) {
      toast(e instanceof ApiError ? e.message : "Ýalňyşlyk");
    }
  }

  if (notFound) return <div className="empty"><h1>Maşyn tapylmady</h1><Link href="/machines">← Maşynlar</Link></div>;
  if (!s) return <p className="empty">Ýüklenýär…</p>;

  const p = s.plan;
  const u = s.usage;
  const on = s.status === "running";
  const locked = s.busy || s.status === "suspended";
  const meter = (label: string, val: string, pct: number) => (
    <div className="meter"><div className="top"><b>{label}</b><span className="muted">{val}</span></div>
      <div className="bar"><span style={{ width: `${Math.min(100, Math.max(0, pct))}%` }} /></div></div>
  );

  return (
    <>
      <div className="page-head">
        <div>
          <Link href="/machines">← Maşynlar</Link>
          <div style={{ display: "flex", alignItems: "center", gap: 12, marginTop: 6 }}><h1>{s.hostname}</h1><Badge status={s.status} />{s.busy && <span className="muted">amal dowam edýär…</span>}</div>
          <p>{s.image.name} {s.image.version} · {p.code} · {p.vcpu} CPU · {ramGB(p.ram_mb)} RAM · {p.disk_gb} GB HDD · {p.bandwidth_mbit} Mbit/s</p>
        </div>
        <div className="row-actions">
          <button type="button" className="btn btn-ghost" disabled={locked || !["running", "stopped"].includes(s.status)}
            onClick={() => act(on ? "stop" : "start", undefined, on ? "Maşyn durdurylýar" : "Maşyn başladylýar")}><Icon name="power" small />{on ? "Durdur" : "Başlat"}</button>
          <button type="button" className="btn btn-ghost" disabled={locked || !on} onClick={() => act("reboot", undefined, "Maşyn täzeden ýüklenýär")}><Icon name="refresh" small />Täzeden ýükle</button>
          <button type="button" className="btn btn-ghost" disabled title="noVNC konsoly soňky tapgyrda goşulýar"><Icon name="terminal" small />Konsol</button>
        </div>
      </div>

      {s.status === "suspended" && <div className="alert alert-warn"><Icon name="alert" /><span>Maşyn balans gutaranlygy sebäpli saklanýar. Balans doldurylanda awtomatik başlar.</span><Link href="/billing"><b>Balans</b></Link></div>}
      {s.status === "error" && <div className="alert alert-warn"><Icon name="alert" /><span>Maşyny döretmekde ýalňyşlyk boldy{s.last_job_error ? `: ${s.last_job_error}` : ""}. OS-i täzeden guruň ýa-da maşyny pozuň.</span></div>}
      {pw && (
        <div className="alert alert-info pw-box"><span>Täze root parol (diňe bir gezek görkezilýär): <b className="mono">{pw}</b></span>
          <button type="button" className="btn btn-ghost" onClick={() => copy(pw)}><Icon name="copy" small />Göçür</button></div>
      )}

      {s.ip && (
        <section className="ssh-box">
          <div><div className="label">SSH bilen birikmek</div><code>ssh root@{s.ip}</code></div>
          <button type="button" className="btn btn-ghost" onClick={() => copy(`ssh root@${s.ip}`)}><Icon name="copy" small />Göçür</button>
        </section>
      )}

      <div className="split">
        <section className="card card-pad" style={{ flex: "1 1 360px" }}>
          <h2 className="step-title" style={{ marginBottom: 16 }}>Maglumat</h2>
          <dl className="kv">
            <dt>IPv4</dt><dd className="mono">{s.ip || "—"}</dd>
            <dt>Şlýuz</dt><dd className="mono">{s.gateway || "—"}</dd>
            <dt>Ulanyjy</dt><dd>root</dd>
            <dt>Node</dt><dd>{s.node}{s.vmid ? ` · VM ${s.vmid}` : ""}</dd>
            <dt>Döredildi</dt><dd>{dt(s.created_at)}</dd>
            <dt>Baha</dt><dd>{money(p.price_monthly)} / aý · ≈ {hourly(p)} TMT / sag</dd>
            <dt>Şu aý tutulan</dt><dd>{money(s.month_used)}</dd>
          </dl>
        </section>
        <section className="card card-pad" style={{ flex: "1 1 360px", display: "flex", flexDirection: "column", gap: 18 }}>
          <h2 className="step-title">Häzirki ulanyş</h2>
          {u && u.status === "running" ? (
            <>
              {meter("CPU", `${(u.cpu * 100).toFixed(0)}%`, u.cpu * 100)}
              {meter("RAM", `${bytes(u.mem_used)} / ${bytes(u.mem_max)}`, (u.mem_used / (u.mem_max || 1)) * 100)}
              <div className="kv" style={{ display: "grid" }}>
                <span className="muted">Disk</span><span>{bytes(u.disk_max)}</span>
                <span className="muted">Tor (giriş / çykyş)</span><span>{bytes(u.net_in)} / {bytes(u.net_out)}</span>
                <span className="muted">Işleýän wagty</span><span>{Math.floor(u.uptime / 3600)} sag {Math.floor((u.uptime % 3600) / 60)} min</span>
              </div>
            </>
          ) : <p className="muted" style={{ margin: 0 }}>Maşyn işlemeýär ýa-da maglumat elýeterli däl.</p>}
        </section>
      </div>

      <div className="split">
        <section className="card card-pad" style={{ flex: "1 1 360px", display: "flex", flexDirection: "column", gap: 14 }}>
          <h2 className="step-title">OS-i täzeden gurmak</h2>
          <p className="muted" style={{ margin: 0 }}>Diskdäki ähli maglumatlar pozular. IP salgy we SSH açarlar üýtgemeýär.</p>
          <label className="field">Obraz
            <select className="select" value={reimg} onChange={(e) => setReimg(+e.target.value)}>
              {images.map((i) => <option key={i.id} value={i.id}>{i.name} {i.version}</option>)}
            </select></label>
          <button type="button" className="btn btn-ghost" style={{ alignSelf: "flex-start" }} disabled={locked}
            onClick={() => window.confirm("Diskdäki ähli maglumatlar pozular. Dowam edilsinmi?") && act("reinstall", { image_id: reimg }, "OS täzeden gurulýar")}>Täzeden gur</button>
        </section>
        <section className="card card-pad" style={{ flex: "1 1 360px", display: "flex", flexDirection: "column", gap: 14 }}>
          <h2 className="step-title">Root paroly täzele</h2>
          <p className="muted" style={{ margin: 0 }}>Täze parol döredilip, diňe bir gezek görkeziler (qemu-guest-agent arkaly).</p>
          <button type="button" className="btn btn-ghost" style={{ alignSelf: "flex-start" }} disabled={locked || !on} onClick={() => act("reset-password", undefined, "Parol täzelenýär")}>Täze parol döret</button>
          <hr style={{ border: 0, borderTop: "1px solid var(--border-soft)", width: "100%", margin: "4px 0" }} />
          <h2 className="step-title" style={{ color: "var(--danger)" }}>Maşyny pozmak</h2>
          <p className="muted" style={{ margin: 0 }}>Maşyn we diski yzyna gaýtarylmazdan pozulýar. Tassyklamak üçin maşynyň adyny ýazyň.</p>
          <input className="input mono" placeholder={s.hostname} value={confirmName} onChange={(e) => setConfirmName(e.target.value)} aria-label="Maşynyň ady" />
          <button type="button" className="btn btn-danger" style={{ alignSelf: "flex-start" }} disabled={s.busy || confirmName !== s.hostname} onClick={remove}><Icon name="trash" small />Maşyny poz</button>
        </section>
      </div>
    </>
  );
}
