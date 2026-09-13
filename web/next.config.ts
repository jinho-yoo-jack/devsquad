import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  reactStrictMode: true,
  output: "standalone",
  async rewrites() {
    // 13-Frontend 설계 §2: Control Plane REST 를 same-origin 으로 프록시
    const target = process.env.CONTROL_PLANE_URL ?? "http://localhost:8080";
    return [{ source: "/api/:path*", destination: `${target}/api/:path*` }];
  },
};

export default nextConfig;
