# NodeMon

여러 컴퓨터의 자원 사용 현황(CPU·메모리·디스크·네트워크·부하)을 Google 계정 하나로 모아 보는 대시보드.
수집은 무료 오픈소스 [Netdata 에이전트](https://learn.netdata.cloud/docs/netdata-agent/installation)에 맡기고,
이 프로젝트는 **계정 연동 · 컴퓨터 등록 · 멀티 호스트 통합 화면**을 담당합니다.

```
[각 PC] Netdata Agent (127.0.0.1:19999) ← Connector ──HTTPS push──▶ [NodeMon 서버] ◀── 브라우저(Google 로그인)
```

- `server/` Go API + SQLite + Google OAuth (빌드된 `web/`을 내장)
- `connector/` 각 PC에서 Netdata 로컬 API를 읽어 서버로 보내는 작은 Go 프로그램
- `web/` React 대시보드

## 서버 실행

1. Google Cloud Console에서 OAuth 클라이언트(웹 애플리케이션)를 만들고 리디렉션 URI에 `<BASE_URL>/auth/google/callback`을 등록합니다.
2. `.env.example`을 `.env`로 복사해 값을 채웁니다. (`ALLOWED_EMAILS`로 로그인 가능한 계정을 제한할 수 있습니다.)
3. `docker compose up -d --build`

로그인 없이 로컬에서만 시험하려면 `DEV_LOGIN=1`로 실행하고 `/auth/dev`에 접속합니다. **운영 환경에서는 절대 켜지 마세요.**
운영에서는 반드시 HTTPS 뒤에서 실행하세요(리버스 프록시: Caddy, nginx 등). 에이전트 키와 세션 쿠키가 평문으로 오가면 안 됩니다.

## 컴퓨터 등록

1. 대상 PC에 Netdata 에이전트를 설치합니다. (Netdata Cloud에 claim할 필요 없음)
2. `./build-connector.sh`로 만든 `dist/nodemon-connector-<os>-<arch>`를 그 PC에 복사합니다.
3. 대시보드의 **컴퓨터 추가**에서 등록 코드를 받아 한 번 실행합니다.
   ```
   ./nodemon-connector --server https://your.host --code XXXX-XXXX-XXXX
   ```
   기기 전용 키가 설정 파일(Linux `~/.config/nodemon/connector.json` 등, 권한 0600)에 저장됩니다.
4. 이후에는 인자 없이 실행하면 됩니다. 서비스 등록 예: `deploy/nodemon-connector.service` (systemd), Windows는 작업 스케줄러/NSSM.

## 개발

```
cd server && DEV_LOGIN=1 go run .      # :8080
cd web && npm install && npm run dev   # :5173, /api·/auth는 :8080으로 프록시
cd connector && go test ./...
```

## 알려진 한계

- Connector의 지표 매핑은 Linux 기준 Netdata 차트 이름(`system.cpu`, `system.ram`, `system.net`, `system.load`, `disk_space.*`)을 따릅니다.
  macOS/Windows용 Netdata는 일부 차트·차원 이름이 다를 수 있어 fallback을 두었지만 **실제 장비에서 검증이 필요합니다.**
  값이 비정상이면 `curl localhost:19999/api/v1/allmetrics?format=json`을 확인하고 `connector/netdata.go`의 `toSample`을 조정하세요.
- 원본 데이터(10초 간격)는 30일 보관 후 삭제됩니다. 다운샘플링·알림·팀 공유는 아직 없습니다.
