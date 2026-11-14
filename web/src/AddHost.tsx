// SPDX-License-Identifier: GPL-3.0-or-later
import { useEffect, useState } from "react";
import { api, type EnrollCode } from "./api";
import {
  ARCHES, autostartScript, BUILD_LOCAL, checkCommand, downloadCommand, downloadUrl, fileName, guessOs, OS_LABEL, RELEASES_URL, REPO_URL, runCommand, zipName,
  type Arch, type Build, type Os,
} from "./connectorTargets";

function Cmd({ label, text }: { label?: string; text: string }) {
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      /* clipboard unavailable: the text is selectable */
    }
  };
  return (
    <div className="cmd">
      {label && <div className="cmd-label">{label}</div>}
      <div className="cmd-box">
        {text.includes("\n") ? <pre>{text}</pre> : <code>{text}</code>}
        <button className="link" onClick={copy}>{copied ? "복사됨" : "복사"}</button>
      </div>
    </div>
  );
}

export function AddHost({ onClose }: { onClose: () => void }) {
  const [code, setCode] = useState<EnrollCode | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [os, setOs] = useState<Os>(guessOs);
  const [arch, setArch] = useState<Arch>("amd64");
  const [build, setBuild] = useState<Build>("download");
  const [name, setName] = useState("");

  useEffect(() => {
    api.createCode().then(setCode).catch((e) => setError(String(e)));
    const esc = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", esc);
    return () => window.removeEventListener("keydown", esc);
  }, [onClose]);

  const pickOs = (next: Os) => {
    setOs(next);
    setArch(ARCHES[next][0].arch); // default to the first (most common) CPU type of that OS
  };
  const file = fileName(os, arch, build);
  // The name goes into a shell command line, so characters that a shell would interpret are dropped.
  const cleanName = name.replace(/["'`$\\%^&|<>;\r\n]/g, "").trim();
  const args = code ? `${code.args}${cleanName ? ` --name "${cleanName}"` : ""}` : "";

  return (
    <div className="overlay" onClick={onClose}>
      <div className="modal card" role="dialog" aria-modal="true" aria-label="컴퓨터 추가" onClick={(e) => e.stopPropagation()}>
        <header className="modal-head">
          <h2>컴퓨터 추가</h2>
          <button className="link" onClick={onClose}>닫기</button>
        </header>
        {error && <p className="err">{error}</p>}
        {!code && !error && <p className="sub">등록 코드를 만드는 중…</p>}
        {code && (
          <>
            <p className="sub">등록 코드 (15분 안에 한 번만 사용 가능)</p>
            <div className="code">{code.code}</div>

            <h3 className="step-title">1. Netdata 설치</h3>
            <p className="sub">
              등록할 컴퓨터에 <a href="https://learn.netdata.cloud/docs/netdata-agent/installation" target="_blank" rel="noreferrer">Netdata 에이전트</a>를 설치합니다.
              Netdata Cloud 연결(claim)은 필요 없습니다. Netdata는 별도로 설치하는 타사 소프트웨어이며 NodeMon과 무관합니다.
            </p>

            <h3 className="step-title">2. 컴퓨터 이름과 종류 선택</h3>
            <label htmlFor="pc-name" className="sub">컴퓨터 이름 (대시보드에 표시됩니다. 비워 두면 그 컴퓨터의 호스트 이름을 씁니다)</label>
            <input id="pc-name" className="search" type="text" maxLength={60} placeholder="예: 사무실 NAS" value={name} onChange={(e) => setName(e.target.value)} />
            <div className="seg" role="group" aria-label="운영체제">
              {(Object.keys(OS_LABEL) as Os[]).map((o) => (
                <button key={o} className={os === o ? "active" : ""} aria-pressed={os === o} onClick={() => pickOs(o)}>{OS_LABEL[o]}</button>
              ))}
            </div>
            {ARCHES[os].length > 1 && (
              <>
                <label htmlFor="arch" className="sub">CPU 종류</label>
                <select id="arch" className="select" value={arch} onChange={(e) => setArch(e.target.value as Arch)}>
                  {ARCHES[os].map((a) => <option key={a.arch} value={a.arch}>{a.label}</option>)}
                </select>
              </>
            )}
            <details className="help">
              <summary>내 컴퓨터가 어떤 종류인지 모르겠어요</summary>
              <p className="sub">등록할 컴퓨터의 터미널{os === "windows" ? "(PowerShell)" : ""}에서 아래 명령을 실행하고 나온 값을 보세요.</p>
              <Cmd text={checkCommand(os)} />
              <ul className="sub">
                {ARCHES[os].map((a) => <li key={a.arch}><code>{a.reported}</code> → {a.label.split(" — ")[1] ?? a.label}</li>)}
              </ul>
            </details>

            <h3 className="step-title">3. Connector 프로그램 준비</h3>
            <div className="seg" role="group" aria-label="준비 방법">
              <button className={build === "download" ? "active" : ""} aria-pressed={build === "download"} onClick={() => setBuild("download")}>바로 다운로드</button>
              <button className={build === "dist" ? "active" : ""} aria-pressed={build === "dist"} onClick={() => setBuild("dist")}>빌드한 파일 복사</button>
              <button className={build === "local" ? "active" : ""} aria-pressed={build === "local"} onClick={() => setBuild("local")}>직접 빌드</button>
            </div>
            {build === "download" && (
              <>
                <p className="sub">
                  등록할 컴퓨터의 터미널{os === "windows" ? "(PowerShell)" : ""}에서 GitHub 릴리스의 ZIP을 내려받아 풉니다. 풀면 <strong className="file">{file}</strong> 이 생깁니다.
                </p>
                <Cmd text={downloadCommand(os, arch)} />
                <p className="sub">
                  브라우저로 받으려면 <a href={downloadUrl(zipName(os, arch))}>{zipName(os, arch)}</a> ·{" "}
                  <a href={downloadUrl("SHA256SUMS")}>체크섬(SHA256SUMS)</a>으로 변조 여부를 확인할 수 있습니다 ·{" "}
                  <a href={RELEASES_URL} target="_blank" rel="noreferrer">모든 릴리스</a>.
                  GPL-3.0-or-later 프로그램이며 <a href={REPO_URL} target="_blank" rel="noreferrer">소스 코드</a>를 공개합니다.
                  {os === "darwin" && " macOS에서는 curl로 받으면 ‘확인되지 않은 개발자’ 경고가 나오지 않습니다."}
                </p>
                <p className="sub">링크가 404이면 아직 릴리스가 없는 것입니다. 저장소 관리자가 <code>v0.1.0</code> 같은 태그를 올리면 자동으로 만들어집니다. 그 전에는 “직접 빌드”를 쓰세요.</p>
              </>
            )}
            {build === "dist" && (
              <p className="sub">
                저장소에서 <code>./build-connector.sh</code>를 실행하면 <code>dist/</code> 폴더에 모든 종류의 파일이 만들어집니다.
                그중 <strong className="file">{file}</strong> 을 등록할 컴퓨터로 복사하세요(USB, <code>scp</code> 등).
              </p>
            )}
            {build === "local" && (
              <>
                <p className="sub">등록할 컴퓨터에 Go(1.24 이상)와 저장소가 있으면 CPU 종류를 고를 필요 없이 그 컴퓨터용 파일이 만들어집니다. 저장소 폴더에서:</p>
                <Cmd text={BUILD_LOCAL} />
                <p className="sub">만들어진 파일은 <strong className="file">{file}</strong> 입니다(저장소 폴더 안).</p>
              </>
            )}

            <h3 className="step-title">4. 등록 (한 번만 실행)</h3>
            <p className="sub">
              파일이 있는 폴더의 터미널{os === "windows" ? "(PowerShell)" : ""}에서 실행합니다. 등록 코드{cleanName ? " 와 컴퓨터 이름" : ""}이 명령에 이미 들어 있습니다.
              {!cleanName && " 이름을 정하지 않아 그 컴퓨터의 호스트 이름으로 표시됩니다."}
            </p>
            <Cmd text={runCommand(os, file, args)} />
            <p className="sub">
              <code>enrolled</code>가 나오면 등록 성공이고, 이 화면의 <strong>노드</strong>에 곧 나타납니다. 이후에도 실행이 계속되므로 시험이 끝나면 <code>Ctrl+C</code>로 멈추세요.
              기본으로 3분마다 업로드하며, 명령 끝에 <code>--upload-every 5m</code>을 붙이면 간격을 바꿀 수 있습니다.
            </p>
            {os === "darwin" && (
              <details className="help">
                <summary>macOS가 실행을 막을 때</summary>
                <p className="sub">"확인되지 않은 개발자" 경고가 나오면(브라우저로 내려받은 파일):</p>
                <Cmd text={`xattr -d com.apple.quarantine ./${file}`} />
                <p className="sub"><code>Killed: 9</code>로 종료되면:</p>
                <Cmd text={`codesign -s - ./${file}`} />
              </details>
            )}
            {os === "windows" && (
              <p className="sub">Windows가 "PC를 보호했습니다" 창을 띄우면 <strong>추가 정보 → 실행</strong>을 누르세요.</p>
            )}

            <h3 className="step-title">5. 켤 때마다 자동으로 실행 (선택)</h3>
            <details className="help">
              <summary>자동 실행 설정 보기</summary>
              <p className="sub">4번에서 등록이 성공한 뒤에 하세요. 등록할 때 저장된 자격 증명을 그대로 씁니다.</p>
              <Cmd text={autostartScript(os, file, window.location.origin)} />
              {os === "windows" && (
                <p className="sub">
                  로그인할 때 창 없이 시계 옆 아이콘으로 실행됩니다. 아이콘 색이 상태(초록 정상, 노랑 문제, 빨강 멈춤)이고, 오른쪽 클릭하면
                  마지막 업로드 시각, 대시보드 열기, 로그 보기, 종료가 있습니다. Windows 11에서는 처음에 ^ 안에 숨겨져 있으니 작업 표시줄로 끌어다 놓으세요.
                  ※ 이 방법은 아직 실제 {OS_LABEL[os]} 장비에서 충분히 검증하지 못했습니다. 동작하지 않으면 알려 주세요.
                </p>
              )}
            </details>
          </>
        )}
      </div>
    </div>
  );
}
