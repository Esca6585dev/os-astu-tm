"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";
import { api, ApiError } from "@/lib/api";

function Verify() {
  const token = useSearchParams().get("token") || "";
  const router = useRouter();
  const [msg, setMsg] = useState("Tassyklanýar…");
  useEffect(() => {
    api("/auth/verify", { method: "POST", body: { token } })
      .then(() => { setMsg("Email tassyklandy!"); setTimeout(() => router.replace("/dashboard"), 1200); })
      .catch((e) => setMsg(e instanceof ApiError ? e.message : "Ýalňyşlyk"));
  }, [token, router]);
  return <div className="auth-card"><h2>Email tassyklama</h2><p>{msg}</p><Link href="/login">Giriş sahypasyna</Link></div>;
}

export default function Page() {
  return <Suspense><Verify /></Suspense>;
}
