// SPDX-License-Identifier: GPL-3.0-or-later
import { useEffect, useState } from "react";
import { api, type EnrollCode } from "./api";
import {
  ARCHES, autostartScript, BUILD_LOCAL, checkCommand, downloadCommand, downloadUrl, fileName, guessOs, OS_LABEL, runCommand,
  type Arch, type Build, type Os,
} from "./connectorTargets";
import { DEMO } from "./env";

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
  const [available, setAvailable] = useState<boolean | null>(null); // is the file really hosted by this site?
  const origin = DEMO ? "https://your-site.web.app" : window.location.origin;

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
  const args = code?.args ?? "";

  useEffect(() => {
    if (DEMO) return setAvailable(true);
    let stop = false;
    setAvailable(null);
    fetch(downloadUrl(origin, fileName(os, arch, "download")), { method: "HEAD" })
      // A server that answers unknown paths with the app's index page would also say 200, so check the type too.
      .then((r) => !stop && setAvailable(r.ok && !(r.headers.get("content-type") ?? "").includes("text/html")))
      .catch(() => !stop && setAvailable(false));
    return () => {
      stop = true;
    };
  }, [os, arch, origin]);

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

            <h3 className="step-title">2. 등록할 컴퓨터의 종류 선택</h3>
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
              available === false ? (
                <p className="err">
                  이 사이트에는 <strong className="file">{file}</strong> 파일이 아직 없습니다. 배포한 컴퓨터에 Go가 없었을 수 있습니다.
                  Go를 설치하고 배포 스크립트를 다시 실행하면 생기고, 그 전에는 “빌드한 파일 복사”나 “직접 빌드”를 쓰세요.
                </p>
              ) : (
                <>
                  <p className="sub">등록할 컴퓨터의 터미널{os === "windows" ? "(PowerShell)" : ""}에서 내려받습니다. <strong className="file">{file}</strong></p>
                  <Cmd text={downloadCommand(os, origin, file)} />
                  <p className="sub">
                    브라우저로 받으려면 <a href={downloadUrl(origin, file)} download>{file}</a> ·{" "}
                    <a href={downloadUrl(origin, "SHA256SUMS")} target="_blank" rel="noreferrer">체크섬(SHA256SUMS)</a>으로 파일이 변조되지 않았는지 확인할 수 있습니다.
                    GPL-3.0-or-later 프로그램이며 <a href="https://github.com/CHOICHI-HWAN/NodeMon" target="_blank" rel="noreferrer">소스 코드</a>를 공개합니다.
                    {os === "darwin" && " macOS에서는 curl로 받으면 ‘확인되지 않은 개발자’ 경고가 나오지 않습니다."}
                  </p>
                </>
              )
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
            <p className="sub">파일이 있는 폴더의 터미널{os === "windows" ? "(PowerShell)" : ""}에서 실행합니다.</p>
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
              <Cmd text={autostartScript(os, file)} />
              {os !== "linux" && (
                <p className="sub">※ 이 방법은 아직 실제 {OS_LABEL[os]} 장비에서 검증하지 못했습니다. 동작하지 않으면 알려 주세요.</p>
              )}
            </details>
          </>
        )}
      </div>
    </div>
  );
}
