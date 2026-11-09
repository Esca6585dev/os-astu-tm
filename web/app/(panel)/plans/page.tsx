"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { PageHead, useCrumbs } from "@/components/Shell";
import { Icon } from "@/components/icons";
import { api, hourly, ramGB, type Plan } from "@/lib/api";

export default function Plans() {
  useCrumbs("Hasaplaýyş", "Konfigurasiýalar");
  const [plans, setPlans] = useState<Plan[]>([]);
  useEffect(() => { api<Plan[]>("/plans").then(setPlans); }, []);
  const li = (k: string, v: string | number) => <li><span>{k}</span><span>{v}</span></li>;
  return (
    <>
      <PageHead title="Konfigurasiýalar" sub="Wirtual hususy serweriň (VPS Hosting) nyrhnamasy" />
      <div className="grid plans">
        {plans.map((p) => (
          <article key={p.id} className={`card plan ${p.is_popular ? "popular" : ""}`}>
            {p.is_popular && <span className="tag">Köp saýlanýan</span>}
            <h2>{p.code}</h2>
            <div><div className="price">{(p.price_monthly / 100).toFixed(2)} <small>TMT / aý</small></div><div className="muted">≈ {hourly(p)} TMT sagatda</div></div>
            <ul>{li("CPU", p.vcpu)}{li("RAM", ramGB(p.ram_mb))}{li("HDD", `${p.disk_gb} GB`)}{li("IP", p.ip_count)}{li("Tizlik", `${p.bandwidth_mbit} Mbit/s`)}</ul>
            <Link className={`btn ${p.is_popular ? "" : "btn-outline"}`} href={`/machines/new?plan=${p.id}`}>Saýla</Link>
          </article>
        ))}
      </div>
      <div className="alert alert-info"><Icon name="info" /><span>Töleg her sagat balansdan aýrylýar (aýlyk baha / 720). Bir aýda tarifiň aýlyk bahasyndan köp tutulmaýar.</span></div>
    </>
  );
}
