"use client";

import Link from "next/link";
import { useState } from "react";
import { api } from "@/lib/api";

export default function Forgot() {
  const [sent, setSent] = useState(false);
  async function submit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const email = new FormData(e.currentTarget).get("email");
    await api("/auth/forgot", { method: "POST", body: { email } }).catch(() => {});
    setSent(true);
  }
  return (
    <form className="auth-card" onSubmit={submit}>
      <div><h2>Paroly dikeltmek</h2><p className="muted" style={{ margin: "4px 0 0" }}>Emailiňize täze parol goýmak üçin salgy iberiler</p></div>
      {sent ? <div className="alert alert-info"><span>Eger bu email bilen hasap bar bolsa, hat iberildi.</span></div> : (
        <>
          <label className="field">Email<input className="input" name="email" type="email" required autoComplete="email" /></label>
          <button className="btn btn-lg btn-block">Iber</button>
        </>
      )}
      <Link href="/login">← Giriş</Link>
    </form>
  );
}
