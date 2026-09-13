# web

Next.js 15 · React 19 · Tailwind 4 · TanStack Query · Zustand. 구조는 `docs/13-Frontend-설계.md`.

```bash
npm install
npm run dev            # http://localhost:3000  (/api/* 는 CONTROL_PLANE_URL 로 프록시)
npm run typecheck && npm test -- --run
```

골격 단계: 레이아웃·토큰·Providers·/tasks 빈 화면·Control Plane health 배지·이벤트 zod 스키마.
