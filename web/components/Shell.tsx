"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { createContext, useCallback, useContext, useEffect, useRef, useState } from "react";
import { api, ApiError, money, type User } from "@/lib/api";
import { Icon } from "./icons";

type Ctx = { me: User; refresh: () => Promise<void>; toast: (m: string) => void; setCrumbs: (c: string[]) => void };
const PanelCtx = createContext<Ctx | null>(null);
export const usePanel = () => {
  const c = useContext(PanelCtx);
  if (!c) throw new Error("usePanel outside Shell");
  return c;
};

/** Sahypanyň ýolyny (breadcrumb) goýýar. */
export function useCrumbs(...parts: string[]) {
  const { setCrumbs } = usePanel();
  const key = parts.join("/");
  useEffect(() => setCrumbs(parts), [key]); // eslint-disable-line react-hooks/exhaustive-deps
}

type NavItem = { href: string; label: string; icon: string; soon?: boolean };
type NavGroup = { group: string; icon: string; items: NavItem[] };
const NAV: (NavItem | NavGroup)[] = [
  { href: "/dashboard", label: "Baş sahypa", icon: "home" },
  { group: "Hasaplaýyş", icon: "cpu", items: [
    { href: "/machines", label: "Maşynlar", icon: "server" },
    { href: "/ssh-keys", label: "SSH açarlar", icon: "key" },
    { href: "/plans", label: "Konfigurasiýalar", icon: "sliders" },
    { href: "/images", label: "Obrazlar", icon: "disk" },
    { href: "/backups", label: "Bekaplar", icon: "backup", soon: true },
  ]},
  { group: "Tor", icon: "net", items: [
    { href: "/ips", label: "IP salgylar", icon: "net", soon: true },
    { href: "/firewall", label: "Firewall", icon: "shield", soon: true },
  ]},
  { group: "Maliýe", icon: "wallet", items: [
    { href: "/billing", label: "Balans", icon: "wallet" },
  ]},
];

export function Shell({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const path = usePathname();
  const [me, setMe] = useState<User | null>(null);
  const [crumbs, setCrumbs] = useState<string[]>([]);
  const [menuOpen, setMenuOpen] = useState(false);
  const [msg, setMsg] = useState("");
  const t = useRef<ReturnType<typeof setTimeout>>(undefined);

  const refresh = useCallback(async () => {
    try {
      setMe(await api<User>("/me"));
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) router.replace("/login");
    }
  }, [router]);

  useEffect(() => { refresh(); }, [refresh]);
  useEffect(() => setMenuOpen(false), [path]);
  // Balans her minutda täzelenýär
  useEffect(() => { const i = setInterval(refresh, 60_000); return () => clearInterval(i); }, [refresh]);

  const toast = useCallback((m: string) => {
    setMsg(m);
    clearTimeout(t.current);
    t.current = setTimeout(() => setMsg(""), 2600);
  }, []);

  async function logout() {
    await api("/auth/logout", { method: "POST" }).catch(() => {});
    router.replace("/login");
  }

  if (!me) return <div className="loading-screen" aria-busy="true">Ýüklenýär…</div>;

  const active = (href: string) => path === href || path.startsWith(href + "/");
  const link = (it: NavItem, sub: boolean) => (
    <Link key={it.href + it.label} href={it.href} className={`${sub ? "sub " : ""}${active(it.href) ? "active" : ""}`}
      aria-current={active(it.href) ? "page" : undefined}>
      <Icon name={it.icon} small={sub} />{it.label}{it.soon && <span className="soon">soň</span>}
    </Link>
  );

  return (
    <PanelCtx.Provider value={{ me, refresh, toast, setCrumbs }}>
      <div className="app">
        <aside className={`sidebar ${menuOpen ? "open" : ""}`} aria-label="Esasy menýu">
          <Link className="brand" href="/dashboard">
            <span className="brand-logo"><img src="/logo.svg" alt="ASTU" /></span>
            <span className="brand-name">VPS</span>
          </Link>
          <nav className="nav">
            {NAV.map((n) =>
              "group" in n ? (
                <details key={n.group} open={n.group === "Hasaplaýyş" || n.items.some((i) => active(i.href))}>
                  <summary>
                    <span style={{ display: "flex", alignItems: "center", gap: 12 }}><Icon name={n.icon} />{n.group}</span>
                    <span className="chev" style={{ display: "flex" }}><Icon name="chev" small /></span>
                  </summary>
                  <div className="group-items">{n.items.map((i) => link(i, true))}</div>
                </details>
              ) : link(n, false),
            )}
          </nav>
          <div className="sidebar-foot nav">
            {me.role === "admin" && link({ href: "/admin", label: "Admin paneli", icon: "shield" }, false)}
            <button type="button" className="nav-btn" onClick={logout}><Icon name="logout" />Çykmak</button>
          </div>
        </aside>

        <div className="content">
          <header className="topbar">
            <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
              <button type="button" className="btn btn-ghost btn-icon menu-toggle" aria-label="Menýuny aç" onClick={() => setMenuOpen((v) => !v)}>
                <Icon name="menu" />
              </button>
              <nav className="crumbs" aria-label="Ýol">
                {crumbs.map((c, i) =>
                  i === crumbs.length - 1 ? <span key={i} className="current">{c}</span>
                    : <span key={i} style={{ display: "contents" }}><span>{c}</span><span className="sep" aria-hidden="true">/</span></span>,
                )}
              </nav>
            </div>
            <div className="top-actions">
              <Link className="balance-pill" href="/billing" aria-label={`Balans ${money(me.balance)}, doldurmak`}>
                <span style={{ color: me.balance < 0 ? "var(--danger)" : undefined }}>{money(me.balance)}</span>
                <span className="plus"><Icon name="plus" small sw={2.6} /></span>
              </Link>
              <label>
                <span className="sr-only">Dil</span>
                <select className="select" defaultValue="TM" aria-label="Dil"><option>TM</option><option disabled>RU</option><option disabled>EN</option></select>
              </label>
              <span className="user-btn"><span className="avatar">{(me.full_name || me.email)[0]?.toUpperCase()}</span><span className="name">{me.email}</span></span>
            </div>
          </header>
          <main id="view">
            {me.status !== "active" && (
              <div className="alert alert-warn"><Icon name="alert" /><span>Emailiňizi tassyklaň: size iberilen hatdaky salga geçiň. Tassyklanýança maşyn döredip bolmaýar.</span></div>
            )}
            {children}
          </main>
        </div>
      </div>
      <div className={`toast ${msg ? "show" : ""}`} role="status" aria-live="polite">{msg}</div>
    </PanelCtx.Provider>
  );
}

/** Sahypanyň sözbaşysy */
export function PageHead({ title, sub, children }: { title: string; sub?: React.ReactNode; children?: React.ReactNode }) {
  return (
    <div className="page-head">
      <div><h1>{title}</h1>{sub && <p>{sub}</p>}</div>
      {children && <div className="row-actions">{children}</div>}
    </div>
  );
}

export function Badge({ status }: { status: string }) {
  const map: Record<string, [string, string]> = {
    running: ["IŞJEŇ", "badge-ok"], stopped: ["DURDURYLAN", "badge-off"], suspended: ["SAKLANAN", "badge-warn"],
    provisioning: ["DÖREDILÝÄR", "badge-info"], reinstalling: ["TÄZEDEN GURULÝAR", "badge-info"],
    deleting: ["POZULÝAR", "badge-warn"], error: ["ÝALŇYŞLYK", "badge-danger"],
    active: ["IŞJEŇ", "badge-ok"], pending: ["TASSYKLANMADYK", "badge-off"], blocked: ["BLOKLANAN", "badge-danger"],
  };
  const [label, cls] = map[status] || [status, "badge-off"];
  return <span className={`badge ${cls}`}>{label}</span>;
}

export function useCopy() {
  const { toast } = usePanel();
  return (text: string) =>
    navigator.clipboard?.writeText(text).then(() => toast("Göçürildi: " + text), () => toast(text)) ?? toast(text);
}
