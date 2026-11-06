# 개발

## 실행

```
cd server && DEV_LOGIN=1 go run .      # 자체 서버 방식, :8080
cd web && npm install && npm run dev   # :5173, /api·/auth는 :8080으로 프록시
cd web && npm run build:demo           # 가상 데이터로 동작하는 데모 번들(dist-demo)
```

## 검사 (CI가 같은 명령을 실행합니다)

```
cd connector && gofmt -l . && go vet ./... && go test -race ./...
cd web && npx tsc --noEmit && npm run build && npx knip        # 안 쓰는 코드 탐지
node scripts/check-targets.mjs                                  # 빌드 대상 목록 3곳이 같은지
node scripts/third-party-notices.mjs                            # 서드파티 라이선스 고지 갱신(호환되지 않는 라이선스가 있으면 실패)
```

Firebase 규칙과 Connector 통합 테스트는 에뮬레이터가 필요합니다(Java 21 + firebase-tools).

```
ALLOWED_EMAILS=owner@example.com,other@example.com node firebase/gen-rules.mjs
npx firebase-tools emulators:exec --only auth,database --project demo-nodemon \
  "node firebase/rules.test.mjs && cd connector && NM_FIREBASE_EMULATOR=1 go test ./..."
```

## 구성

| 경로 | 내용 |
|---|---|
| `connector/` | 각 컴퓨터에서 도는 Go 프로그램. `options.go`(옵션 검증) · `runner.go`(반복과 종료 처리) · `netdata.go`(Netdata 읽기) · `sample.go`(측정값과 평균) · `fbclient.go`·`fbenroll.go`·`fbsink.go`(Firebase) · `serverbackend.go`(자체 서버) · `usage.go`·`uptime*.go` |
| `web/` | React 대시보드. 데이터 소스는 `firebaseApi.ts`(Firebase), `api.ts`(자체 서버), `demo.ts`(데모) |
| `firebase/` | 보안 규칙 생성기(`gen-rules.mjs`), 규칙 테스트, 한 번에 배포하는 `deploy.mjs` |
| `server/` | 자체 서버(Go, SQLite) |
| `scripts/` | 빌드·라이선스 고지·동기화 검사 |

## 알려진 한계

- Firebase 방식의 상세 차트는 1분 평균이 최소 단위입니다(최신 값은 차트 끝에 이어 붙임). 분 평균은 2일, 시간 평균은 100일 보관합니다. 알림·팀 공유는 아직 없습니다.
- 허용 계정은 보안 규칙에 들어가므로 바꾸려면 규칙을 다시 배포해야 합니다.
- Connector의 지표 매핑은 Linux 기준 Netdata 차트 이름(`system.cpu`, `system.ram`, `system.net`, `system.load`, `system.uptime`, `disk_space.*`)을 따릅니다. macOS/Windows용 Netdata는 일부 차트·차원 이름이 다를 수 있어 대체 로직을 두었지만 **실제 장비에서 검증이 필요합니다.** 값이 비정상이면 `curl localhost:19999/api/v1/allmetrics?format=json`을 확인하고 `connector/netdata.go`의 `toSample`을 조정하세요.
- 자체 서버 방식에는 가동 시간과 업로드 총량 표시가 없고 테스트도 없습니다.
