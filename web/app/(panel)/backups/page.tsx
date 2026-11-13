"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Suspense, useCallback, useEffect, useState } from "react";
import { PageHead, useCrumbs, usePanel } from "@/components/Shell";
import { Icon } from "@/components/icons";
import { api, ApiError, bytes, dt, type Backup, type Server } from "@/lib/api";

const B_STATUS: Record<string, [string, string]> = {
  creating: ["ALYNÝAR", "badge-info"], ready: ["TAÝÝAR", "badge-ok"], restoring: ["DIKELDILÝÄR", "badge-info"],
  deleting: ["POZULÝAR", "badge-warn"], error: ["ÝALŇYŞLYK", "badge-danger"],
};
const BUSY = ["creating", "restoring", "deleting"];

export default function Page() {
  return <Suspense fallback={<p className="empty">Ýüklenýär…</p>}><Backups /></Suspense>;
}

function Backups() {
  useCrumbs("Hasaplaýyş", "Bekaplar");
  const { toast } = usePanel();
  const params = useSearchParams();
  const [servers, setServers] = useState<Server[] | null>(null);
  const [list, setList] = useState<Backup[] | null>(null);
  const [limit, setLimit] = useState(3);
  const [sid, setSid] = useState(0);
  const [note, setNote] = useState("");
  const [sending, setSending] = useState(false);

  const load = useCallback(() => Promise.all([
    api<{ limit: number; backups: Backup[] }>("/backups").then((r) => { setList(r.backups); setLimit(r.limit); }),
    api<Server[]>("/servers").then(setServers),
  ]).catch(() => { setList((l) => l ?? []); setServers((s) => s ?? []); }), []);
  useEffect(() => { load(); }, [load]);

  // Saýlanan maşyn: ?server=ID ýa-da ilkinji bekap alyp boljak maşyn
  useEffect(() => {
    if (!servers || sid) return;
    const fromUrl = Number(params.get("server"));
    const ok = servers.filter((s) => ["running", "stopped"].includes(s.status));
    setSid(servers.some((s) => s.id === fromUrl) ? fromUrl : ok[0]?.id ?? servers[0]?.id ?? 0);
  }, [servers, sid, params]);

  // Amal dowam edýän bolsa sanaw her 3 sekuntda täzelenýär
  const working = !!list?.some((b) => BUSY.includes(b.status)) || !!servers?.some((s) => s.busy);
  useEffect(() => {
    if (!working) return;
    const i = setInterval(load, 3000);
    return () => clearInterval(i);
  }, [working, load]);

  const server = servers?.find((s) => s.id === sid);
  const forServer = (list ?? []).filter((b) => b.server_id === sid && b.status !== "error");
  const filter = Number(params.get("server")) || 0;
  const shown = (list ?? []).filter((b) => !filter || b.server_id === filter);
  const canCreate = !!server && ["running", "stopped"].includes(server.status) && !server.busy && forServer.length < limit;

  async function create(e: React.FormEvent) {
    e.preventDefault();
    if (!server) return;
    setSending(true);
    try {
      await api(`/servers/${server.id}/backups`, { method: "POST", body: { note } });
      setNote(""); toast(`${server.hostname}: bekap alynýar`); load();
    } catch (err) { toast(err instanceof ApiError ? err.message : "Ýalňyşlyk"); }
    finally { setSending(false); }
  }

  async function restore(b: Backup) {
    if (!window.confirm(`"${b.hostname}" maşyny ${dt(b.created_at)} ýagdaýyna gaýtarylar.\n\nŞondan soň ýazylan ähli maglumatlar ýiter. IP salgy we firewall üýtgemeýär. Dowam edilsinmi?`)) return;
    try { await api(`/backups/${b.id}/restore`, { method: "POST" }); toast("Maşyn bekapdan dikeldilýär"); load(); }
    catch (err) { toast(err instanceof ApiError ? err.message : "Ýalňyşlyk"); }
  }

  async function remove(b: Backup) {
    if (!window.confirm(`${b.hostname} · ${dt(b.created_at)} bekaby pozulsynmy? Bu hereketi yzyna gaýtaryp bolmaýar.`)) return;
    try { await api(`/backups/${b.id}`, { method: "DELETE" }); toast("Bekap pozulýar"); load(); }
    catch (err) { toast(err instanceof ApiError ? err.message : "Ýalňyşlyk"); }
  }

  return (
    <>
      <PageHead title="Bekaplar" sub={`Maşynyň diskiniň we sazlamalarynyň doly nusgasy. Her maşyn üçin ${limit} bekap mugt saklanýar.`}>
        <button type="button" className="btn btn-ghost btn-icon" aria-label="Täzele" onClick={() => load().then(() => toast("Sanaw täzelendi"))}><Icon name="refresh" /></button>
      </PageHead>

      <form className="card card-pad" onSubmit={create} style={{ display: "flex", flexDirection: "column", gap: 14 }}>
        <h2 className="step-title">Täze bekap al</h2>
        {servers !== null && servers.length === 0 ? (
          <p className="muted" style={{ margin: 0 }}>Maşyn ýok. <Link href="/machines/new">Maşyn dörediň</Link></p>
        ) : (
          <>
            <div className="inline-form" style={{ gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))" }}>
              <label className="field">Maşyn
                <select className="select" value={sid} onChange={(e) => setSid(+e.target.value)}>
                  {servers?.map((s) => <option key={s.id} value={s.id}>{s.hostname}{s.ip ? ` · ${s.ip}` : ""}</option>)}
                </select></label>
              <label className="field">Bellik (islege görä)
                <input className="input" value={note} maxLength={100} onChange={(e) => setNote(e.target.value)} placeholder="mysal: täzelenmeden öň" /></label>
              {server && (
                <div className="meter" style={{ paddingBottom: 6 }}>
                  <div className="top"><span className="muted">Bu maşynyň bekaplary</span><b>{forServer.length} / {limit}</b></div>
                  <div className="bar"><span className={forServer.length >= limit ? "warn" : ""} style={{ width: `${Math.min(100, (forServer.length / limit) * 100)}%` }} /></div>
                </div>
              )}
              <button className="btn" disabled={!canCreate || sending}><Icon name="backup" small />Bekap al</button>
            </div>
            {server && forServer.length >= limit && <p className="form-error">Çäk doldy: täze bekap üçin köne bekaplaryň birini pozuň.</p>}
            {server?.status === "suspended" && <p className="form-error">Maşyn balans gutaranlygy sebäpli öçürildi.</p>}
            {server && !["running", "stopped", "suspended"].includes(server.status) && <p className="form-error">Maşyn işleýän ýa-da durdurylan bolmaly.</p>}
            <span className="hint">Işleýän maşyn durdurylmaýar (snapshot). Bekap alynýança maşynda başga amal edip bolmaýar.</span>
          </>
        )}
      </form>

    <section className="card">
        <div className="card-head">
          <h2>{filter ? `${servers?.find((s) => s.id === filter)?.hostname ?? "…"} bekaplary` : "Ähli bekaplar"}</h2>
          {filter ? <Link href="/backups">Hemmesi →</Link> : null}
        </div>
        <div className="table-wrap"><table style={{ minWidth: 640 }}>
          <thead><tr><th>Maşyn</th><th>Bellik</th><th className="num">Ölçegi</th><th>Ýagdaýy</th><th>Alnan wagty</th><th><span className="sr-only">Hereketler</span></th></tr></thead>
          <tbody>
            {list === null ? <tr><td colSpan={6} className="empty">Ýüklenýär…</td></tr> :
              shown.length === 0 ? <tr><td colSpan={6} className="empty">Entek bekap ýok. Ýokardaky formadan ilkinji bekaby alyň.</td></tr> :
                shown.map((b) => {
                  const [label, cls] = B_STATUS[b.status] ?? [b.status, "badge-off"];
                  const srv = servers?.find((s) => s.id === b.server_id);
                  const locked = !!srv?.busy || srv?.status === "suspended";
                  return (
                    <tr key={b.id}>
                      <td><Link href={`/machines/${b.server_id}`}><b>{b.hostname}</b></Link><div className="muted" style={{ fontSize: 12, whiteSpace: "nowrap" }}>{b.image}</div></td>
                      <td>{b.note || <span className="muted">—</span>}</td>
                      <td className="num" style={{ whiteSpace: "nowrap" }}>{b.status === "ready" || b.status === "restoring" ? bytes(b.size_bytes) : "—"}</td>
                      <td><span className={`badge ${cls}`}>{label}</span>{b.error && <div className="form-error" style={{ fontSize: 12, maxWidth: 260 }}>{b.error}</div>}</td>
                      <td className="muted" style={{ fontVariantNumeric: "tabular-nums", whiteSpace: "nowrap" }}>{dt(b.created_at)}</td>
                      <td className="num" style={{ whiteSpace: "nowrap" }}>
                        {b.status === "ready" && <button type="button" className="btn btn-outline" style={{ minHeight: 34 }} disabled={locked} onClick={() => restore(b)}>Dikelt</button>}{" "}
                        {(b.status === "ready" || b.status === "error") && <button type="button" className="btn btn-danger" style={{ minHeight: 34 }} disabled={locked} onClick={() => remove(b)}>Poz</button>}
                      </td>
                    </tr>
                  );
                })}
          </tbody>
        </table></div>
      </section>

      <section className="card card-pad" style={{ display: "flex", flexDirection: "column", gap: 10 }}>
        <h2 className="step-title">Bekapdan dikeltmek nähili işleýär</h2>
        <ol style={{ margin: 0, paddingLeft: 20, display: "flex", flexDirection: "column", gap: 6 }}>
          <li>Maşyn durdurylýar we diski saýlanan bekabyň ýagdaýyna gaýtarylýar.</li>
          <li>IP salgy, SSH açarlar we firewall düzgünleri üýtgemeýär.</li>
          <li>Maşyn awtomatik başladylýar. Bekapdan soň ýazylan maglumatlar ýitýär.</li>
          <li>Maşyn pozulanda onuň ähli bekaplary hem pozulýar.</li>
        </ol>
      </section>
    </>
  );
}
