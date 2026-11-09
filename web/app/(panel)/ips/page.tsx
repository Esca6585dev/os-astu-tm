"use client";

import Link from "next/link";
import { PageHead, useCrumbs } from "@/components/Shell";

export default function Page() {
  useCrumbs("IP salgylar");
  return (
    <>
      <PageHead title="IP salgylar" sub="Bu bölüm indiki tapgyrda goşulýar." />
      <Link className="btn" href="/machines" style={{ alignSelf: "flex-start" }}>Maşynlar</Link>
    </>
  );
}
