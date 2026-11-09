import Link from "next/link";

export default function AuthLayout({ children }: { children: React.ReactNode }) {
  const check = (
    <svg className="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M5 12l4.5 4.5L19 7" /></svg>
  );
  return (
    <div className="auth">
      <section className="auth-hero">
        <Link className="brand" href="/login"><span className="brand-logo"><img src="/logo.svg" alt="ASTU" /></span><span className="brand-name">VPS</span></Link>
        <div style={{ display: "flex", flexDirection: "column", gap: 20 }}>
          <h1>Ýerli VPS serwerler, sagatlaýyn töleg bilen</h1>
          <p>Konfigurasiýa saýlaň, maşynyňyzy açyň we SSH bilen giriň. Diňe ulanan sagadyňyz üçin tölärsiňiz.</p>
          <ul>
            <li>{check}Ubuntu, Debian we beýleki obrazlar</li>
            <li>{check}Root parol ýa-da SSH açar bilen giriş</li>
            <li>{check}Aýlyk çäk: tarif bahasyndan köp tutulmaýar</li>
          </ul>
        </div>
        <p style={{ margin: 0, color: "#C7D7FF", fontSize: 13 }}>© {new Date().getFullYear()} ASTU</p>
      </section>
      <section className="auth-form">
        <div style={{ width: "100%", maxWidth: 400, display: "flex", flexDirection: "column", gap: 24 }}>{children}</div>
      </section>
    </div>
  );
}
