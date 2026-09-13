# DevSquad

승인 기반 AI 개발팀. 사용자가 정의한 AI 팀원 agent들이 한 줄 명령을 받아 **계획 → 사람 승인 → 실행 → 결과 승인**을 거치며 일하고, Discord/GitHub로 협업하고, 웹 UI에서 일하는 모습을 실시간으로 본다.

```
devsquad/
├── control-plane/   Spring Boot 3 · Java 21 — Task/Approval 상태 기계, Discord·GitHub·WebSocket 어댑터
├── agent-runtime/   Python 3.12 · FastAPI · LangGraph — pipeline.yaml → StateGraph, interrupt() 승인 게이트
├── web/             Next.js 15 · TanStack Query · Zustand — 관찰·승인 UI
├── infra/           docker-compose (postgres, redis), .env.example
├── docs/            기획·설계 문서 (10 PRD ~ 17 Agent 정의 가이드)
└── examples/
    └── devsquad-templates/   사용자 repo에 복사해 쓰는 .devsquad/ 팀원 템플릿 6종
```

## 빠른 시작 (개발)

```bash
cp infra/.env.example infra/.env            # 값 채우기
docker compose -f infra/docker-compose.yml up -d postgres redis

(cd control-plane && ./gradlew bootRun)      # :8080  GET /actuator/health
(cd agent-runtime && uv run uvicorn app.main:app --port 8100)   # :8100  GET /health
(cd web && npm install && npm run dev)       # :3000
```

## 문서

| | |
|---|---|
| [10 PRD](docs/10-PRD.md) | 무엇을, 왜, 어디까지 |
| [11 시스템 아키텍처](docs/11-시스템-아키텍처.md) | Control Plane / Agent Runtime 경계, 프로토콜, 상태 기계 |
| [12 디자인](docs/12-디자인-화면-설계.md) · [13 Frontend](docs/13-Frontend-설계.md) · [14 Backend](docs/14-Backend-설계.md) | 설계 |
| [15 API·이벤트 명세](docs/15-API-이벤트-명세.md) | 계약의 단일 기준 |
| [16 구현 로드맵](docs/16-구현-로드맵.md) | Phase 1~3 DoD와 작업 분해 |
| [17 Agent 정의 가이드](docs/17-Agent-정의-가이드.md) | 팀원을 정의하는 규격과 템플릿 |

## 상태

Phase 0 — 골격. 세 서비스가 기동하고 health check에 응답한다. 다음은 [로드맵](docs/16-구현-로드맵.md) Phase 1 (AR-1~13, CP-1~11, W-1~9).

## 라이선스

MIT — [LICENSE](LICENSE)
