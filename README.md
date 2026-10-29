# NodeMon

여러 컴퓨터의 자원 사용 현황(CPU·메모리·디스크·네트워크·부하)을 Google 계정 하나로 모아 보는 대시보드.
수집은 무료 오픈소스 [Netdata 에이전트](https://learn.netdata.cloud/docs/netdata-agent/installation)에 맡기고,
이 프로젝트는 **계정 연동 · 컴퓨터 등록 · 멀티 호스트 통합 화면**을 담당합니다.

```
[각 PC] Netdata Agent (127.0.0.1:19999) ← Connector ──HTTPS push──▶ [NodeMon 서버] ◀── 브라우저(Google 로그인)
```

- `connector/` 각 PC에서 Netdata 로컬 API를 읽어 보내는 작은 Go 프로그램 (서버 방식 / Firebase 방식 선택)
- `web/` React 대시보드 (서버 방식 / Firebase 방식 / 데모 빌드)
- `firebase/` Firebase 보안 규칙 생성기와 규칙 테스트
- `server/` (선택) 직접 운영하는 Go API + SQLite + Google OAuth

**백엔드는 둘 중 하나를 고릅니다.**

| | Firebase (Spark 무료 요금제) | 자체 서버 |
|---|---|---|
| 서버 운영 | 필요 없음 | NAS·VPS 등에서 직접 실행 |
| 로그인 | Firebase Auth (Google) | Google OAuth 직접 구현 |
| 데이터 | Realtime Database | SQLite |
| 비용 | 0원 (한도 초과 시 과금이 아니라 일시 차단) | 서버 비용 |

## Firebase(Spark 무료 요금제)로 쓰기

Spark 요금제에서는 서버 코드(Cloud Functions)를 쓸 수 없으므로 서버 없이 동작하도록 설계했습니다.
각 PC의 Connector가 Realtime Database에 직접 쓰고, **보안 규칙**이 "내 계정의 기기만 읽고, 등록된 기기만 자기 경로에 쓴다"를 강제합니다.

```
[PC] Netdata ← Connector ──(익명 계정, 1회용 코드로 내 계정에 연결)──▶ Realtime Database ◀── 대시보드(Firebase Hosting, Google 로그인)
```

### 설정
1. [Firebase 콘솔](https://console.firebase.google.com)에서 프로젝트를 만듭니다. (Spark 요금제 그대로)
2. **Authentication > 로그인 방법**에서 **Google**과 **익명**을 모두 사용 설정합니다. 익명은 각 PC(Connector)가 자기 신원을 갖는 데 씁니다.
3. **Realtime Database**를 만들고(잠금 모드), 표시되는 URL을 메모합니다. (리전에 따라 `https://<이름>-default-rtdb.<리전>.firebasedatabase.app` 형태)
4. **프로젝트 설정 > 내 앱**에서 웹 앱을 등록하고 `apiKey`, `authDomain`, `databaseURL`, `projectId`를 `web/.env.firebase.local`에 넣습니다. (`web/.env.firebase.example` 참고. 이 값들은 공개되어도 되는 식별자입니다.)
5. 로그인을 허용할 Google 계정을 넣어 보안 규칙을 만들고 배포합니다.
   ```
   cp .firebaserc.example .firebaserc      # 프로젝트 ID 수정
   ALLOWED_EMAILS=me@gmail.com node firebase/gen-rules.mjs
   (cd web && npm install && npm run build:firebase)
   npx firebase-tools login
   npx firebase-tools deploy --only database,hosting
   ```
   허용 계정을 바꾸려면 `ALLOWED_EMAILS`를 고쳐 다시 생성하고 `deploy --only database` 하면 됩니다.
6. `https://<프로젝트>.web.app`에 접속해 Google로 로그인하고 **컴퓨터 추가**에서 나오는 명령을 각 PC에서 한 번 실행합니다.
   ```
   ./nodemon-connector --firebase-api-key <키> --firebase-db-url <DB URL> --code XXXX-XXXX-XXXX
   ```
   이후에는 `./nodemon-connector`만 실행하면 됩니다. 데이터를 더 아끼려면 `--upload-every 5m`을 붙이세요.

### 데이터 구조
- Connector는 `--interval`(기본 10초)마다 측정하지만, **업로드는 `--upload-every`(기본 1분)마다 한 번**만 합니다. 그 사이의 측정값은 PC 안에서 평균을 내고, 현재값과 끝난 1분 평균(필요하면 1시간 평균)을 요청 **한 건**에 묶어 보냅니다.
- `latest/<기기>`: 현재값. `m/<기기>/<날짜>`: 1분 평균(2일 보관). `h/<기기>/<월>`: 1시간 평균(100일 보관). 오래된 데이터는 Connector가 스스로 지웁니다.
- 업로드가 실패하면 끝난 구간을 PC에 쌓아 두었다가 다음 성공 때 함께 보냅니다(오프라인 뒤 자동 보충). 기본 방식(여러 경로를 한 번에 쓰기)이 거부되면 경로별 쓰기로 자동 전환합니다.
- 숫자는 반올림해서(CPU·부하 소수 2자리, 나머지 정수) 요청 크기를 고정합니다.
- 15분~24시간 보기는 분 평균, 7일·30일 보기는 시간 평균을 읽습니다. 대시보드는 이미 받은 구간은 다시 받지 않고 **새로 생긴 구간만** 받아 이어 붙이며, 탭이 숨겨지면 갱신을 멈춥니다.

### 전송량 (PC 1대, 하루 업로드)
요청 1건은 약 0.8~1.4KB이고 대부분이 URL에 붙는 인증 토큰입니다. 그래서 **요청 횟수를 줄이는 것**이 가장 효과적입니다. 측정은 Firebase 에뮬레이터 앞에서 실제 Connector의 요청 크기를 재고, 실제 서비스의 토큰 서명 크기를 더해 계산했습니다. 실제 서비스에서는 ±20% 정도 다를 수 있습니다.

| `--upload-every` | 하루 요청 수 | PC 1대 하루 | PC 5대 한 달 |
|---|---|---|---|
| 10초 (이전 방식, 매 측정마다 전송) | 약 10,100 | 약 11.6MB | 약 1.7GB |
| **1분 (기본)** | 약 1,500 | **약 2.0MB** | 약 0.3GB |
| 2분 | 약 750 | 약 1.2MB | 약 0.17GB |
| 5분 | 약 310 | 약 0.7MB | 약 0.1GB |
| 10분 | 약 170 | 약 0.5MB | 약 0.07GB |

실시간 표시는 업로드 간격만큼 늦어집니다(예: 5분이면 현재값이 5분마다 갱신). 1분 평균 그래프는 간격과 관계없이 같은 해상도로 보충됩니다. 데이터가 가장 중요하면 `--upload-every 5m`을 쓰세요.

### Spark 한도
- 제가 아는 Spark 한도는 DB 저장 1GB, **다운로드 월 10GB**, 동시 접속 100입니다. 5대 기준 저장량은 수 MB입니다. 업로드(쓰기)는 한도에 들어가지 않고, Connector가 쓰기 응답을 되받지 않도록(`print=silent`) 설정해 두었습니다.
- 한도는 바뀔 수 있으니 [요금제 페이지](https://firebase.google.com/pricing)에서 확인하세요. 한도를 넘으면 청구되지 않고 해당 기능이 일시적으로 막힙니다.

### 보안 참고
- 기기 자격 증명(익명 계정의 refresh token)은 PC의 설정 파일에 0600 권한으로 저장됩니다. 대시보드에서 컴퓨터를 삭제하면 그 기기는 즉시 쓰기 권한을 잃습니다.
- 규칙은 `firebase/rules.test.mjs`로 검증합니다(허용되지 않은 계정, 다른 사용자, 위조 기기, 코드 재사용, 잘못된 값 등). 에뮬레이터 실행 방법은 아래 "개발"을 보세요.

## 자체 서버 실행

1. Google Cloud Console에서 OAuth 클라이언트(웹 애플리케이션)를 만들고 리디렉션 URI에 `<BASE_URL>/auth/google/callback`을 등록합니다.
2. `.env.example`을 `.env`로 복사해 값을 채웁니다. (`ALLOWED_EMAILS`로 로그인 가능한 계정을 제한할 수 있습니다.)
3. `docker compose up -d --build`

로그인 없이 로컬에서만 시험하려면 `DEV_LOGIN=1`로 실행하고 `/auth/dev`에 접속합니다. **운영 환경에서는 절대 켜지 마세요.**
운영에서는 반드시 HTTPS 뒤에서 실행하세요(리버스 프록시: Caddy, nginx 등). 에이전트 키와 세션 쿠키가 평문으로 오가면 안 됩니다.

## (자체 서버 방식) 시놀로지 NAS에 배포 (DSM 7.2+, Container Manager)

1. **소스 올리기**: 이 저장소를 ZIP으로 받아 NAS의 `/volume1/docker/nodemon`에 풀어 둡니다. (File Station 사용)
2. **도메인과 인증서**: 제어판 > 외부 액세스 > DDNS에서 `xxx.synology.me` 주소를 만들고, 제어판 > 보안 > 인증서에서 Let's Encrypt 인증서를 발급합니다.
3. **리버스 프록시**: 제어판 > 로그인 포털 > 고급 > 리버스 프록시에서 `HTTPS · nodemon.xxx.synology.me · 443` 을 `HTTP · localhost · 8080` 으로 연결하고 위 인증서를 지정합니다. 공유기에서 443(인증서 발급 시 80도)을 NAS로 포트 포워딩합니다.
4. **Google OAuth**: Google Cloud Console에서 OAuth 클라이언트(웹)를 만들고 리디렉션 URI에 `https://nodemon.xxx.synology.me/auth/google/callback` 을 등록합니다. 동의 화면은 "테스트" 상태에 본인 계정을 테스트 사용자로 추가하면 심사 없이 쓸 수 있습니다.
5. **프로젝트 생성**: Container Manager > 프로젝트 > 생성에서 경로를 `/volume1/docker/nodemon`으로 지정하고, 같은 폴더에 `.env`(`.env.example` 참고)를 만든 뒤 `docker-compose.yml`로 빌드·실행합니다. NAS에서 직접 빌드하므로 Intel/ARM 모델 모두 동작합니다.
6. 각 PC의 Connector에는 `--server https://nodemon.xxx.synology.me` 를 사용합니다.

외부에 여는 것은 이 앱의 리버스 프록시 하나뿐이어야 합니다. `ALLOWED_EMAILS`에 본인 계정을 넣고, DSM 관리 포트(5000/5001)는 포워딩하지 마세요.
Google 로그인은 `localhost` 외의 주소에서는 HTTPS 도메인이 필요하므로, 집 안에서만 쓰더라도 위 도메인·인증서 설정이 필요합니다.

## 컴퓨터 등록 (자체 서버 방식)

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
cd web && npm run build:demo        # 가상 데이터로 동작하는 데모 번들(dist-demo)

# Firebase 규칙·Connector 테스트 (에뮬레이터 필요: Java + firebase-tools)
ALLOWED_EMAILS=owner@example.com,other@example.com node firebase/gen-rules.mjs
npx firebase-tools emulators:start --only auth,database --project demo-nodemon   # 다른 터미널
node firebase/rules.test.mjs
(cd connector && NM_FIREBASE_EMULATOR=1 go test ./... -run Emulator)
```

## 알려진 한계

- Firebase 방식의 상세 차트는 1분 평균이 최소 단위입니다(최신 값은 차트 끝에 이어 붙임). 허용 계정은 규칙에 박히므로 변경 시 규칙을 다시 배포해야 합니다.
- Connector의 지표 매핑은 Linux 기준 Netdata 차트 이름(`system.cpu`, `system.ram`, `system.net`, `system.load`, `disk_space.*`)을 따릅니다.
  macOS/Windows용 Netdata는 일부 차트·차원 이름이 다를 수 있어 fallback을 두었지만 **실제 장비에서 검증이 필요합니다.**
  값이 비정상이면 `curl localhost:19999/api/v1/allmetrics?format=json`을 확인하고 `connector/netdata.go`의 `toSample`을 조정하세요.
- 원본 데이터(10초 간격)는 30일 보관 후 삭제됩니다. 다운샘플링·알림·팀 공유는 아직 없습니다.
