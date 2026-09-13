# control-plane

Spring Boot 3.4 · Java 21. 구조는 `docs/14-Backend-설계.md` Part A.

```bash
docker compose -f ../infra/docker-compose.yml up -d postgres redis
./gradlew bootRun          # http://localhost:8080/api/v1/health , /actuator/health , /swagger-ui.html
./gradlew test
```

골격 단계: `common/HealthController` 와 Flyway 파이프라인(V1 baseline)만 있다. 패키지 폴더(task, approval, event …)는 14-Backend A.2 순서대로 채운다.
