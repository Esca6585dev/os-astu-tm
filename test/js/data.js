// Test maglumatlary. Hakyky ulgamda bularyň hemmesi Go Fiber API-den gelýär.
window.DATA = {
  balance: 845.60,

  // VPS Hosting nyrhnamasy. Sagatlyk = aýlyk / 720. Aýda aýlyk bahadan köp tutulmaýar.
  plans: [
    { id: "a", code: "VPS A-2020", monthly: 140.00, cpu: 1, ram: 1, disk: 20,  ip: 1, speed: 8 },
    { id: "b", code: "VPS B-2020", monthly: 175.00, cpu: 1, ram: 1, disk: 40,  ip: 1, speed: 8 },
    { id: "c", code: "VPS C-2020", monthly: 297.50, cpu: 2, ram: 2, disk: 60,  ip: 1, speed: 12, popular: true },
    { id: "d", code: "VPS D-2020", monthly: 420.00, cpu: 4, ram: 4, disk: 80,  ip: 1, speed: 24 },
    { id: "e", code: "VPS E-2020", monthly: 595.00, cpu: 6, ram: 6, disk: 120, ip: 1, speed: 32 }
  ],

  images: [
    { id: "u24", name: "Ubuntu", version: "24.04 LTS", slug: "ubuntu-24_04_2025-10" },
    { id: "u22", name: "Ubuntu", version: "22.04 LTS", slug: "ubuntu-22_04_2025-06" },
    { id: "deb", name: "Debian", version: "12", slug: "debian-12_2025-09" },
    { id: "alma", name: "AlmaLinux", version: "9", slug: "almalinux-9_2025-08" },
    { id: "rocky", name: "Rocky Linux", version: "9", slug: "rocky-9_2025-08" }
  ],

  // IP salgylar resminamalar üçin bölünen 203.0.113.0/24 aralygyndan (mysal)
  servers: [
    { id: 1, name: "web-prod-01", image: "ubuntu-24_04_2025-10", ip: "203.0.113.24", plan: "c", status: "active",  created: "2026-11-02 11:42", node: "node-1", monthUsed: 68.18 },
    { id: 2, name: "db-main",     image: "debian-12_2025-09",    ip: "203.0.113.25", plan: "d", status: "active",  created: "2026-10-14 09:15", node: "node-1", monthUsed: 117.25 },
    { id: 3, name: "vpn-gw",      image: "ubuntu-24_04_2025-10", ip: "203.0.113.40", plan: "a", status: "active",  created: "2026-09-30 18:03", node: "node-2", monthUsed: 39.08 },
    { id: 4, name: "test-bot",    image: "ubuntu-22_04_2025-06", ip: "203.0.113.31", plan: "b", status: "stopped", created: "2026-08-21 12:30", node: "node-2", monthUsed: 48.86 }
  ],

  sshKeys: [
    { id: 1, name: "noutbuk-is",   type: "ed25519",  fp: "SHA256:pX3kQ2…f9aQ", added: "2026-10-02" },
    { id: 2, name: "oy-kompyuter", type: "rsa 4096", fp: "SHA256:Lm8rV7…a2cT", added: "2026-08-19" }
  ],

  transactions: [
    { at: "2026-11-09 09:00", type: "charge", note: "4 maşyn · 08:00–09:00", amount: -1.43, after: 845.60 },
    { at: "2026-11-09 08:00", type: "charge", note: "4 maşyn · 07:00–08:00", amount: -1.43, after: 847.03 },
    { at: "2026-11-08 14:20", type: "topup",  note: "Administrator · çek №2231", amount: 200.00, after: 872.77 },
    { at: "2026-11-05 16:05", type: "refund", note: "tmp-build · döredilmedi", amount: 0.41, after: 776.40 }
  ],

  users: [
    { id: "U-10482", name: "Aman Amanow",     email: "aman@mysal.tm",  servers: 4, balance: 845.60, status: "active" },
    { id: "U-10477", name: "Maýa Berdiýewa",  email: "maya@mysal.tm",  servers: 1, balance: -0.40,  status: "suspended" },
    { id: "U-10469", name: "Döwlet Orazow",   email: "dowlet@mysal.tm", servers: 0, balance: 0.00,  status: "pending" },
    { id: "U-10455", name: "Selbi Annaýewa",  email: "selbi@mysal.tm", servers: 2, balance: 58.20,  status: "active" }
  ]
};
