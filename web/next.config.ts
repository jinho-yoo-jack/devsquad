import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  reactStrictMode: true,
  output: "standalone",
  async rewrites() {
    // REST and WebSocket share the browser origin and the Go service destination.
    const target = process.env.CONTROL_PLANE_URL ?? "http://localhost:8080";
    return [
      { source: "/api/:path*", destination: `${target}/api/:path*` },
      { source: "/ws", destination: `${target}/ws` },
    ];
  },
};

export default nextConfig;
