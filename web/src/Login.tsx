// SPDX-License-Identifier: GPL-3.0-or-later
import { useState } from "react";
import { api } from "./api";
import { BACKEND } from "./env";

export function Login({ onLogin, startupError }: { onLogin: () => void; startupError?: string | null }) {
  const [error, setError] = useState<string | null>(null);
  const [email, setEmail] = useState("owner@example.com");

  const run = async (fn: () => Promise<void>) => {
    setError(null);
    try {
      await fn();
      onLogin();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  };

  return (
    <div className="login">
      <div className="card login-card">
        <h1>NodeMon</h1>
        <p className="sub">여러 컴퓨터의 자원 사용 현황을 한 곳에서 확인하세요.</p>
        {BACKEND === "firebase" && api.login ? (
          <button className="primary big" onClick={() => run(() => api.login!())}>Google 계정으로 로그인</button>
        ) : (
          <a className="primary big" href="/auth/google">Google 계정으로 로그인</a>
        )}
        {api.emulatorLogin && (
          <div className="emu">
            <label htmlFor="emu-email" className="sub">에뮬레이터 계정 (개발용)</label>
            <input id="emu-email" value={email} onChange={(e) => setEmail(e.target.value)} />
            <button className="link" onClick={() => run(() => api.emulatorLogin!(email))}>이 계정으로 로그인</button>
          </div>
        )}
        {(error ?? startupError) && <p className="err">{error ?? startupError}</p>}
        <p className="login-foot">NodeMon은 Netdata, Inc.와 무관한 독립 프로젝트입니다. Netdata는 Netdata, Inc.의 상표입니다.</p>
        <p className="login-foot">
          GPL-3.0-or-later · <a href="https://github.com/CHOICHI-HWAN/NodeMon" target="_blank" rel="noreferrer">소스 코드</a> ·{" "}
          <a href="/third-party-licenses.txt" target="_blank" rel="noreferrer">오픈소스 라이선스</a>
        </p>
        {import.meta.env.DEV && BACKEND === "http" && <a className="link" href="/auth/dev">개발용 로그인</a>}
      </div>
    </div>
  );
}
