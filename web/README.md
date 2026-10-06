# web

Next.js 15 · React 19 · Tailwind 4 · TanStack Query · Zustand. 구조는 `docs/13-Frontend-설계.md`.

```bash
npm install
npm run dev            # http://localhost:3000  (/api/* 와 /ws 는 CONTROL_PLANE_URL 로 프록시)
npm run typecheck && npm test -- --run
npx playwright install chromium
npm run e2e            # 별도 3300 포트에서 브라우저 테스트 (HTTP/WS fixture, 모델 호출 없음)
```

Task 목록·생성·상세, 승인 패널과 가상 타임라인을 제공한다. Task 상세의 **에이전트 작업 현황**에서 단계별 현재 동작, 사용 도구와 결과, 승인·선행 단계 대기 이유, 모델과 호출 회차, 마지막 활동 시각을 확인한다. 카드를 선택하면 그 단계의 기록만 표시한다. 같은 에이전트가 여러 단계를 맡아도 기록을 분리한다.

상태는 REST, 작업 내용은 WebSocket 이벤트로 갱신한다. 새로고침·재연결 시 저장된 이벤트를 페이지 끝까지 병합한다. 연결이 끊어지면 마지막 수신 상태임을 표시하고, Task가 취소·실패하면 진행 중 표시를 중단한다. 일시정지는 새 단계의 시작을 막으며 이미 진행 중인 작업은 마무리될 수 있다.

`CONTROL_PLANE_URL` 기본값은 `http://localhost:8080`이다. Next.js는 프록시 목적지를 빌드에 저장하므로 프로덕션에서는 **빌드 시점**에 지정한다. Docker 빌드는 같은 이름의 build arg를 받으며 Compose에서 `http://devsquad:8080`으로 설정한다.
