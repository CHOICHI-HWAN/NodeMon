import { useEffect, useState } from "react";
import { api, type EnrollCode } from "./api";

function Cmd({ label, text }: { label: string; text: string }) {
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      /* clipboard unavailable: user can select the text */
    }
  };
  return (
    <div className="cmd">
      <div className="cmd-label">{label}</div>
      <div className="cmd-box">
        <code>{text}</code>
        <button className="link" onClick={copy}>{copied ? "복사됨" : "복사"}</button>
      </div>
    </div>
  );
}

export function AddHost({ onClose }: { onClose: () => void }) {
  const [code, setCode] = useState<EnrollCode | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api.createCode().then(setCode).catch((e) => setError(String(e)));
    const esc = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", esc);
    return () => window.removeEventListener("keydown", esc);
  }, [onClose]);

  const args = code?.args ?? "";
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
            <ol className="steps">
              <li>
                대상 컴퓨터에 <a href="https://learn.netdata.cloud/docs/netdata-agent/installation" target="_blank" rel="noreferrer">Netdata 에이전트</a>를 설치합니다. Netdata Cloud 연결(claim)은 필요 없습니다.
              </li>
              <li>같은 컴퓨터에서 NodeMon Connector를 아래 명령으로 한 번 실행해 등록합니다.</li>
            </ol>
            <Cmd label="Linux / macOS" text={`./nodemon-connector ${args}`} />
            <Cmd label="Windows (PowerShell)" text={`.\\nodemon-connector.exe ${args}`} />
            <p className="sub">등록 후에는 자격 증명이 저장되므로 인자 없이 실행하면 됩니다. 서비스로 등록하는 방법은 README를 참고하세요.</p>
          </>
        )}
      </div>
    </div>
  );
}
