"use client";

import { useEffect, useState } from "react";
import { PageHead, useCrumbs } from "@/components/Shell";
import { api, type Image } from "@/lib/api";

export default function Images() {
  useCrumbs("Hasaplaýyş", "Obrazlar");
  const [list, setList] = useState<Image[]>([]);
  useEffect(() => { api<Image[]>("/images").then(setList); }, []);
  return (
    <>
      <PageHead title="Obrazlar" sub="Täze maşyn ýa-da OS-i täzeden gurmak üçin elýeterli operasion ulgamlar" />
      <section className="card"><div className="table-wrap"><table>
        <thead><tr><th>Ady</th><th>Wersiýa</th><th>Kod</th></tr></thead>
        <tbody>{list.length === 0 ? <tr><td colSpan={3} className="empty">Obraz ýok</td></tr> :
          list.map((i) => <tr key={i.id}><td><b>{i.name}</b></td><td>{i.version}</td><td className="mono">{i.slug}</td></tr>)}</tbody>
      </table></div></section>
    </>
  );
}
