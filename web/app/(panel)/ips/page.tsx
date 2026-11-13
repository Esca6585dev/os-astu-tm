"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { Badge, PageHead, useCopy, useCrumbs } from "@/components/Shell";
import { Icon } from "@/components/icons";
import { api, type IPAddr } from "@/lib/api";

export default function IPs() {
  useCrumbs("Tor", "IP salgylar");
  const copy = useCopy();
  const [list, setList] = useState<IPAddr[] | null>(null);
  const [sel, setSel] = useState<string>("");
  useEffect(() => { api<IPAddr[]>("/ips").then((l) => { setList(l); setSel(l[0]?.address ?? ""); }).catch(() => setList([])); }, []);

  const ip = list?.find((x) => x.address === sel);
  const netplan = ip ? `network:
  version: 2
  ethernets:
    eth0:
      addresses: [${ip.address}/${ip.prefix}]
      routes:
        - to: default
          via: ${ip.gateway}
      nameservers:
        addresses: [${ip.dns.join(", ")}]` : "";

  return (
    <>
      <PageHead title="IP salgylar" sub="Maşynlaryňyzyň jemgyýetçilik IPv4 salgylary we tor sazlamalary" />
      <section className="card">
        <div className="table-wrap"><table style={{ minWidth: 860 }}>
          <thead><tr><th>IP salgy</th><th>Maska</th><th>Şlýuz</th><th>DNS</th><th>Maşyn</th><th>Firewall</th><th><span className="sr-only">Hereketler</span></th></tr></thead>
          <tbody>
            {list === null ? <tr><td colSpan={7} className="empty">Ýüklenýär…</td></tr> :
              list.length === 0 ? <tr><td colSpan={7} className="empty">IP salgy ýok: ol maşyn döredilende awtomatik berilýär. <Link href="/machines/new">Maşyn döret</Link></td></tr> :
                list.map((x) => (
                  <tr key={x.address} style={sel === x.address ? { background: "var(--primary-soft)" } : undefined}>
                    <td><span style={{ display: "inline-flex", alignItems: "center", gap: 6 }}><b className="mono">{x.address}</b>
                      <button type="button" className="icon-btn" aria-label={`${x.address} göçür`} onClick={() => copy(x.address)}><Icon name="copy" small /></button></span></td>
                    <td className="mono">/{x.prefix} <span className="muted">· {x.netmask}</span></td>
                    <td className="mono">{x.gateway}</td>
                    <td className="mono" style={{ fontSize: 12 }}>{x.dns.join(", ")}</td>
                    <td><Link href={`/machines/${x.server.id}`}><b>{x.server.hostname}</b></Link> <Badge status={x.server.status} /></td>
                    <td><Link href={`/firewall?server=${x.server.id}`}>{x.firewall.policy_in === "DROP" ? "Diňe rugsat berlenler" : "Hemmesine rugsat"} · {x.firewall.rules} düzgün</Link></td>
                    <td className="num"><button type="button" className="btn btn-ghost" style={{ minHeight: 34 }} onClick={() => setSel(x.address)}>Sazlamalar</button></td>
                  </tr>
                ))}
          </tbody>
        </table></div>
      </section>

      {ip && (
        <div className="split">
          <section className="card card-pad main-col" style={{ display: "flex", flexDirection: "column", gap: 12 }}>
            <div style={{ display: "flex", flexWrap: "wrap", justifyContent: "space-between", alignItems: "center", gap: 8 }}>
              <h2 className="step-title">Tor sazlamalary · <span className="mono">{ip.address}</span></h2>
              <button type="button" className="btn btn-ghost" onClick={() => copy(netplan)}><Icon name="copy" small />Göçür</button>
            </div>
            <p className="muted" style={{ margin: 0 }}>Bu sazlamalar maşyn döredilende cloud-init arkaly awtomatik goýulýar. Olary diňe tory el bilen täzeden sazlasaňyz ulanyň (Ubuntu netplan, <code className="mono">/etc/netplan/50-cloud-init.yaml</code>):</p>
            <pre className="code-block">{netplan}</pre>
          </section>
          <section className="card card-pad side-col" style={{ display: "flex", flexDirection: "column", gap: 10 }}>
            <h2 className="step-title">Bilmeli zatlar</h2>
            <ul style={{ margin: 0, paddingLeft: 18, display: "flex", flexDirection: "column", gap: 8 }}>
              <li>Her maşyna 1 jemgyýetçilik IPv4 salgy berilýär.</li>
              <li>IP salgy maşyn pozulýança üýtgemeýär: OS täzeden gurlanda we bekapdan dikeldilende hem.</li>
              <li>Maşyn diňe öz IP salgysyny ulanyp bilýär, başga IP goýulsa tor işlemez.</li>
              <li>Çykýan 25-nji port (SMTP) spam garşy ýapyk.</li>
            </ul>
          </section>
        </div>
      )}
    </>
  );
}
