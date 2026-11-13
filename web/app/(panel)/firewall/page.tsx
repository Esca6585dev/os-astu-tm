"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useCallback, useEffect, useMemo, useState } from "react";
import { Badge, PageHead, useCrumbs, usePanel } from "@/components/Shell";
import { Icon } from "@/components/icons";
import { api, ApiError, type Firewall, type FWRule, type Server } from "@/lib/api";

const PRESETS: { label: string; rule: FWRule }[] = [
  { label: "SSH (22)", rule: { direction: "in", action: "ACCEPT", proto: "tcp", dport: "22", addr: "", comment: "SSH" } },
  { label: "HTTP (80)", rule: { direction: "in", action: "ACCEPT", proto: "tcp", dport: "80", addr: "", comment: "HTTP" } },
  { label: "HTTPS (443)", rule: { direction: "in", action: "ACCEPT", proto: "tcp", dport: "443", addr: "", comment: "HTTPS" } },
  { label: "Ping (ICMP)", rule: { direction: "in", action: "ACCEPT", proto: "icmp", dport: "", addr: "", comment: "Ping" } },
  { label: "MySQL (3306)", rule: { direction: "in", action: "ACCEPT", proto: "tcp", dport: "3306", addr: "", comment: "MySQL" } },
  { label: "PostgreSQL (5432)", rule: { direction: "in", action: "ACCEPT", proto: "tcp", dport: "5432", addr: "", comment: "PostgreSQL" } },
];
const EMPTY: FWRule = { direction: "in", action: "ACCEPT", proto: "tcp", dport: "", addr: "", comment: "" };

const ruleText = (r: FWRule) =>
  `${r.direction === "in" ? "Giriş" : "Çykyş"} · ${r.action === "ACCEPT" ? "Rugsat" : "Blokla"} · ${r.proto ? r.proto.toUpperCase() : "Hemmesi"}${r.dport ? " " + r.dport : ""}${r.addr ? " · " + r.addr : ""}`;

/** SSH (22) giriş arkaly açykmy: DROP syýasatynda maşyna girip bolmazlygyň öňüni almak üçin */
function sshOpen(policy: string, rules: FWRule[]) {
  for (const r of rules) {
    if (r.direction !== "in" || r.addr) continue;
    if (r.proto !== "" && r.proto !== "tcp") continue;
    const ports = r.dport.split(",").filter(Boolean);
    const hits = ports.length === 0 || ports.some((p) => {
      const [lo, hi] = p.split(":").map(Number);
      return hi ? lo <= 22 && 22 <= hi : lo === 22;
    });
    if (hits) return r.action === "ACCEPT";
  }
  return policy === "ACCEPT";
}

export default function Page() {
  return <Suspense fallback={<p className="empty">Ýüklenýär…</p>}><FirewallPage /></Suspense>;
}

function FirewallPage() {
  useCrumbs("Tor", "Firewall");
  const { toast } = usePanel();
  const router = useRouter();
  const params = useSearchParams();
  const [servers, setServers] = useState<Server[] | null>(null);
  const [fw, setFw] = useState<Firewall | null>(null);
  const [policy, setPolicy] = useState<"ACCEPT" | "DROP">("ACCEPT");
  const [rules, setRules] = useState<FWRule[]>([]);
  const [err, setErr] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => { api<Server[]>("/servers").then(setServers).catch(() => setServers([])); }, []);
  const sid = Number(params.get("server")) || servers?.[0]?.id || 0;

  const load = useCallback(async (reset: boolean) => {
    if (!sid) return;
    const f = await api<Firewall>(`/servers/${sid}/firewall`);
    setFw(f);
    if (reset) { setPolicy(f.policy_in); setRules(f.rules); setErr(""); }
    return f;
  }, [sid]);
  useEffect(() => { setFw(null); load(true).catch(() => setFw(null)); }, [load]);

  // Ýatda saklanandan soň Proxmox-a ýazylýança ýagdaýy soraýarys
  useEffect(() => {
    if (!fw?.busy) return;
    const i = setInterval(() => load(false).then((f) => {
      if (f && !f.busy) toast(f.last_job_error ? `Firewall ulanylmady: ${f.last_job_error}` : "Firewall düzgünleri ulanyldy");
    }), 2000);
    return () => clearInterval(i);
  }, [fw?.busy, load, toast]);

  const dirty = useMemo(() => !!fw && (policy !== fw.policy_in || JSON.stringify(rules) !== JSON.stringify(fw.rules)), [fw, policy, rules]);
  const set = (i: number, patch: Partial<FWRule>) => setRules((rs) => rs.map((r, j) => {
    if (j !== i) return r;
    const n = { ...r, ...patch };
    if (n.proto !== "tcp" && n.proto !== "udp") n.dport = "";
    return n;
  }));
  const move = (i: number, d: number) => setRules((rs) => {
    const a = [...rs]; const j = i + d;
    if (j < 0 || j >= a.length) return rs;
    [a[i], a[j]] = [a[j], a[i]]; return a;
  });
  const add = (r: FWRule) => setRules((rs) => [...rs, { ...r }]);

  async function save() {
    setErr(""); setSaving(true);
    try {
      await api(`/servers/${sid}/firewall`, { method: "PUT", body: { policy_in: policy, rules } });
      toast("Ýatda saklandy, maşyna ýazylýar…");
      await load(true);
    } catch (e) { setErr(e instanceof ApiError ? e.message : "Ýalňyşlyk"); }
    finally { setSaving(false); }
  }

  if (servers === null) return <p className="empty">Ýüklenýär…</p>;
  if (servers.length === 0) return (
    <><PageHead title="Firewall" sub="Maşynlaryňyza gelýän we olardan çykýan trafigi dolandyryň" />
      <div className="card empty">Entek maşyn ýok. <Link href="/machines/new">Maşyn dörediň</Link></div></>
  );

  const locked = !fw || !fw.editable || fw.busy || saving;
  const sshWarn = !sshOpen(policy, rules);

  return (
    <>
      <PageHead title="Firewall" sub="Maşyna gelýän we ondan çykýan trafigi dolandyryň. Düzgünler ýokardan aşak barlanýar, ilkinji gabat gelen düzgün ulanylýar.">
        <label style={{ display: "flex", alignItems: "center", gap: 8 }}>Maşyn
          <select className="select" value={sid} onChange={(e) => router.replace(`/firewall?server=${e.target.value}`)}>
            {servers.map((s) => <option key={s.id} value={s.id}>{s.hostname}{s.ip ? ` · ${s.ip}` : ""}</option>)}
          </select></label>
      </PageHead>

      {!fw ? <p className="empty">Ýüklenýär…</p> : (
        <>
          {!fw.editable && <div className="alert alert-warn"><Icon name="alert" /><span>Firewall-y diňe işleýän ýa-da durdurylan maşynda üýtgedip bolýar. Häzir: <Badge status={fw.status} /></span></div>}
          {fw.busy && <div className="alert alert-info"><Icon name="refresh" /><span>Düzgünler maşyna ýazylýar…</span></div>}
          {!fw.busy && fw.last_job_error && <div className="alert alert-warn"><Icon name="alert" /><span>Soňky amal şowsuz boldy: {fw.last_job_error}</span></div>}

          <section className="card card-pad" style={{ display: "flex", flexWrap: "wrap", alignItems: "center", justifyContent: "space-between", gap: 16 }}>
            <div style={{ flex: "1 1 320px" }}>
              <h2 className="step-title">Gelýän trafik</h2>
              <p className="muted" style={{ margin: "4px 0 0" }}>{policy === "ACCEPT"
                ? "Ähli portlar açyk. Aşakdaky düzgünler bilen käbir portlary ýa-da salgylary ýapyp bilersiňiz."
                : "Diňe aşakda rugsat berlen portlar açyk, galanlary ýapyk. Bu has howpsuz."}</p>
            </div>
            <div className="segmented" role="radiogroup" aria-label="Gelýän trafik syýasaty">
              <button type="button" role="radio" aria-checked={policy === "ACCEPT"} disabled={locked} onClick={() => setPolicy("ACCEPT")}>Hemmesine rugsat</button>
              <button type="button" role="radio" aria-checked={policy === "DROP"} disabled={locked} onClick={() => setPolicy("DROP")}>Diňe rugsat berlenler</button>
            </div>
          </section>

          {sshWarn && (
            <div className="alert alert-warn"><Icon name="alert" /><span>SSH (22-nji port) ýapyk bolar: maşyna SSH bilen girip bilmersiňiz.</span>
              <button type="button" className="btn btn-ghost" disabled={locked} onClick={() => setRules((rs) => [{ ...PRESETS[0].rule }, ...rs])}>SSH-a rugsat ber</button></div>
          )}

          <section className="card">
            <div className="card-head"><h2>Düzgünler</h2><span className="muted">{rules.length} / 50</span></div>
            <div className="table-wrap"><table className="fw-table" style={{ minWidth: 900 }}>
              <thead><tr><th style={{ width: 70 }}>Tertip</th><th>Ugur</th><th>Hereket</th><th>Protokol</th><th>Port</th><th>Salgy (IP / CIDR)</th><th>Bellik</th><th><span className="sr-only">Poz</span></th></tr></thead>
              <tbody>
                {fw.system_rules.map((r, i) => (
                  <tr key={"sys" + i}><td className="sys"><span title="Paneliň düzgüni, üýtgedip bolmaýar">🔒</span></td>
                    <td className="sys" colSpan={6}>{ruleText(r)} — {r.comment}. Spam iberilmez ýaly ähli maşynlarda ýapyk.</td><td /></tr>
                ))}
                {rules.length === 0 && <tr><td colSpan={8} className="empty">Düzgün ýok. Aşakdaky düwmeler bilen goşuň.</td></tr>}
                {rules.map((r, i) => (
                  <tr key={i}>
                    <td style={{ whiteSpace: "nowrap" }}>
                      <button type="button" className="icon-btn" aria-label={`${i + 1}-nji düzgüni ýokary göçür`} disabled={locked || i === 0} onClick={() => move(i, -1)}>↑</button>
                      <button type="button" className="icon-btn" aria-label={`${i + 1}-nji düzgüni aşak göçür`} disabled={locked || i === rules.length - 1} onClick={() => move(i, 1)}>↓</button>
                    </td>
                    <td><select className="select" aria-label="Ugur" value={r.direction} disabled={locked} onChange={(e) => set(i, { direction: e.target.value as FWRule["direction"] })}>
                      <option value="in">Giriş</option><option value="out">Çykyş</option></select></td>
                    <td><select className="select" aria-label="Hereket" value={r.action} disabled={locked} onChange={(e) => set(i, { action: e.target.value as FWRule["action"] })}
                      style={{ color: r.action === "ACCEPT" ? "var(--ok)" : "var(--danger)", fontWeight: 600 }}>
                      <option value="ACCEPT">Rugsat</option><option value="DROP">Blokla</option></select></td>
                    <td><select className="select" aria-label="Protokol" value={r.proto} disabled={locked} onChange={(e) => set(i, { proto: e.target.value as FWRule["proto"] })}>
                      <option value="tcp">TCP</option><option value="udp">UDP</option><option value="icmp">ICMP</option><option value="">Hemmesi</option></select></td>
                    <td><input className="input mono" aria-label="Port" value={r.dport} disabled={locked || (r.proto !== "tcp" && r.proto !== "udp")}
                      placeholder={r.proto === "tcp" || r.proto === "udp" ? "22, 80,443, 8000:8100" : "—"} onChange={(e) => set(i, { dport: e.target.value })} /></td>
                    <td><input className="input mono" aria-label="Salgy" value={r.addr} disabled={locked} placeholder="islendik" onChange={(e) => set(i, { addr: e.target.value })} /></td>
                    <td><input className="input" aria-label="Bellik" value={r.comment} maxLength={60} disabled={locked} onChange={(e) => set(i, { comment: e.target.value })} /></td>
                    <td className="num"><button type="button" className="icon-btn" aria-label={`${i + 1}-nji düzgüni poz`} disabled={locked} onClick={() => setRules((rs) => rs.filter((_, j) => j !== i))}><Icon name="trash" small /></button></td>
                  </tr>
                ))}
              </tbody>
            </table></div>
            <div style={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: 12, padding: "14px 16px", borderTop: "1px solid var(--border-soft)" }}>
              <button type="button" className="btn btn-ghost" disabled={locked || rules.length >= 50} onClick={() => add(EMPTY)}><Icon name="plus" small sw={2.4} />Düzgün goş</button>
              <div className="chips" aria-label="Taýýar düzgünler">
                {PRESETS.map((p) => <button key={p.label} type="button" className="chip" disabled={locked || rules.length >= 50} onClick={() => add(p.rule)}>+ {p.label}</button>)}
              </div>
            </div>
          </section>

          {err && <p className="form-error" role="alert">{err}</p>}
          <div style={{ display: "flex", flexWrap: "wrap", gap: 12, alignItems: "center" }}>
            <button type="button" className="btn btn-lg" disabled={locked || !dirty} onClick={save}><Icon name="check" small sw={2.4} />Ýatda sakla we ulan</button>
            <button type="button" className="btn btn-ghost btn-lg" disabled={locked || !dirty} onClick={() => { setPolicy(fw.policy_in); setRules(fw.rules); setErr(""); }}>Ýatyr</button>
            {dirty && <span className="muted">Üýtgeşmeler entek ýatda saklanmady</span>}
          </div>
          <p className="hint" style={{ margin: 0 }}>«Salgy» giriş düzgüninde trafigiň nireden gelýändigini, çykyş düzgüninde nirä gidýändigini aňladýar. Boş bolsa islendik salgy. Açyk birikmeler (mysal üçin, häzirki SSH sessiýasy) üzülmeýär.</p>
        </>
      )}
    </>
  );
}
