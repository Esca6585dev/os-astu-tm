"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useState } from "react";
import { api, ApiError } from "@/lib/api";

function Reset() {
  const token = useSearchParams().get("token") || "";
  const router = useRouter();
  const [err, setErr] = useState("");
  async function submit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const password = new FormData(e.currentTarget).get("password");
    try {
      await api("/auth/reset", { method: "POST", body: { token, password } });
      router.replace("/login");
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Ýalňyşlyk");
    }
  }
  return (
    <form className="auth-card" onSubmit={submit}>
      <h2>Täze parol</h2>
      <label className="field">Täze parol<input className="input" name="password" type="password" minLength={10} required autoComplete="new-password" /></label>
      {err && <p className="form-error" role="alert">{err}</p>}
      <button className="btn btn-lg btn-block">Ýatda sakla</button>
      <Link href="/login">← Giriş</Link>
    </form>
  );
}

export default function Page() {
  return <Suspense><Reset /></Suspense>;
}
