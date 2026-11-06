# NodeMon

> **NodeMon은 Netdata, Inc.와 아무런 관련이 없는 독립 프로젝트입니다.** Netdata, Inc.가 만들거나 후원하거나 보증한 것이 아니며, Netdata는 Netdata, Inc.의 상표입니다.
> *NodeMon is an independent project. It is not affiliated with, endorsed by, or sponsored by Netdata, Inc. Netdata is a trademark of Netdata, Inc.*

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

## 라이선스
NodeMon은 **GNU General Public License v3.0 이상(GPL-3.0-or-later)** 으로 배포됩니다. 전문은 [LICENSE](LICENSE)에 있습니다.

```
Copyright (C) 2026 CHOI AILAB
이 프로그램은 자유 소프트웨어입니다. 자유 소프트웨어 재단이 공표한 GNU GPL 버전 3 또는
(선택에 따라) 그 이후 버전의 조건에 따라 재배포하거나 수정할 수 있습니다.
이 프로그램은 유용하기를 바라며 배포되지만 어떠한 보증도 제공하지 않습니다.
```

- 소스 파일에는 `SPDX-License-Identifier: GPL-3.0-or-later` 표시가 들어 있습니다.
- 이 프로그램을 수정해서 **배포**하면(대시보드 번들, Connector 실행 파일 포함) 같은 GPL-3.0 조건으로 소스를 제공해야 합니다. 배포하지 않고 직접 서버에서 돌리기만 하는 경우에는 소스 공개 의무가 없습니다.
- 포함된 서드파티 소프트웨어와 각 라이선스는 [THIRD_PARTY_LICENSES.md](THIRD_PARTY_LICENSES.md)에 있고(대시보드에서도 `/third-party-licenses.txt`로 제공), 의존성을 바꾼 뒤에는 `node scripts/third-party-notices.mjs`로 다시 만듭니다. GPL-3.0과 호환되지 않는 라이선스가 섞이면 이 스크립트가 실패합니다.

### Netdata 관련 고지
NodeMon은 Netdata, Inc.와 제휴하거나 보증받은 프로젝트가 아닙니다. Netdata는 Netdata, Inc.의 상표입니다. 이 프로젝트는 각 PC에서 별도로 실행되는 Netdata 에이전트(GPLv3+)의 로컬 API를 HTTP로 읽을 뿐, Netdata의 코드나 대시보드 UI, 로고를 포함하지 않습니다.

## Firebase(Spark 무료 요금제)로 쓰기

Spark 요금제에서는 서버 코드(Cloud Functions)를 쓸 수 없으므로 서버 없이 동작하도록 설계했습니다.
각 PC의 Connector가 Realtime Database에 직접 쓰고, **보안 규칙**이 "내 계정의 기기만 읽고, 등록된 기기만 자기 경로에 쓴다"를 강제합니다.

```
[PC] Netdata ← Connector ──(익명 계정, 1회용 코드로 내 계정에 연결)──▶ Realtime Database ◀── 대시보드(Firebase Hosting, Google 로그인)
```

### 한 번에 배포 (권장)
Node.js 18 이상만 있으면 됩니다. 값은 `firebase/project.config.json`에 들어 있습니다(Firebase 공개 식별자). 콘솔 설정(아래 1~3)을 마친 뒤 실행하세요.
```
node firebase/deploy.mjs --emails 내계정@gmail.com
```
설정 파일 생성, 보안 규칙 생성, 빌드, Firebase 로그인, 배포까지 순서대로 진행하고, 배포 직전에 한 번 확인을 묻습니다. `--dry-run`을 붙이면 로그인·배포만 빼고 모두 실행해서 준비 상태를 점검합니다. `--yes`는 확인을 건너뜁니다. 허용 계정은 한 번 입력하면 기억합니다.

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
   `firebase.json`의 `hosting.site`(기본 `comon`)에 배포됩니다. 다른 사이트 ID를 쓰면 이 값을 바꾸세요. **같은 Firebase 프로젝트에 이미 쓰는 앱이 있다면** `deploy --only database`가 그 앱의 데이터베이스 규칙을 덮어쓰므로, 새 프로젝트를 따로 만드는 것을 권합니다.
   허용 계정을 바꾸려면 `ALLOWED_EMAILS`를 고쳐 다시 생성하고 `deploy --only database` 하면 됩니다.
6. `https://<프로젝트>.web.app`에 접속해 Google로 로그인하고 **컴퓨터 추가**에서 나오는 명령을 각 PC에서 한 번 실행합니다.
   ```
   ./nodemon-connector --firebase-api-key <키> --firebase-db-url <DB URL> --code XXXX-XXXX-XXXX
   ```
   이후에는 `./nodemon-connector`만 실행하면 됩니다(기본 3분마다 업로드). 간격은 `--upload-every 5m`처럼 바꿀 수 있습니다.

### 업데이트 반영하기
**GitHub에 코드를 올리는 것(push)과 Firebase에 배포하는 것은 별개입니다.** 코드가 바뀌어도 `https://<사이트>.web.app`은 다시 배포해야 바뀝니다. 배포하는 컴퓨터에서:
```
git pull
node firebase/deploy.mjs --yes
```
보안 규칙과 대시보드, Connector 다운로드 파일이 모두 갱신됩니다. 이미 등록된 컴퓨터의 Connector는 자동으로 바뀌지 않으니, 새 기능이 필요하면 그 컴퓨터에서 새 파일을 내려받아 교체하세요(등록 정보는 그대로 유지됩니다).

### 처음 시험할 때 확인할 것
1. 배포 후 `https://<프로젝트>.web.app`에서 **Google로 로그인**이 되는지 봅니다. (팝업 차단을 풀어 주세요. 허용 계정이 아니면 로그인은 되어도 컴퓨터 추가에서 막힙니다.)
2. **컴퓨터 추가**를 눌러 나온 명령을 Netdata가 설치된 PC에서 실행합니다. 처음에 "enrolled"가 나오면 등록 성공입니다.
3. 등록 직후 첫 업로드가 바로 되므로 **노드** 화면에 노드가 곧바로 나타납니다. 차트의 1분 평균은 몇 분 뒤부터 쌓입니다.
4. 대시보드에서 노드를 삭제하면 그 PC는 다음 업로드부터 거부됩니다(로그에 "rejected"가 나오고 대기 시간이 늘어납니다).

| 증상 | 원인과 해결 |
|---|---|
| 등록 시 `OPERATION_NOT_ALLOWED` / `ADMIN_ONLY_OPERATION` | Authentication에서 **익명** 로그인이 꺼져 있습니다. 켜 주세요. |
| 등록 시 `PERMISSION_DENIED` | 규칙을 배포하지 않았거나(`deploy --only database`), 코드가 만료/사용됐습니다(15분, 1회용). 새 코드를 받으세요. |
| 컴퓨터 추가에서 `PERMISSION_DENIED` | 로그인한 Google 계정이 `ALLOWED_EMAILS`에 없습니다. 규칙을 다시 만들어 배포하세요. |
| 로그인 팝업에서 `auth/unauthorized-domain` | Authentication > 설정 > 승인된 도메인에 접속 중인 도메인을 추가하세요. |
| 대시보드가 비어 있고 콘솔에 URL 오류 | `.env.firebase.local`의 `DATABASE_URL`이 콘솔에 표시된 값(리전 포함)과 다릅니다. 수정 후 다시 빌드·배포하세요. |
| 노드가 "오프라인"으로만 표시 | Connector가 실행 중인지, Netdata(`localhost:19999`)가 응답하는지 확인하세요. |

### Connector 설치 (자세히)
Connector는 Netdata가 설치된 컴퓨터마다 하나씩 실행합니다. 대시보드의 **컴퓨터 추가** 창에서 OS와 CPU 종류를 고르면 아래 내용이 그대로 나옵니다.

**가장 쉬운 방법: 대시보드에서 바로 내려받기.** `node firebase/deploy.mjs`로 배포하면 Connector 실행 파일도 함께 빌드되어 대시보드 사이트(`https://<사이트>.web.app/downloads/`)에 올라갑니다. 컴퓨터 추가 창의 "바로 다운로드"가 등록할 컴퓨터의 터미널에서 실행할 명령을 만들어 줍니다. 예:
```
curl -fLO https://comon.web.app/downloads/nodemon-connector-linux-amd64                     # Linux, macOS
Invoke-WebRequest https://comon.web.app/downloads/nodemon-connector-windows-amd64.exe -OutFile nodemon-connector-windows-amd64.exe   # Windows PowerShell
```
`SHA256SUMS`(같은 폴더)로 변조 여부를 확인할 수 있습니다: `curl -fsSL https://comon.web.app/downloads/SHA256SUMS | grep <파일이름> | sha256sum -c -` (macOS는 `shasum -a 256 -c -`). 배포하는 컴퓨터에 **Go가 없으면** 새로 빌드하지 못하고, 이전에 만들어 둔 `dist/`의 파일이 있으면 그것을, 없으면 다운로드 파일 없이 배포합니다(대시보드는 정상). Go 설치 후 다시 배포하면 생깁니다.

다운로드를 쓰지 않을 때는 아래 방법으로 직접 만들 수 있습니다.

**1. 파일 만들기** (Go 1.24 이상이 있는 아무 컴퓨터 한 대에서. `node scripts/build-connector.mjs`도 같은 일을 하며 체크섬 파일까지 만듭니다)
```
git clone https://github.com/CHOICHI-HWAN/NodeMon && cd NodeMon
git checkout feat/nodemon-mvp        # 아직 main에 합치기 전이라면
./build-connector.sh
```
`dist/`에 모든 종류가 만들어집니다. 등록할 컴퓨터가 Windows라면 `build-connector.sh`는 못 돌리니 다른 컴퓨터에서 만든 `.exe`를 복사하세요.

**2. 내 컴퓨터에 맞는 파일 고르기** — 등록할 컴퓨터에서 아래 명령을 실행해 나온 값으로 고릅니다.

| OS | 확인 명령 | 나온 값 | 사용할 파일 |
|---|---|---|---|
| Linux | `uname -m` | `x86_64` (일반 PC·서버·대부분의 NAS) | `nodemon-connector-linux-amd64` |
| Linux | `uname -m` | `aarch64` (라즈베리파이 4·5 64비트, ARM 서버) | `nodemon-connector-linux-arm64` |
| Linux | `uname -m` | `armv7l` (라즈베리파이 32비트 OS) | `nodemon-connector-linux-arm` |
| macOS | `uname -m` | `arm64` (M1·M2·M3·M4) | `nodemon-connector-darwin-arm64` |
| macOS | `uname -m` | `x86_64` (Intel 맥) | `nodemon-connector-darwin-amd64` |
| Windows | PowerShell: `$env:PROCESSOR_ARCHITECTURE` | `AMD64` | `nodemon-connector-windows-amd64.exe` |

위 표에 없는 종류(예: `armv6l`, Windows ARM)는 지원 파일이 없습니다. **가장 쉬운 방법은 등록할 컴퓨터에서 직접 빌드하는 것**입니다. 그 컴퓨터용 파일이 자동으로 만들어져서 종류를 고를 필요가 없습니다.
```
cd connector && go build -o nodemon-connector . && cd ..      # 파일 이름: nodemon-connector
```

**3. 등록할 컴퓨터로 복사** (예: `scp dist/nodemon-connector-linux-arm64 pi@192.168.0.10:~/`)

**4. 등록 (한 번만 실행)** — 대시보드가 보여 주는 명령을 그대로 복사해 파일이 있는 폴더에서 실행합니다.
```
# Linux / macOS (파일 이름은 2번에서 고른 것)
chmod +x ./nodemon-connector-linux-amd64 && ./nodemon-connector-linux-amd64 --firebase-api-key … --firebase-db-url … --code XXXX-XXXX-XXXX
# Windows (PowerShell)
.\nodemon-connector-windows-amd64.exe --firebase-api-key … --firebase-db-url … --code XXXX-XXXX-XXXX
```
`enrolled`가 나오면 성공입니다. 이후에도 실행이 계속되므로 시험이 끝나면 `Ctrl+C`로 멈추세요.
- macOS가 "확인되지 않은 개발자"라고 막으면 `xattr -d com.apple.quarantine ./파일이름`, `Killed: 9`로 종료되면 `codesign -s - ./파일이름`을 실행하세요.
- Windows가 "PC를 보호했습니다"라고 하면 **추가 정보 → 실행**을 누르세요.

**5. 켤 때마다 자동 실행** — 등록이 성공한 뒤 대시보드 창의 "자동 실행 설정 보기"에 나오는 명령을 실행합니다. Linux는 systemd 서비스, macOS는 launchd, Windows는 작업 스케줄러(로그인할 때 실행)로 등록합니다. macOS와 Windows 방법은 아직 실제 장비에서 검증하지 못했습니다.

### 데이터 구조
- Connector는 `--interval`(기본 10초)마다 측정하지만, **업로드는 `--upload-every`(기본 3분)마다 한 번**만 합니다. 그 사이의 측정값은 PC 안에서 평균을 내고, 현재값과 끝난 1분 평균(필요하면 1시간 평균)을 요청 **한 건**에 묶어 보냅니다.
- `latest/<기기>`: 현재값. `m/<기기>/<날짜>`: 1분 평균(2일 보관). `h/<기기>/<월>`: 1시간 평균(100일 보관). 오래된 데이터는 Connector가 스스로 지웁니다.
- 업로드가 실패하면 끝난 구간을 PC에 쌓아 두었다가 다음 성공 때 함께 보냅니다(오프라인 뒤 자동 보충). 기본 방식(여러 경로를 한 번에 쓰기)이 거부되면 경로별 쓰기로 자동 전환합니다.
- 숫자는 반올림해서(CPU·부하 소수 2자리, 나머지 정수) 요청 크기를 고정합니다.
- 15분~24시간 보기는 분 평균, 7일·30일 보기는 시간 평균을 읽습니다. 대시보드는 이미 받은 구간은 다시 받지 않고 **새로 생긴 구간만** 받아 이어 붙이며, 탭이 숨겨지면 갱신을 멈춥니다.

### 전송량 (PC 1대, 하루 업로드)
요청 1건은 약 0.8~1.4KB이고 대부분이 URL에 붙는 인증 토큰입니다. 그래서 **요청 횟수를 줄이는 것**이 가장 효과적입니다. 측정은 Firebase 에뮬레이터 앞에서 실제 Connector의 요청 크기를 재고, 실제 서비스의 토큰 서명 크기를 더해 계산했습니다. 실제 서비스에서는 ±20% 정도 다를 수 있습니다.

| `--upload-every` | 하루 요청 수 | PC 1대 하루 | PC 5대 한 달 |
|---|---|---|---|
| 10초 (이전 방식, 매 측정마다 전송) | 약 10,100 | 약 11.6MB | 약 1.7GB |
| 1분 | 약 1,500 | 약 2.0MB | 약 0.3GB |
| **3분 (기본)** | 약 530 | **약 0.9MB** | 약 0.14GB |
| 2분 | 약 750 | 약 1.2MB | 약 0.17GB |
| 5분 | 약 310 | 약 0.7MB | 약 0.1GB |
| 10분 | 약 170 | 약 0.5MB | 약 0.07GB |

실시간 표시는 업로드 간격만큼 늦어집니다(예: 5분이면 현재값이 5분마다 갱신). 1분 평균 그래프는 간격과 관계없이 같은 해상도로 보충됩니다. 더 아끼려면 `--upload-every 5m`, 더 실시간에 가깝게 보려면 `--upload-every 1m`을 쓰세요. 대시보드는 Connector가 알려 주는 업로드 간격의 2.5배(최소 90초) 안에 소식이 있으면 "실시간"으로 표시합니다.

### 컴퓨터별 업로드 총량 표시
각 컴퓨터의 Connector가 Firebase로 **실제로 보낸 데이터량**을 직접 세어서 대시보드에 보여 줍니다. 개요 화면의 "업로드 총량" 타일(전체 합계), **노드** 표의 "업로드" 열(총량과 최근 24시간), 노드 상세의 같은 타일에서 볼 수 있습니다. 받은 데이터량도 함께 셉니다.

- **무엇을 세나**: 네트워크 연결(소켓)에서 오간 바이트입니다. 암호화와 프로토콜 오버헤드가 포함되고 IP/TCP 헤더는 빠지므로, 통신사 청구 기준 사용량보다 3~5% 정도 적게 나올 수 있습니다. 로컬 Netdata에서 읽는 데이터는 세지 않습니다.
- **정확도**: Connector가 센 값과 별도로 만든 측정 프록시의 요청 바이트 합계를 비교했을 때 **0.0% 차이**였습니다(에뮬레이터 환경).
- **기준 시점**: 등록한 뒤부터의 누적이고, 몇 분 늦게 반영됩니다. 값은 Connector의 설정 파일에 저장되어 재시작하거나 PC를 껐다 켜도 이어서 쌓입니다(종료할 때와 1분마다 저장). 설정 파일을 지우고 다시 등록하면 0부터 다시 셉니다.
- **최근 24시간**: 1시간 단위 기록에 남겨 둔 누적값과의 차이로 계산합니다.
- **이 기능이 생기기 전의 Connector**로 등록한 컴퓨터는 "–"로 표시됩니다. 새 Connector로 교체하면 그때부터 셉니다. 이 값은 Firebase 콘솔의 사용량(다운로드 기준 한도)과는 별개의 참고용 값입니다.
- 이 기능을 쓰려면 규칙을 다시 배포해야 합니다(`node firebase/deploy.mjs`가 함께 처리합니다).

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
