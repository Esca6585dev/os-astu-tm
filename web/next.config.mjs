/** @type {import('next').NextConfig} */
const API_URL = process.env.API_URL || "http://localhost:8080";

const nextConfig = {
  output: "standalone",
  poweredByHeader: false,
  // Brauzer /api/... soraglaryny şol bir domende iberýär; Next olary Go API-e geçirýär.
  // Production-da nginx /api-ni gönüden-göni API-e geçirýär.
  async rewrites() {
    return [{ source: "/api/:path*", destination: `${API_URL}/api/:path*` }];
  },
};

export default nextConfig;
