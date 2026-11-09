"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { Badge, PageHead, useCrumbs, usePanel } from "@/components/Shell";
import { Icon } from "@/components/icons";
import { api, dt, money, tmt, type Balance, type Server, type Tx } from "@/lib/api";

export default function Dashboard() {
  useCrumbs("Baş sahypa");
  const { me } = usePanel();
  const [servers, setServers] = useState<Server[] | null>(null);
  const [bal, setBal] = useState<Balance | null>(null);
  const [txs, setTxs] = useState<Tx[]>([]);

  useEffect(() => {
    api<Server[]>("/servers").then(setServers).catch(() => setServers([]));
    api<Balance>("/me/balance").then(setBal).catch(() => {});
    api<Tx[]>("/me/transactions?limit=5").then(setTxs).catch(() => {});
  }, []);

  const running = servers?.filter((s) => s.status === "running").length ?? 0;
  return (
    <>
      <PageHead title={`Hoş geldiňiz${me.full_name ? ", " + me.full_name.split(" ")[0] : ""}`} sub="Maşynlaryňyzyň we balansyňyzyň gysgaça ýagdaýy">
        <Link className="btn" href="/machines/new"><Icon name="plus" small sw={2.4} />Täze maşyn</Link>
      </PageHead>
      <section className="grid grid-stats" aria-label="Görkezijiler">
        <div className="card stat"><span className="label">Balans</span><span className="value">{money(me.balance)}</span><Link href="/billing">Doldurmak →</Link></div>
        <div className="card stat"><span className="label">Işjeň maşynlar</span><span className="value">{running} <small>/ {servers?.length ?? 0}</small></span><span className="muted">{(servers?.length ?? 0) - running} başga ýagdaýda</span></div>
        <div className="card stat"><span className="label">Häzirki tutum</span><span className="value">{bal ? tmt(bal.hourly) : "—"} <small>TMT / sag</small></span><span className="muted">Aýlyk çäk {bal ? money(bal.monthly_cap) : "—"}</span></div>
        <div className="card stat"><span className="label">Şu aý tutulan</span><span className="value">{bal ? money(bal.month_used) : "—"}</span><span className="muted">{bal?.days_left != null ? `Balans ≈ ${bal.days_left} gün ýeter` : "Tölegli maşyn ýok"}</span></div>
      </section>
      {me.balance <= 0 && (servers?.length ?? 0) > 0 ? (
        <div className="alert alert-warn"><Icon name="alert" /><span>Balansyňyz gutardy. Maşynlar saklanýar we 7 günden soň pozulýar.</span><Link href="/billing"><b>Balansy doldur</b></Link></div>
      ) : (
        <div className="alert alert-info"><Icon name="info" /><span>Balans 0 bolanda maşynlar awtomatik durdurylýar, 7 günden soň pozulýar.</span></div>
      )}
      <div className="split">
        <section className="card main-col">
          <div className="card-head"><h2>Maşynlarym</h2><Link href="/machines">Hemmesi →</Link></div>
          <div className="table-wrap">
            <table>
              <thead><tr><th>Ady</th><th>IP salgy</th><th>Konfigurasiýa</th><th>Ýagdaýy</th></tr></thead>
              <tbody>
                {servers === null ? <tr><td colSpan={4} className="empty">Ýüklenýär…</td></tr> :
                  servers.length === 0 ? <tr><td colSpan={4} className="empty">Entek maşyn ýok. <Link href="/machines/new">Ilkinji maşynyňyzy dörediň</Link></td></tr> :
                    servers.slice(0, 5).map((s) => (
                      <tr key={s.id}>
                        <td><Link href={`/machines/${s.id}`}><b>{s.hostname}</b></Link><div className="muted" style={{ fontSize: 12 }}>{s.image.slug}</div></td>
                        <td className="mono">{s.ip || "—"}</td><td>{s.plan.code}</td><td><Badge status={s.status} /></td>
                      </tr>
                    ))}
              </tbody>
            </table>
          </div>
        </section>
        <section className="card side-col">
          <div className="card-head"><h2>Soňky amallar</h2><Link href="/billing">Taryh →</Link></div>
          {txs.length === 0 && <p className="empty">Amal ýok</p>}
          {txs.map((t) => (
            <div key={t.id} style={{ display: "flex", justifyContent: "space-between", gap: 12, padding: "12px 20px", borderTop: "1px solid var(--border-soft)" }}>
              <div><div style={{ fontWeight: 500 }}>{t.note || t.type}</div><div className="muted" style={{ fontSize: 12 }}>{dt(t.created_at)}</div></div>
              <b style={{ color: t.amount > 0 ? "var(--ok)" : undefined }}>{t.amount > 0 ? "+" : ""}{tmt(t.amount)}</b>
            </div>
          ))}
        </section>
      </div>
    </>
  );
}
