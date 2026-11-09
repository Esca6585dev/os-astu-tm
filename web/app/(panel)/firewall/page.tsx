"use client";

import Link from "next/link";
import { PageHead, useCrumbs } from "@/components/Shell";

export default function Page() {
  useCrumbs("Firewall");
  return (
    <>
      <PageHead title="Firewall" sub="Bu bölüm indiki tapgyrda goşulýar." />
      <Link className="btn" href="/machines" style={{ alignSelf: "flex-start" }}>Maşynlar</Link>
    </>
  );
}
