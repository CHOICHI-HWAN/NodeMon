# NodeMon

> **NodeMon은 Netdata, Inc.와 아무런 관련이 없는 독립 프로젝트입니다.** Netdata, Inc.가 만들거나 후원하거나 보증한 것이 아니며, Netdata는 Netdata, Inc.의 상표입니다.
> *NodeMon is an independent project. It is not affiliated with, endorsed by, or sponsored by Netdata, Inc. Netdata is a trademark of Netdata, Inc.*

여러 컴퓨터의 자원 사용 현황(CPU·메모리·디스크·네트워크·부하·가동 시간)을 Google 계정 하나로 모아 보는 대시보드입니다.
수집은 무료 오픈소스 [Netdata 에이전트](https://learn.netdata.cloud/docs/netdata-agent/installation)에 맡기고, 이 프로젝트는 **계정 연동 · 컴퓨터 등록 · 여러 컴퓨터를 한눈에 보는 화면**을 담당합니다.

```
[각 PC] Netdata ← Connector ──(1회용 코드로 내 계정에 연결)──▶ Firebase Realtime Database ◀── 대시보드 (Firebase Hosting, Google 로그인)
```

- 서버를 직접 운영하지 않아도 되고 **Firebase 무료(Spark) 요금제**로 동작합니다. 한도를 넘으면 과금이 아니라 일시 차단됩니다.
- 컴퓨터를 추가하려면 대시보드가 만든 1회용 코드가 필요하고, 보안 규칙이 "내 계정의 기기만 읽고 등록된 기기만 자기 경로에 쓴다"를 강제합니다.
- 컴퓨터 1대당 한 달 업로드는 기본 설정에서 약 17.5MB입니다(연결이 매번 끊기면 최대 약 51MB). 실제 업로드 총량은 대시보드에서 직접 볼 수 있습니다.

## 빠른 시작

1. [Firebase 콘솔](https://console.firebase.google.com)에서 프로젝트를 만들고 Google·익명 로그인과 Realtime Database를 켭니다.
2. 배포하는 컴퓨터에서 한 줄로 배포합니다(Node.js 18+).
   ```
   git clone https://github.com/CHOICHI-HWAN/NodeMon && cd NodeMon
   node firebase/deploy.mjs --emails 내계정@gmail.com
   ```
3. `https://<사이트>.web.app`에서 Google로 로그인하고 **컴퓨터 추가**를 누릅니다. OS와 CPU를 고르면 Connector를 내려받고 등록하는 명령이 나옵니다.

## 문서

| 문서 | 내용 |
|---|---|
| [Firebase로 쓰기](docs/firebase.md) | 콘솔 설정, 배포, 업데이트, 처음 시험할 때의 문제 해결 |
| [Connector 설치](docs/connector.md) | Linux·macOS·Windows별 파일 고르기, 등록, 자동 실행, 가동 시간 |
| [데이터 구조·전송량·사용량](docs/data-and-usage.md) | 무엇을 저장하나, 한 달 업로드량 계산, 압축, Spark 한도 |
| [개발](docs/development.md) | 실행, 검사, 버전·릴리스 방법, 구성, 알려진 한계 |
| [변경 기록](CHANGELOG.md) | 버전별 변경 내용 |

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
