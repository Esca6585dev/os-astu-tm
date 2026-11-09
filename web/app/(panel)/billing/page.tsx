"use client";

import { useEffect, useState } from "react";
import { Badge, PageHead, useCopy, useCrumbs, usePanel } from "@/components/Shell";
import { Icon } from "@/components/icons";
import { api, dt, money, tmt, type Balance, type Server, type Tx } from "@/lib/api";

const TYPE: Record<string, React.ReactNode> = {
  charge: "Tutum",
  topup: <span className="badge badge-ok">Doldurmak</span>,
  refund: <span className="badge badge-info">Gaýtarmak</span>,
  adjustment: <span className="badge badge-off">Düzediş</span>,
};

export default function Billing() {
  useCrumbs("Maliýe", "Balans");
  const { me } = usePanel();
  const copy = useCopy();
  const [bal, setBal] = useState<Balance | null>(null);
  const [servers, setServers] = useState<Server[]>([]);
  const [txs, setTxs] = useState<Tx[]>([]);
  const [type, setType] = useState("");
  const [page, setPage] = useState(1);

  useEffect(() => { api<Balance>("/me/balance").then(setBal); api<Server[]>("/servers").then(setServers); }, []);
  useEffect(() => { api<Tx[]>(`/me/transactions?limit=25&page=${page}&type=${type}`).then(setTxs); }, [type, page]);

  return (
    <>
      <PageHead title="Balans" sub="Töleg her sagat balansdan awtomatik aýrylýar" />
      <div className="split">
        <section className="card card-pad" style={{ flex: "1 1 340px", background: "var(--primary-dark)", borderColor: "var(--primary-dark)", color: "#fff", display: "flex", flexDirection: "column", gap: 18 }}>
          <span style={{ color: "#C7D7FF" }}>Häzirki balans</span>
          <span style={{ font: "600 44px/1 var(--font-head)" }}>{tmt(me.balance)} <span style={{ fontSize: 20, color: "#DCE6FF" }}>TMT</span></span>
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
            <div><div style={{ color: "#C7D7FF" }}>Sagatlaýyn tutum</div><b>{bal ? tmt(bal.hourly) : "—"} TMT</b></div>
            <div><div style={{ color: "#C7D7FF" }}>Ýeter</div><b>{bal?.days_left != null ? `≈ ${bal.days_left} gün` : "—"}</b></div>
          </div>
        </section>
        <section className="card card-pad" style={{ flex: "1 1 380px", display: "flex", flexDirection: "column", gap: 14 }}>
          <h2 className="step-title">Balansy nädip doldurmaly</h2>
          <ol style={{ margin: 0, paddingLeft: 20, display: "flex", flexDirection: "column", gap: 8 }}>
            <li>Administratora hasap belgiňizi we möçberi aýdyň.</li>
            <li>Tölegi bank geçirimi ýa-da kassa arkaly geçiriň.</li>
            <li>Administrator tassyklandan soň balans şu ýerde görüner.</li>
          </ol>
          <div style={{ display: "flex", flexWrap: "wrap", alignItems: "center", justifyContent: "space-between", gap: 12, background: "#F9FAFB", border: "1px solid var(--border-soft)", borderRadius: 8, padding: "12px 14px" }}>
            <div><div className="muted" style={{ fontSize: 12 }}>Hasap belgiňiz</div><b className="mono" style={{ fontSize: 16 }}>{me.account_no}</b></div>
            <button type="button" className="btn btn-ghost" onClick={() => copy(me.account_no)}><Icon name="copy" small />Göçür</button>
          </div>
        </section>
      </div>
      <section className="card card-pad">
        <div style={{ display: "flex", flexWrap: "wrap", justifyContent: "space-between", gap: 8, marginBottom: 16 }}>
          <h2 className="step-title">Şu aý boýunça tutum</h2>
          <span className="muted">Jemi <b style={{ color: "var(--ink)" }}>{bal ? money(bal.month_used) : "—"}</b> · aýlyk çäk {bal ? money(bal.monthly_cap) : "—"}</span>
        </div>
        {servers.length === 0 ? <p className="muted" style={{ margin: 0 }}>Maşyn ýok</p> : (
          <div className="grid" style={{ gridTemplateColumns: "repeat(auto-fit,minmax(220px,1fr))", gap: 12 }}>
            {servers.map((s) => (
              <div key={s.id} className="meter" style={{ border: "1px solid var(--border-soft)", borderRadius: 10, padding: "14px 16px" }}>
                <div className="top"><b>{s.hostname}</b><span>{tmt(s.month_used)}</span></div>
                <div className="bar"><span style={{ width: `${Math.min(100, (s.month_used / s.plan.price_monthly) * 100)}%` }} /></div>
                <span className="hint" style={{ display: "flex", justifyContent: "space-between" }}><span>{s.plan.code} · çäk {money(s.plan.price_monthly)}</span><Badge status={s.status} /></span>
              </div>
            ))}
          </div>
        )}
      </section>
      <section className="card">
        <div className="card-head"><h2>Amallar taryhy</h2>
          <label style={{ display: "flex", alignItems: "center", gap: 8 }}>Görnüşi
            <select className="select" value={type} onChange={(e) => { setType(e.target.value); setPage(1); }}>
              <option value="">Hemmesi</option><option value="topup">Doldurmak</option><option value="charge">Tutum</option><option value="refund">Gaýtarmak</option><option value="adjustment">Düzediş</option>
            </select></label>
        </div>
        <div className="table-wrap"><table style={{ minWidth: 720 }}>
          <thead><tr><th>Wagty</th><th>Görnüşi</th><th>Düşündiriş</th><th className="num">Möçberi</th><th className="num">Galyndy</th></tr></thead>
          <tbody>{txs.length === 0 ? <tr><td colSpan={5} className="empty">Amal ýok</td></tr> : txs.map((t) => (
            <tr key={t.id}><td className="muted">{dt(t.created_at)}</td><td>{TYPE[t.type] ?? t.type}</td><td>{t.note}</td>
              <td className="num" style={{ fontWeight: 600, color: t.amount > 0 ? "var(--ok)" : undefined }}>{t.amount > 0 ? "+" : ""}{tmt(t.amount)}</td>
              <td className="num muted">{tmt(t.balance_after)}</td></tr>
          ))}</tbody>
        </table></div>
        <div className="pager"><span>Sahypa {page}</span>
          <div style={{ display: "flex", gap: 8 }}>
            <button type="button" className="btn btn-ghost btn-icon" aria-label="Öňki" disabled={page <= 1} onClick={() => setPage(page - 1)}>‹</button>
            <button type="button" className="btn btn-ghost btn-icon" aria-label="Indiki" disabled={txs.length < 25} onClick={() => setPage(page + 1)}>›</button>
          </div></div>
      </section>
    </>
  );
}
