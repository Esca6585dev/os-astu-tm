// Go API klienti. Ähli pul mukdarlary API-den teňňede (1 TMT = 100) gelýär.

export type User = {
  id: number; email: string; full_name: string; phone: string;
  role: "user" | "admin"; status: string; balance: number; account_no: string; email_verified: boolean;
};
export type Plan = {
  id: number; code: string; vcpu: number; ram_mb: number; disk_gb: number; ip_count: number;
  bandwidth_mbit: number; price_monthly: number; price_hourly: number; is_popular: boolean; is_active: boolean; sort: number;
};
export type Image = { id: number; slug: string; name: string; version: string; proxmox_template_vmid?: number; is_active: boolean; sort: number };
export type Usage = { status: string; cpu: number; mem_used: number; mem_max: number; disk_max: number; net_in: number; net_out: number; uptime: number };
export type Server = {
  id: number; hostname: string; status: string; busy: boolean; ip: string | null; gateway: string | null;
  plan: Plan; image: Image; node: string; vmid: number | null; created_at: string; suspended_at: string | null;
  month_used: number; user_email?: string; usage?: Usage; last_job_error: string | null;
};
export type SSHKey = { id: number; name: string; type: string; fingerprint: string; created_at: string; used_by: string[] };
export type Tx = { id: number; type: string; amount: number; balance_after: number; note: string; server_id: number | null; created_at: string; user_email?: string };
export type Balance = { balance: number; hourly: number; monthly_cap: number; month_used: number; days_left: number | null; billable_servers: number; account_no: string; payment_info: string };

export type FWRule = { direction: "in" | "out"; action: "ACCEPT" | "DROP"; proto: "" | "tcp" | "udp" | "icmp"; dport: string; addr: string; comment: string };
export type Firewall = {
  server_id: number; hostname: string; ip: string | null; status: string; busy: boolean; editable: boolean;
  policy_in: "ACCEPT" | "DROP"; rules: FWRule[]; system_rules: FWRule[]; last_job_error: string | null;
};
export type Backup = {
  id: number; server_id: number; hostname: string; note: string; status: string; size_bytes: number;
  image: string; error: string; created_at: string; finished_at: string | null;
};
export type IPAddr = {
  address: string; prefix: number; netmask: string; gateway: string; dns: string[];
  server: { id: number; hostname: string; status: string }; firewall: { policy_in: string; rules: number };
};
export type Topup = {
  id: number; amount: number; method: "bank" | "cash" | "card"; reference: string; comment: string; status: string;
  admin_note: string; created_at: string; decided_at: string | null;
  user_id?: number; user_email?: string; full_name?: string; account_no?: string; user_balance?: number;
};
export const METHOD: Record<string, string> = { bank: "Bank geçirimi", cash: "Kassa (nagt)", card: "Bank kartyndan" };
export const TOPUP_STATUS: Record<string, [string, string]> = {
  pending: ["GARAŞYLÝAR", "badge-warn"], approved: ["TASSYKLANDY", "badge-ok"],
  rejected: ["RET EDILDI", "badge-danger"], cancelled: ["ÝATYRYLDY", "badge-off"],
};

export class ApiError extends Error {
  constructor(public status: number, public code: string, message: string) { super(message); }
}

export async function api<T = unknown>(path: string, opts: { method?: string; body?: unknown } = {}): Promise<T> {
  const res = await fetch(`/api/v1${path}`, {
    method: opts.method || "GET",
    credentials: "same-origin",
    headers: {
      "X-Requested-With": "fetch",
      ...(opts.body !== undefined ? { "Content-Type": "application/json" } : {}),
    },
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
    cache: "no-store",
  });
  const text = await res.text();
  const data = text ? JSON.parse(text) : null;
  if (!res.ok) {
    const e = data?.error || {};
    throw new ApiError(res.status, e.code || "error", e.message || `Ýalňyşlyk (${res.status})`);
  }
  return data as T;
}

// ---------- Formatlar ----------
export const tmt = (tenge: number) => {
  const sign = tenge < 0 ? "−" : "";
  const v = Math.abs(tenge);
  return `${sign}${Math.floor(v / 100).toLocaleString("en-US").replace(/,/g, " ")}.${String(v % 100).padStart(2, "0")}`;
};
export const money = (tenge: number) => `${tmt(tenge)} TMT`;
export const hourly = (p: Plan) => (p.price_monthly / 720 / 100).toFixed(2);
export const ramGB = (mb: number) => (mb % 1024 === 0 ? `${mb / 1024} GB` : `${(mb / 1024).toFixed(1)} GB`);
export const dt = (iso: string) => {
  const d = new Date(iso);
  const p = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
};
export const bytes = (n: number) => {
  const u = ["B", "KB", "MB", "GB", "TB"]; let i = 0;
  while (n >= 1024 && i < u.length - 1) { n /= 1024; i++; }
  return `${n.toFixed(i ? 1 : 0)} ${u[i]}`;
};

export const STATUS: Record<string, { label: string; cls: string }> = {
  running: { label: "IŞJEŇ", cls: "badge-ok" },
  stopped: { label: "DURDURYLAN", cls: "badge-off" },
  suspended: { label: "ÖÇÜRILEN", cls: "badge-warn" },
  provisioning: { label: "DÖREDILÝÄR", cls: "badge-info" },
  reinstalling: { label: "TÄZEDEN GURULÝAR", cls: "badge-info" },
  restoring: { label: "DIKELDILÝÄR", cls: "badge-info" },
  deleting: { label: "POZULÝAR", cls: "badge-warn" },
  error: { label: "ÝALŇYŞLYK", cls: "badge-danger" },
};
