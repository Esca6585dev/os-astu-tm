"use client";

import { useCallback, useEffect, useState } from "react";
import { PageHead, useCrumbs, usePanel } from "@/components/Shell";
import { Icon } from "@/components/icons";
import { api, ApiError, dt, type SSHKey } from "@/lib/api";

export default function Keys() {
  useCrumbs("Hasaplaýyş", "SSH açarlar");
  const { toast } = usePanel();
  const [keys, setKeys] = useState<SSHKey[] | null>(null);
  const [name, setName] = useState("");
  const [pub, setPub] = useState("");
  const [err, setErr] = useState("");
  const load = useCallback(() => api<SSHKey[]>("/ssh-keys").then(setKeys), []);
  useEffect(() => { load(); }, [load]);

  async function add(e: React.FormEvent) {
    e.preventDefault(); setErr("");
    try {
      await api("/ssh-keys", { method: "POST", body: { name, public_key: pub } });
      setName(""); setPub(""); toast("Açar goşuldy"); load();
    } catch (e) { setErr(e instanceof ApiError ? e.message : "Ýalňyşlyk"); }
  }
  async function del(k: SSHKey) {
    if (!window.confirm(`"${k.name}" açary pozulsynmy? Bar bolan maşynlardaky açar galar.`)) return;
    await api(`/ssh-keys/${k.id}`, { method: "DELETE" }).then(() => { toast("Açar pozuldy"); load(); }).catch(() => toast("Ýalňyşlyk"));
  }

  return (
    <>
      <PageHead title="SSH açarlar" sub="Täze maşyn döredilende saýlanan açarlar root ulanyjysyna goşulýar" />
      <div className="split">
        <section className="card main-col"><div className="table-wrap"><table style={{ minWidth: 640 }}>
          <thead><tr><th>Ady</th><th>Görnüşi</th><th>Barmak yzy</th><th>Ulanylýar</th><th>Goşuldy</th><th><span className="sr-only">Hereketler</span></th></tr></thead>
          <tbody>
            {keys === null ? <tr><td colSpan={6} className="empty">Ýüklenýär…</td></tr> :
              keys.length === 0 ? <tr><td colSpan={6} className="empty">Entek açar ýok</td></tr> :
                keys.map((k) => (
                  <tr key={k.id}><td><b>{k.name}</b></td><td>{k.type}</td><td className="mono" style={{ fontSize: 12 }}>{k.fingerprint}</td>
                    <td className="muted">{k.used_by.join(", ") || "—"}</td><td className="muted">{dt(k.created_at).slice(0, 10)}</td>
                    <td className="num"><button type="button" className="btn btn-danger" style={{ minHeight: 34 }} onClick={() => del(k)}>Poz</button></td></tr>
                ))}
          </tbody>
        </table></div></section>
        <form className="card card-pad side-col" onSubmit={add} style={{ display: "flex", flexDirection: "column", gap: 14 }}>
          <h2 className="step-title">Täze açar goş</h2>
          <label className="field">Ady<input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="mysal: is-noutbuk" required maxLength={60} /></label>
          <label className="field">Açyk açar<textarea className="textarea" rows={5} value={pub} onChange={(e) => setPub(e.target.value)} placeholder="ssh-ed25519 AAAAC3Nza… siz@kompýuter" required /></label>
          <span className="hint">Açary almak üçin: <code className="mono">cat ~/.ssh/id_ed25519.pub</code></span>
          {err && <p className="form-error" role="alert">{err}</p>}
          <button className="btn"><Icon name="plus" small sw={2.4} />Açary goş</button>
        </form>
      </div>
    </>
  );
}
