export function Login() {
  return (
    <div className="login">
      <div className="card login-card">
        <h1>NodeMon</h1>
        <p className="sub">여러 컴퓨터의 자원 사용 현황을 한 곳에서 확인하세요.</p>
        <a className="primary big" href="/auth/google">Google 계정으로 로그인</a>
        {import.meta.env.DEV && <a className="link" href="/auth/dev">개발용 로그인</a>}
      </div>
    </div>
  );
}
