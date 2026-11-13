"use client";

import { useCallback, useEffect, useState } from "react";
import { Badge, PageHead, useCopy, useCrumbs, usePanel } from "@/components/Shell";
import { Icon } from "@/components/icons";
import { api, ApiError, dt, METHOD, money, tmt, TOPUP_STATUS, type Balance, type Server, type Topup, type Tx } from "@/lib/api";

const TYPE: Record<string, React.ReactNode> = {
  charge: "Tutum",
  topup: <span className="badge badge-ok">Doldurmak</span>,
  refund: <span className="badge badge-info">Gaýtarmak</span>,
  adjustment: <span className="badge badge-off">Düzediş</span>,
};

export default function Billing() {
  useCrumbs("Maliýe", "Balans");
  const { me, toast, refresh } = usePanel();
  const copy = useCopy();
  const [bal, setBal] = useState<Balance | null>(null);
  const [servers, setServers] = useState<Server[]>([]);
  const [txs, setTxs] = useState<Tx[]>([]);
  const [type, setType] = useState("");
  const [page, setPage] = useState(1);
  const [topups, setTopups] = useState<Topup[] | null>(null);
  const [amount, setAmount] = useState("100");
  const [method, setMethod] = useState<"bank" | "cash" | "card">("bank");
  const [reference, setReference] = useState("");
  const [comment, setComment] = useState("");
  const [err, setErr] = useState("");
  const [sending, setSending] = useState(false);

  const loadTopups = useCallback(() => api<Topup[]>("/topups").then(setTopups).catch(() => setTopups([])), []);
  useEffect(() => { api<Balance>("/me/balance").then(setBal); api<Server[]>("/servers").then(setServers); loadTopups(); }, [loadTopups]);
  useEffect(() => { api<Tx[]>(`/me/transactions?limit=25&page=${page}&type=${type}`).then(setTxs); }, [type, page]);
  // Garaşylýan haýyş bar bolsa, tassyklananda balans özi täzelensin
  const waiting = !!topups?.some((t) => t.status === "pending");
  useEffect(() => {
    if (!waiting) return;
    const i = setInterval(() => {
      api<Topup[]>("/topups").then((l) => {
        if (l.some((t) => t.status === "approved" && topups?.find((o) => o.id === t.id)?.status === "pending")) {
          toast("Balansyňyz dolduryldy"); refresh(); api<Balance>("/me/balance").then(setBal);
          api<Tx[]>(`/me/transactions?limit=25&page=${page}&type=${type}`).then(setTxs);
        }
        setTopups(l);
      }).catch(() => {});
    }, 15000);
    return () => clearInterval(i);
  }, [waiting, topups, toast, refresh, page, type]);

  async function sendTopup(e: React.FormEvent) {
    e.preventDefault(); setErr(""); setSending(true);
    try {
      await api("/topups", { method: "POST", body: { amount: Number(amount), method, reference, comment } });
      setReference(""); setComment(""); toast("Haýyş iberildi. Administrator tölegi barlap tassyklar."); loadTopups();
    } catch (e) { setErr(e instanceof ApiError ? e.message : "Ýalňyşlyk"); }
    finally { setSending(false); }
  }
  async function cancel(t: Topup) {
    if (!window.confirm(`${money(t.amount)} doldurma haýyşy ýatyrylsynmy?`)) return;
    try { await api(`/topups/${t.id}/cancel`, { method: "POST" }); toast("Haýyş ýatyryldy"); loadTopups(); }
    catch (e) { toast(e instanceof ApiError ? e.message : "Ýalňyşlyk"); }
  }
  const refHint: Record<string, string> = {
    bank: "Bank geçiriminiň (platýožkanyň) belgisi",
    cash: "Kassanyň beren çekiniň belgisi",
    card: "Kart geçiriminiň belgisi ýa-da kartyň soňky 4 sany",
  };

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
        <form id="doldur" className="card card-pad" onSubmit={sendTopup} style={{ flex: "1 1 420px", display: "flex", flexDirection: "column", gap: 14, border: "2px solid var(--primary)" }}>
          <div><h2 className="step-title">Balansy doldur</h2>
            <span className="muted">1) Tölegi geçiriň · 2) Çek belgisini şu ýere ýazyň · 3) Administrator tassyklanda balans goşulýar</span></div>
          <div style={{ display: "flex", flexWrap: "wrap", alignItems: "center", justifyContent: "space-between", gap: 12, background: "#F9FAFB", border: "1px solid var(--border-soft)", borderRadius: 8, padding: "10px 14px" }}>
            <div><div className="muted" style={{ fontSize: 12 }}>Tölegiň maksadyna hasap belgiňizi ýazyň</div><b className="mono" style={{ fontSize: 16 }}>{me.account_no}</b></div>
            <button type="button" className="btn btn-ghost" onClick={() => copy(me.account_no)}><Icon name="copy" small />Göçür</button>
          </div>
          {bal?.payment_info && <div className="note-box"><div className="muted" style={{ fontSize: 12, marginBottom: 4 }}>Töleg rekwizitleri</div>{bal.payment_info}</div>}
          <div className="segmented seg3" role="radiogroup" aria-label="Töleg usuly" style={{ maxWidth: "none" }}>
            {(["bank", "cash", "card"] as const).map((m) => (
              <button key={m} type="button" role="radio" aria-checked={method === m} onClick={() => setMethod(m)}>{METHOD[m]}</button>))}
          </div>
          <label className="field">Möçber (TMT)<input className="input" type="number" min="1" max="100000" step="0.01" required value={amount} onChange={(e) => setAmount(e.target.value)} /></label>
          <div className="chips">{[50, 100, 200, 500, 1000].map((v) => (
            <button key={v} type="button" className="chip" onClick={() => setAmount(String(v))}>{v} TMT</button>))}</div>
          <label className="field">Çek ýa-da töleg belgisi<input className="input" required maxLength={80} value={reference} onChange={(e) => setReference(e.target.value)} placeholder={refHint[method]} /></label>
          <label className="field">Bellik (islege görä)<input className="input" maxLength={200} value={comment} onChange={(e) => setComment(e.target.value)} placeholder="mysal: Halkbank, 13.11.2026" /></label>
          {err && <p className="form-error" role="alert">{err}</p>}
          <button className="btn btn-lg" disabled={sending || !(Number(amount) >= 1) || !reference.trim()}>Haýyş iber · {Number(amount) > 0 ? money(Math.round(Number(amount) * 100)) : "—"}</button>
          <span className="hint">Balans tassyklanan pursady goşulýar. Öçürilen serwerleriňiz bir minudyň dowamynda awtomatik açylýar.</span>
        </form>
      </div>
      <section className="card">
        <div className="card-head"><h2>Doldurma haýyşlarym</h2>{waiting && <span className="muted">Tassyklanmagyna garaşylýar…</span>}</div>
        <div className="table-wrap"><table style={{ minWidth: 760 }}>
          <thead><tr><th>Wagty</th><th className="num">Möçberi</th><th>Usul</th><th>Çek / töleg belgisi</th><th>Ýagdaýy</th><th>Jogap</th><th><span className="sr-only">Hereketler</span></th></tr></thead>
          <tbody>{topups === null ? <tr><td colSpan={7} className="empty">Ýüklenýär…</td></tr> :
            topups.length === 0 ? <tr><td colSpan={7} className="empty">Entek haýyş ýok</td></tr> : topups.map((t) => {
              const [label, cls] = TOPUP_STATUS[t.status] ?? [t.status, "badge-off"];
              return (
                <tr key={t.id}><td className="muted" style={{ whiteSpace: "nowrap" }}>{dt(t.created_at)}</td>
                  <td className="num" style={{ fontWeight: 600 }}>{tmt(t.amount)}</td><td>{METHOD[t.method]}</td>
                  <td><span className="mono">{t.reference}</span>{t.comment && <div className="muted" style={{ fontSize: 12 }}>{t.comment}</div>}</td>
                  <td><span className={`badge ${cls}`}>{label}</span></td>
                  <td className="muted">{t.admin_note || (t.decided_at ? dt(t.decided_at) : "—")}</td>
                  <td className="num">{t.status === "pending" && <button type="button" className="btn btn-ghost" style={{ minHeight: 34 }} onClick={() => cancel(t)}>Ýatyr</button>}</td></tr>
              );
            })}</tbody>
        </table></div>
      </section>
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
