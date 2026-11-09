"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { Badge, PageHead, useCopy, useCrumbs, usePanel } from "@/components/Shell";
import { Icon } from "@/components/icons";
import { api, dt, tmt, type Server } from "@/lib/api";

export default function Machines() {
  useCrumbs("Hasaplaýyş", "Maşynlar");
  const { toast } = usePanel();
  const copy = useCopy();
  const [list, setList] = useState<Server[] | null>(null);
  const [q, setQ] = useState("");
  const [st, setSt] = useState("");
  const [size, setSize] = useState(10);
  const [page, setPage] = useState(1);

  const load = useCallback(() => api<Server[]>("/servers").then(setList).catch(() => setList([])), []);
  useEffect(() => { load(); }, [load]);
  // Amal dowam edýän maşyn bar bolsa, sanaw her 4 sekuntda täzelenýär
  useEffect(() => {
    if (!list?.some((s) => s.busy)) return;
    const i = setInterval(load, 4000);
    return () => clearInterval(i);
  }, [list, load]);

  const all = list ?? [];
  const filtered = all.filter((s) =>
    (!q || s.hostname.includes(q.toLowerCase()) || (s.ip || "").includes(q)) && (!st || s.status === st));
  const pages = Math.max(1, Math.ceil(filtered.length / size));
  const shown = filtered.slice((page - 1) * size, page * size);
  const hourly = all.filter((s) => ["running", "stopped"].includes(s.status)).reduce((a, s) => a + s.plan.price_monthly / 720, 0);

  return (
    <>
      <PageHead title="Maşynlar" sub={`${all.length} maşyn · ${all.filter((s) => s.status === "running").length} işjeň · jemi ${tmt(Math.round(hourly))} TMT / sag`}>
        <button type="button" className="btn btn-ghost btn-icon" aria-label="Täzele" onClick={() => load().then(() => toast("Sanaw täzelendi"))}><Icon name="refresh" /></button>
        <Link className="btn" href="/machines/new"><Icon name="plus" small sw={2.4} />Täze maşyn</Link>
      </PageHead>
      <section className="card">
        <div className="toolbar">
          <label className="search"><Icon name="search" small /><span className="sr-only">Gözleg</span>
            <input type="search" value={q} onChange={(e) => { setQ(e.target.value); setPage(1); }} placeholder="Ady ýa-da IP boýunça gözle" /></label>
          <label style={{ display: "flex", alignItems: "center", gap: 8 }}>Ýagdaýy
            <select className="select" value={st} onChange={(e) => { setSt(e.target.value); setPage(1); }}>
              <option value="">Hemmesi</option><option value="running">Işjeň</option><option value="stopped">Durdurylan</option>
              <option value="suspended">Saklanan</option><option value="provisioning">Döredilýär</option><option value="error">Ýalňyşlyk</option>
            </select></label>
        </div>
        <div className="table-wrap">
          <table style={{ minWidth: 920 }}>
            <thead><tr><th>Ady</th><th>Obraz</th><th>IP salgy</th><th>Konfigurasiýa</th><th>Ýagdaýy</th><th>Döredildi</th><th><span className="sr-only">Hereketler</span></th></tr></thead>
            <tbody>
              {list === null ? <tr><td colSpan={7} className="empty">Ýüklenýär…</td></tr> :
                shown.length === 0 ? <tr><td colSpan={7} className="empty">{all.length ? "Hiç zat tapylmady" : "Entek maşyn ýok"}</td></tr> :
                  shown.map((s) => (
                    <tr key={s.id}>
                      <td><Link href={`/machines/${s.id}`}><b>{s.hostname}</b></Link></td>
                      <td>{s.image.slug}</td>
                      <td>{s.ip ? <span style={{ display: "inline-flex", alignItems: "center", gap: 6 }}><span className="mono">{s.ip}</span>
                        <button type="button" className="icon-btn" aria-label={`${s.ip} göçür`} onClick={() => copy(s.ip!)}><Icon name="copy" small /></button></span> : "—"}</td>
                      <td><Link href="/plans">{s.plan.code}</Link></td>
                      <td><Badge status={s.status} /></td>
                      <td className="muted" style={{ fontVariantNumeric: "tabular-nums" }}>{dt(s.created_at)}</td>
                      <td className="num"><Link className="btn btn-ghost" style={{ minHeight: 34 }} href={`/machines/${s.id}`}>Dolandyr</Link></td>
                    </tr>
                  ))}
            </tbody>
          </table>
        </div>
        <div className="pager">
          <label style={{ display: "flex", alignItems: "center", gap: 8 }}>Görkez
            <select className="select" value={size} onChange={(e) => { setSize(+e.target.value); setPage(1); }}><option>10</option><option>25</option><option>50</option></select></label>
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <span>{filtered.length ? `${(page - 1) * size + 1}–${Math.min(page * size, filtered.length)} / ${filtered.length}` : "0"}</span>
            <button type="button" className="btn btn-ghost btn-icon" aria-label="Öňki sahypa" disabled={page <= 1} onClick={() => setPage(page - 1)}>‹</button>
            <button type="button" className="btn btn-ghost btn-icon" aria-label="Indiki sahypa" disabled={page >= pages} onClick={() => setPage(page + 1)}>›</button>
          </div>
        </div>
      </section>
      <p className="muted" style={{ margin: 0 }}>Durdurylan maşyn hem tölegli: disk we IP salgy siz üçin saklanýar. Tutumy togtatmak üçin maşyny pozuň.</p>
      <Link className="fab" href="/machines/new" aria-label="Täze maşyn döret"><Icon name="plus" sw={2.4} /></Link>
    </>
  );
}
