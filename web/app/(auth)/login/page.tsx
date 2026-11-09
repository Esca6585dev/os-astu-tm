"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { api, ApiError } from "@/lib/api";

export default function LoginPage() {
  const router = useRouter();
  const [tab, setTab] = useState<"login" | "register">("login");
  const [err, setErr] = useState("");
  const [info, setInfo] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = Object.fromEntries(new FormData(e.currentTarget)) as Record<string, string>;
    setErr(""); setInfo(""); setBusy(true);
    try {
      if (tab === "login") {
        await api("/auth/login", { method: "POST", body: { email: f.email, password: f.password } });
        router.replace("/dashboard");
        return;
      }
      if (!f.terms) { setErr("Ulanyş şertlerini kabul ediň"); return; }
      const r = await api<{ verify_required: boolean }>("/auth/register", { method: "POST",
        body: { email: f.email, password: f.password, full_name: f.full_name, phone: f.phone } });
      setTab("login");
      setInfo(r.verify_required ? "Hasap döredildi. Emailiňize iberilen salga geçip tassyklaň." : "Hasap döredildi. Indi giriň.");
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Birikmede ýalňyşlyk");
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <div className="segmented" role="tablist" aria-label="Giriş görnüşi" style={{ maxWidth: "none" }}>
        <button type="button" role="tab" aria-selected={tab === "login"} aria-checked={tab === "login"} onClick={() => { setTab("login"); setErr(""); }}>Giriş</button>
        <button type="button" role="tab" aria-selected={tab === "register"} aria-checked={tab === "register"} onClick={() => { setTab("register"); setErr(""); setInfo(""); }}>Hasaba alyş</button>
      </div>
      <form className="auth-card" onSubmit={submit} key={tab}>
        {tab === "login" ? (
          <>
            <div><h2>Hasabyňyza giriň</h2><p className="muted" style={{ margin: "4px 0 0" }}>Email we parolyňyzy ýazyň</p></div>
            {info && <div className="alert alert-info"><span>{info}</span></div>}
            <label className="field">Email<input className="input" name="email" type="email" required autoComplete="username" placeholder="siz@mysal.tm" /></label>
            <label className="field">Parol<input className="input" name="password" type="password" required autoComplete="current-password" placeholder="••••••••" /></label>
            <div style={{ display: "flex", justifyContent: "flex-end" }}><Link href="/forgot">Paroly unutdyňyzmy?</Link></div>
          </>
        ) : (
          <>
            <div><h2>Täze hasap açyň</h2><p className="muted" style={{ margin: "4px 0 0" }}>Emailiňize tassyklama salgysy iberiler</p></div>
            <label className="field">Doly adyňyz<input className="input" name="full_name" autoComplete="name" placeholder="Aman Amanow" /></label>
            <label className="field">Email<input className="input" name="email" type="email" required autoComplete="email" placeholder="siz@mysal.tm" /></label>
            <label className="field">Telefon<input className="input" name="phone" type="tel" autoComplete="tel" placeholder="+993 6X XXXXXX" /></label>
            <label className="field">Parol<input className="input" name="password" type="password" required minLength={10} autoComplete="new-password" placeholder="Azyndan 10 simwol" /></label>
            <label style={{ display: "flex", gap: 8, alignItems: "flex-start" }}><input type="checkbox" name="terms" style={{ width: 18, height: 18, marginTop: 2 }} />Ulanyş şertlerini we gizlinlik syýasatyny kabul edýärin</label>
          </>
        )}
        {err && <p className="form-error" role="alert">{err}</p>}
        <button className="btn btn-lg btn-block" type="submit" disabled={busy}>{tab === "login" ? "Giriş" : "Hasaby döret"}</button>
      </form>
    </>
  );
}
