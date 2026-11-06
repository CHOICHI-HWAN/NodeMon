# Firebase(Spark 무료 요금제)로 쓰기

Spark 요금제에서는 서버 코드(Cloud Functions)를 쓸 수 없으므로 서버 없이 동작하도록 설계했습니다.
각 PC의 Connector가 Realtime Database에 직접 쓰고, **보안 규칙**이 "내 계정의 기기만 읽고, 등록된 기기만 자기 경로에 쓴다"를 강제합니다.

```
[PC] Netdata ← Connector ──(익명 계정, 1회용 코드로 내 계정에 연결)──▶ Realtime Database ◀── 대시보드(Firebase Hosting, Google 로그인)
```

## 한 번에 배포 (권장)
Node.js 18 이상만 있으면 됩니다. 값은 `firebase/project.config.json`에 들어 있습니다(Firebase 공개 식별자). 콘솔 설정(아래 1~3)을 마친 뒤 실행하세요.
```
node firebase/deploy.mjs --emails 내계정@gmail.com
```
설정 파일 생성, 보안 규칙 생성, 빌드, Firebase 로그인, 배포까지 순서대로 진행하고, 배포 직전에 한 번 확인을 묻습니다. `--dry-run`을 붙이면 로그인·배포만 빼고 모두 실행해서 준비 상태를 점검합니다. `--yes`는 확인을 건너뜁니다. 허용 계정은 한 번 입력하면 기억합니다.


## 설정
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


## 업데이트 반영하기
**GitHub에 코드를 올리는 것(push)과 Firebase에 배포하는 것은 별개입니다.** 코드가 바뀌어도 `https://<사이트>.web.app`은 다시 배포해야 바뀝니다. 배포하는 컴퓨터에서:
```
git pull
node firebase/deploy.mjs --yes
```
보안 규칙과 대시보드가 갱신됩니다(Connector 파일은 GitHub 릴리스에 있어 배포와 무관합니다). 이미 등록된 컴퓨터의 Connector는 자동으로 바뀌지 않으니, 새 기능이 필요하면 그 컴퓨터에서 새 파일을 내려받아 교체하세요(등록 정보는 그대로 유지됩니다).


## 처음 시험할 때 확인할 것
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


## 보안 참고
- 기기 자격 증명(익명 계정의 refresh token)은 PC의 설정 파일에 0600 권한으로 저장됩니다. 대시보드에서 컴퓨터를 삭제하면 그 기기는 즉시 쓰기 권한을 잃습니다.
- 규칙은 `firebase/rules.test.mjs`로 검증합니다(허용되지 않은 계정, 다른 사용자, 위조 기기, 코드 재사용, 잘못된 값 등). 에뮬레이터 실행 방법은 [개발 문서](development.md)를 보세요.


관련 문서: [Connector 설치](connector.md) · [데이터 구조와 전송량](data-and-usage.md) · [개발](development.md)
