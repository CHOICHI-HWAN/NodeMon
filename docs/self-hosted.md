# 자체 서버로 운영하기 (선택)

Firebase 대신 NAS·VPS에서 직접 서버를 돌리는 방식입니다. Firebase 방식과 달리 서버가 가동 시간·업로드 총량 표시를 지원하지 않습니다.

## 자체 서버 실행

1. Google Cloud Console에서 OAuth 클라이언트(웹 애플리케이션)를 만들고 리디렉션 URI에 `<BASE_URL>/auth/google/callback`을 등록합니다.
2. `.env.example`을 `.env`로 복사해 값을 채웁니다. (`ALLOWED_EMAILS`로 로그인 가능한 계정을 제한할 수 있습니다.)
3. `docker compose up -d --build`

로그인 없이 로컬에서만 시험하려면 `DEV_LOGIN=1`로 실행하고 `/auth/dev`에 접속합니다. **운영 환경에서는 절대 켜지 마세요.**
운영에서는 반드시 HTTPS 뒤에서 실행하세요(리버스 프록시: Caddy, nginx 등). 에이전트 키와 세션 쿠키가 평문으로 오가면 안 됩니다.


## 시놀로지 NAS에 배포 (DSM 7.2+, Container Manager)

1. **소스 올리기**: 이 저장소를 ZIP으로 받아 NAS의 `/volume1/docker/nodemon`에 풀어 둡니다. (File Station 사용)
2. **도메인과 인증서**: 제어판 > 외부 액세스 > DDNS에서 `xxx.synology.me` 주소를 만들고, 제어판 > 보안 > 인증서에서 Let's Encrypt 인증서를 발급합니다.
3. **리버스 프록시**: 제어판 > 로그인 포털 > 고급 > 리버스 프록시에서 `HTTPS · nodemon.xxx.synology.me · 443` 을 `HTTP · localhost · 8080` 으로 연결하고 위 인증서를 지정합니다. 공유기에서 443(인증서 발급 시 80도)을 NAS로 포트 포워딩합니다.
4. **Google OAuth**: Google Cloud Console에서 OAuth 클라이언트(웹)를 만들고 리디렉션 URI에 `https://nodemon.xxx.synology.me/auth/google/callback` 을 등록합니다. 동의 화면은 "테스트" 상태에 본인 계정을 테스트 사용자로 추가하면 심사 없이 쓸 수 있습니다.
5. **프로젝트 생성**: Container Manager > 프로젝트 > 생성에서 경로를 `/volume1/docker/nodemon`으로 지정하고, 같은 폴더에 `.env`(`.env.example` 참고)를 만든 뒤 `docker-compose.yml`로 빌드·실행합니다. NAS에서 직접 빌드하므로 Intel/ARM 모델 모두 동작합니다.
6. 각 PC의 Connector에는 `--server https://nodemon.xxx.synology.me` 를 사용합니다.

외부에 여는 것은 이 앱의 리버스 프록시 하나뿐이어야 합니다. `ALLOWED_EMAILS`에 본인 계정을 넣고, DSM 관리 포트(5000/5001)는 포워딩하지 마세요.
Google 로그인은 `localhost` 외의 주소에서는 HTTPS 도메인이 필요하므로, 집 안에서만 쓰더라도 위 도메인·인증서 설정이 필요합니다.


## 컴퓨터 등록

1. 대상 PC에 Netdata 에이전트를 설치합니다. (Netdata Cloud에 claim할 필요 없음)
2. `./build-connector.sh`로 만든 `dist/nodemon-connector-<os>-<arch>`를 그 PC에 복사합니다.
3. 대시보드의 **컴퓨터 추가**에서 등록 코드를 받아 한 번 실행합니다.
   ```
   ./nodemon-connector --server https://your.host --code XXXX-XXXX-XXXX
   ```
   기기 전용 키가 설정 파일(Linux `~/.config/nodemon/connector.json` 등, 권한 0600)에 저장됩니다.
4. 이후에는 인자 없이 실행하면 됩니다. 서비스 등록 예: `deploy/nodemon-connector.service` (systemd), Windows는 작업 스케줄러/NSSM.
