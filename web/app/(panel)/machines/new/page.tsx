"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { PageHead, useCrumbs, usePanel } from "@/components/Shell";
import { api, ApiError, hourly, money, ramGB, type Image, type Plan, type SSHKey } from "@/lib/api";

const HOST_RE = /^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$/;

export default function NewMachine() {
  useCrumbs("Hasaplaýyş", "Maşynlar", "Täze maşyn");
  const router = useRouter();
  const { me, toast } = usePanel();
  const [images, setImages] = useState<Image[]>([]);
  const [plans, setPlans] = useState<Plan[]>([]);
  const [keys, setKeys] = useState<SSHKey[]>([]);
  const [imageId, setImageId] = useState<number>(0);
  const [planId, setPlanId] = useState<number>(0);
  const [auth, setAuth] = useState<"key" | "pass">("key");
  const [keyIds, setKeyIds] = useState<number[]>([]);
  const [password, setPassword] = useState("");
  const [hostname, setHostname] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [created, setCreated] = useState<{ id: number; pw?: string } | null>(null);

  useEffect(() => {
    api<Image[]>("/images").then((l) => { setImages(l); if (l[0]) setImageId(l[0].id); });
    api<Plan[]>("/plans").then((l) => {
      setPlans(l);
      const pre = Number(new URLSearchParams(window.location.search).get("plan"));
      setPlanId(l.find((p) => p.id === pre)?.id ?? l.find((p) => p.is_popular)?.id ?? l[0]?.id ?? 0);
    });
    api<SSHKey[]>("/ssh-keys").then((l) => { setKeys(l); if (l[0]) setKeyIds([l[0].id]); else setAuth("pass"); });
  }, []);

  const plan = plans.find((p) => p.id === planId);
  const img = images.find((i) => i.id === imageId);
  useEffect(() => {
    if (img && plan && !hostname) setHostname(`${img.name.toLowerCase().replace(/[^a-z0-9]/g, "")}-${plan.code.replace(/^VPS\s+/, "").replace(/-\d+$/, "").toLowerCase()}-01`);
  }, [img, plan]); // eslint-disable-line react-hooks/exhaustive-deps

  const hostOk = HOST_RE.test(hostname);
  const need = plan ? Math.ceil((plan.price_monthly * 24) / 720) : 0;
  const enough = me.balance >= need;

  async function create() {
    if (!plan || !img) return;
    if (auth === "key" && keyIds.length === 0) { setErr("Azyndan bir SSH açar saýlaň"); return; }
    if (auth === "pass" && password && password.length < 12) { setErr("Root parol azyndan 12 simwol bolmaly"); return; }
    setErr(""); setBusy(true);
    try {
      const r = await api<{ id: number; root_password?: string }>("/servers", { method: "POST", body: {
        plan_id: plan.id, image_id: img.id, hostname,
        ssh_key_ids: auth === "key" ? keyIds : [], password: auth === "pass" ? password : "",
      } });
      if (r.root_password) setCreated({ id: r.id, pw: r.root_password });
      else { toast("Maşyn döredilýär…"); router.push(`/machines/${r.id}`); }
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Ýalňyşlyk");
    } finally {
      setBusy(false);
    }
  }

  if (created) {
    return (
      <section className="card card-pad" style={{ maxWidth: 560, display: "flex", flexDirection: "column", gap: 14 }}>
        <h1 className="step-title">Maşyn döredilýär</h1>
        <p style={{ margin: 0 }}>Root parol diňe şu ýerde bir gezek görkezilýär. Ony howpsuz ýerde saklaň.</p>
        <div className="alert alert-info pw-box"><b className="mono" style={{ fontSize: 16 }}>{created.pw}</b>
          <button type="button" className="btn btn-ghost" onClick={() => navigator.clipboard?.writeText(created.pw!).then(() => toast("Göçürildi"))}>Göçür</button></div>
        <Link className="btn" href={`/machines/${created.id}`} style={{ alignSelf: "flex-start" }}>Maşyna geç →</Link>
      </section>
    );
  }

  return (
    <div className="split">
      <div className="main-col" style={{ display: "flex", flexDirection: "column", gap: 20 }}>
        <PageHead title="Täze maşyn döret" sub="Obraz, konfigurasiýa we giriş usulyny saýlaň" />
        <section className="card card-pad" style={{ display: "flex", flexDirection: "column", gap: 16 }}>
          <h2 className="step-title"><b>1.</b> Obraz</h2>
          <div className="choices choices-os" role="radiogroup" aria-label="Obraz">
            {images.length === 0 && <p className="muted">Obraz ýok. Administrator Proxmox şablonyny goşmaly.</p>}
            {images.map((i) => (
              <button key={i.id} type="button" className="choice" role="radio" aria-checked={i.id === imageId} onClick={() => setImageId(i.id)}>
                <span className="t">{i.name}</span><span className="s">{i.version}</span>
              </button>
            ))}
          </div>
        </section>
        <section className="card card-pad" style={{ display: "flex", flexDirection: "column", gap: 16 }}>
          <h2 className="step-title"><b>2.</b> Konfigurasiýa</h2>
          <div className="choices choices-plan" role="radiogroup" aria-label="Konfigurasiýa">
            {plans.map((p) => (
              <button key={p.id} type="button" className="choice" role="radio" aria-checked={p.id === planId} onClick={() => setPlanId(p.id)}>
                <span className="s" style={{ fontWeight: 700, color: "var(--primary-dark)" }}>{p.code}</span>
                <span className="price">{(p.price_monthly / 100).toFixed(2)} <small>TMT / aý</small></span>
                <span className="s">≈ {hourly(p)} TMT / sag</span>
                <span className="specs">{`${p.vcpu} CPU\n${ramGB(p.ram_mb)} RAM\n${p.disk_gb} GB HDD\n${p.bandwidth_mbit} Mbit/s · ${p.ip_count} IP`}</span>
              </button>
            ))}
          </div>
        </section>
        <section className="card card-pad" style={{ display: "flex", flexDirection: "column", gap: 16 }}>
          <h2 className="step-title"><b>3.</b> Giriş usuly</h2>
          <div className="segmented" role="radiogroup" aria-label="Giriş usuly">
            <button type="button" role="radio" aria-checked={auth === "key"} onClick={() => setAuth("key")}>SSH açar</button>
            <button type="button" role="radio" aria-checked={auth === "pass"} onClick={() => setAuth("pass")}>Root parol</button>
          </div>
          {auth === "key" ? (
            <div>
              {keys.length === 0 && <p className="muted">SSH açaryňyz ýok.</p>}
              {keys.map((k) => (
                <label key={k.id} style={{ display: "flex", alignItems: "center", gap: 12, padding: "12px 14px", border: "1px solid #D0D5DD", borderRadius: 8, marginBottom: 8 }}>
                  <input type="checkbox" checked={keyIds.includes(k.id)} style={{ width: 18, height: 18 }}
                    onChange={(e) => setKeyIds(e.target.checked ? [...keyIds, k.id] : keyIds.filter((x) => x !== k.id))} />
                  <span><b>{k.name}</b><span className="mono muted" style={{ display: "block", fontSize: 12 }}>{k.fingerprint} · {k.type}</span></span>
                </label>
              ))}
              <Link href="/ssh-keys">+ Täze SSH açar goş</Link>
            </div>
          ) : (
            <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
              <label className="field" style={{ maxWidth: 420 }}>Root parol
                <input className="input" type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="Azyndan 12 simwol" autoComplete="new-password" /></label>
              <span className="hint">Boş goýsaňyz, parol awtomatik döredilip, bir gezek görkeziler.</span>
            </div>
          )}
        </section>
        <section className="card card-pad" style={{ display: "flex", flexDirection: "column", gap: 12 }}>
          <h2 className="step-title"><b>4.</b> Maşynyň ady</h2>
          <label className="field" style={{ maxWidth: 420 }}>Hostname
            <input className="input mono" value={hostname} maxLength={63} onChange={(e) => setHostname(e.target.value.toLowerCase())} aria-invalid={!hostOk} /></label>
          <span className="hint" style={{ color: hostOk ? undefined : "var(--danger)" }}>Diňe kiçi latyn harplary, sanlar we “-” belgisi.</span>
        </section>
      </div>
      <aside className="card card-pad side-col summary sticky" aria-label="Sargyt jemi" style={{ display: "flex", flexDirection: "column", gap: 16 }}>
        <h2 className="step-title">Jemi</h2>
        <dl>
          <dt>Obraz</dt><dd>{img ? `${img.name} ${img.version}` : "—"}</dd>
          <dt>Konfigurasiýa</dt><dd>{plan?.code ?? "—"}</dd>
          <dt>Resurslar</dt><dd>{plan ? `${plan.vcpu} CPU · ${ramGB(plan.ram_mb)} · ${plan.disk_gb} GB` : "—"}</dd>
          <dt>Tizlik</dt><dd>{plan ? `${plan.bandwidth_mbit} Mbit/s` : "—"}</dd>
          <dt>Giriş</dt><dd>{auth === "key" ? "SSH açar" : "Root parol"}</dd>
        </dl>
        <hr style={{ border: 0, borderTop: "1px solid var(--border-soft)", width: "100%", margin: 0 }} />
        <div style={{ display: "flex", alignItems: "baseline", justifyContent: "space-between" }}><span className="muted">Aýda iň köp</span><span className="total">{plan ? money(plan.price_monthly) : "—"}</span></div>
        <div style={{ display: "flex", justifyContent: "space-between" }}><span className="muted">Sagatda</span><b>≈ {plan ? hourly(plan) : "—"} TMT</b></div>
        <div style={{ display: "flex", justifyContent: "space-between" }}><span className="muted">Balans</span><b>{money(me.balance)}</b></div>
        {err && <p className="form-error" role="alert">{err}</p>}
        <button type="button" className="btn btn-lg btn-block" disabled={!plan || !img || !hostOk || !enough || busy || me.status !== "active"} onClick={create}>
          {busy ? "Döredilýär…" : "Maşyny döret"}</button>
        <p className="hint" style={{ margin: 0, color: enough ? undefined : "var(--danger)" }}>
          {enough ? "Töleg her sagat balansdan aýrylýar (aýlyk baha / 720)." : `Balans ýeterlik däl: azyndan ${money(need)} (24 sagat) gerek.`}</p>
      </aside>
    </div>
  );
}
