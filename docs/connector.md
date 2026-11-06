# Connector 설치

Connector는 Netdata가 설치된 컴퓨터마다 하나씩 실행합니다. 대시보드의 **컴퓨터 추가** 창에서 OS와 CPU 종류를 고르면 아래 내용이 그대로 나옵니다.

**가장 쉬운 방법: GitHub 릴리스의 ZIP.** 컴퓨터 종류마다 ZIP이 하나씩 있고(프로그램 + 라이선스 + 서드파티 고지), 컴퓨터 추가 창의 "바로 다운로드"가 등록할 컴퓨터의 터미널에서 실행할 명령을 만들어 줍니다. 항상 최신 릴리스를 가리킵니다.
```
curl -fLO https://github.com/CHOICHI-HWAN/NodeMon/releases/latest/download/nodemon-connector-linux-amd64.zip && unzip -o nodemon-connector-linux-amd64.zip     # Linux, macOS
Invoke-WebRequest https://github.com/CHOICHI-HWAN/NodeMon/releases/latest/download/nodemon-connector-windows-amd64.zip -OutFile nodemon-connector-windows-amd64.zip; Expand-Archive nodemon-connector-windows-amd64.zip -DestinationPath . -Force   # Windows PowerShell
```
ZIP 이름은 `nodemon-connector-<linux|darwin|windows>-<amd64|arm64|arm>.zip`이고, 풀면 `nodemon-connector-<os>-<cpu>`(Windows는 `.exe`)가 나옵니다. 같은 주소의 `SHA256SUMS`로 변조 여부를 확인할 수 있습니다: `curl -fsSL https://github.com/CHOICHI-HWAN/NodeMon/releases/latest/download/SHA256SUMS | grep <ZIP이름> | sha256sum -c -` (macOS는 `shasum -a 256 -c -`).

릴리스는 저장소에 `v0.1.0` 같은 태그를 올리면 GitHub Actions(`.github/workflows/release.yml`)가 모든 종류를 빌드해 만듭니다: `git tag -a v0.1.0 -m "NodeMon v0.1.0" && git push origin v0.1.0`. 릴리스가 아직 없으면 위 링크는 404입니다. 대시보드 배포(`firebase/deploy.mjs`)는 Go가 필요 없습니다.

ZIP을 쓰지 않을 때는 아래 방법으로 직접 만들 수 있습니다.

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

## 가동 시간(uptime) 표시
컴퓨터가 마지막으로 켜진 뒤 얼마나 지났는지를 **노드** 표의 "가동 시간" 열과 노드 상세의 "가동 시간" 타일(부팅 시각 포함)에 보여 줍니다.

- **어디서 읽나**: Netdata가 `system.uptime`을 주면 그 값을 쓰고, 없으면 Connector가 운영체제에서 직접 읽습니다. Linux는 `/proc/uptime`, macOS는 `sysctl kern.boottime`, Windows는 시스템 가동 시간(`GetTickCount64`, 작업 관리자의 "가동 시간"과 같은 값)입니다. 그 밖의 시스템에서는 표시되지 않습니다.
- **어떻게 보내나**: 업로드할 때마다 현재값에 초 단위 숫자 하나를 더 싣습니다(전송량 증가는 업로드당 약 12바이트). 대시보드는 업로드 이후 지난 시간을 더해서 업로드 사이에도 맞게 보여 줍니다. 컴퓨터가 오프라인이면 "–"입니다.
- 이 기능이 생기기 전의 Connector로 등록한 컴퓨터는 "–"로 표시되며, 새 Connector로 교체하면 보입니다. macOS와 Windows의 직접 읽기는 아직 실제 장비에서 검증하지 못했습니다(Linux는 확인함).

### 압축과 연결 재사용
업로드 데이터를 줄이기 위해 다음을 하고 있습니다. 측정값은 이 저장소의 테스트로 확인할 수 있습니다.

| 방법 | 효과 | 상태 |
|---|---|---|
| **HTTP/2 헤더 압축** | 업로드마다 같은 긴 주소(인증 토큰 포함)를 보내는데, HTTP/2에서는 두 번째부터 몇 바이트로 줄어듭니다. 요청의 본문 외 오버헤드가 939B에서 69B로 줄었습니다. | 자동 (서버가 HTTP/2를 지원하면) |
| **연결 유지** | 업로드 간격보다 오래 연결을 열어 둡니다. 같은 업로드가 유지된 연결에서는 592B, 연결이 닫혀 있었다면 2,909B였습니다(새 연결의 암호화 협상 비용). | 자동 (최소 5분, 업로드 간격의 2배) |
| **값 반올림, 쓰기 응답 생략** | 숫자를 짧게 하고, 서버가 쓴 데이터를 되돌려 받지 않습니다. | 자동 |
| **대시보드 파일 압축** | 사이트의 JS·CSS는 호스팅이 압축해서 전달합니다. | 자동 (제가 아는 호스팅 동작) |
| **업로드 본문 gzip** | 한 번에 보내는 본문(현재값+분 평균 3개)이 약 930B에서 약 350B로 **약 62% 줄었습니다**(값이 분마다 달라지는 실제와 같은 데이터로 측정). 같은 키가 반복되기 때문입니다. | **실험 옵션** `--compress` (기본 꺼짐) |

`--compress`가 기본으로 꺼져 있는 이유: Realtime Database가 압축된 **요청** 본문을 받는다는 공식 문서를 찾지 못했고, 오래된 Firebase 답변에는 클라이언트가 전송 데이터를 압축하지 않는다는 내용이 있었습니다. 실제로 Firebase **에뮬레이터는 압축된 업로드를 415(지원하지 않는 형식)로 거부**했습니다. 그때 Connector는 설계대로 같은 데이터를 압축 없이 다시 보냈고 데이터는 유실되지 않았습니다. 실제 서비스도 같을 가능성이 높아서, 이 옵션은 큰 기대를 하지 않는 것이 좋습니다. 서버가 거부하면(400/415) 로그에 "does not accept compressed uploads"를 남기고 이후로는 압축하지 않습니다. 켜 보고 싶다면 "업로드 총량"으로 효과를 확인하세요.

위의 "전송량" 표는 HTTP/1.1 기준(헤더 압축 없음, 연결 재사용 없음)의 **보수적인 추정**입니다. HTTP/2와 연결 유지가 실제로 적용되면 표보다 적게 나올 수 있습니다. 3분 간격이면 하루 약 0.5MB도 가능하다고 보지만 실제 서비스에서는 확인하지 못했으니, 몇 시간 돌린 뒤 "업로드 총량"으로 직접 확인하세요.
