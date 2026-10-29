// Runs the security rules against the Firebase emulators:
//   ALLOWED_EMAILS=owner@example.com,other@example.com node firebase/gen-rules.mjs
//   firebase emulators:start --only auth,database --project demo-nodemon   (in another shell)
//   node firebase/rules.test.mjs
const AUTH = "http://127.0.0.1:9099/identitytoolkit.googleapis.com/v1";
const DB = "http://127.0.0.1:9000";
const NS = "demo-nodemon-default-rtdb";

async function anon() {
  const r = await (await fetch(`${AUTH}/accounts:signUp?key=x`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ returnSecureToken: true }) })).json();
  return { token: r.idToken, uid: r.localId };
}
async function google(email) {
  const idt = JSON.stringify({ sub: email, email, email_verified: true });
  const r = await (await fetch(`${AUTH}/accounts:signInWithIdp?key=x`, {
    method: "POST", headers: { "content-type": "application/json" },
    body: JSON.stringify({ postBody: `id_token=${encodeURIComponent(idt)}&providerId=google.com`, requestUri: "http://localhost", returnIdpCredential: true, returnSecureToken: true }),
  })).json();
  return { token: r.idToken, uid: r.localId };
}
async function db(who, method, path, body, query = "") {
  const auth = who ? `&auth=${who.token}` : "";
  const res = await fetch(`${DB}/${path}.json?ns=${NS}${auth}${query}`, { method, body: body === undefined ? undefined : JSON.stringify(body) });
  return res.status;
}

let failed = 0;
const ok = async (name, p) => { const s = await p; if (s !== 200) { failed++; console.log(`FAIL ${name}: expected 200, got ${s}`); } else console.log(`ok   ${name}`); };
const denied = async (name, p) => { const s = await p; if (s !== 401 && s !== 403) { failed++; console.log(`FAIL ${name}: expected denial, got ${s}`); } else console.log(`ok   ${name} (denied)`); };

const sample = { cpu: 10, mem_used: 1, mem_total: 2, disk_used: 1, disk_total: 2, net_rx: 0, net_tx: 0, load1: 0.5 };
const code = "ABCDEFGH2345";
const now = Date.now();

const owner = await google("owner@example.com");
const other = await google("other@example.com");
const stranger = await google("stranger@example.com"); // verified Google account, not on the allow-list
const dev = await anon();
const dev2 = await anon();
const hacker = await anon();

console.log("-- pairing");
await denied("anonymous cannot create a pairing", db(hacker, "PUT", `pairings/${code}`, { owner: hacker.uid, exp: now + 900000 }));
await denied("non-allow-listed Google account cannot create a pairing", db(stranger, "PUT", `pairings/${code}`, { owner: stranger.uid, exp: now + 900000 }));
await denied("pairing cannot name someone else as owner", db(owner, "PUT", `pairings/${code}`, { owner: other.uid, exp: now + 900000 }));
await denied("pairing cannot live longer than an hour", db(owner, "PUT", `pairings/${code}`, { owner: owner.uid, exp: now + 7200000 }));
await ok("owner creates a pairing", db(owner, "PUT", `pairings/${code}`, { owner: owner.uid, exp: now + 900000 }));
await denied("pairings cannot be listed", db(dev, "GET", "pairings"));
await ok("device can read a pairing by its code", db(dev, "GET", `pairings/${code}`));
await denied("claiming a nonexistent code fails", db(dev, "PUT", "pairings/ZZZZZZZZZZZZ/claimedBy", dev.uid));
await denied("cannot claim on behalf of another uid", db(dev, "PUT", `pairings/${code}/claimedBy`, hacker.uid));

console.log("-- enrolment");
await denied("device cannot register before claiming", db(dev, "PUT", `devices/${dev.uid}`, { owner: owner.uid, code, name: "pc", os: "linux" }));
await ok("device claims the code", db(dev, "PUT", `pairings/${code}/claimedBy`, dev.uid));
await denied("a code cannot be claimed twice", db(hacker, "PUT", `pairings/${code}/claimedBy`, hacker.uid));
await denied("claimer cannot register under a different owner", db(dev, "PUT", `devices/${dev.uid}`, { owner: hacker.uid, code, name: "pc", os: "linux" }));
await denied("claimer cannot register under another device id", db(dev, "PUT", `devices/${dev2.uid}`, { owner: owner.uid, code, name: "pc", os: "linux" }));
await denied("someone who did not claim cannot register", db(hacker, "PUT", `devices/${hacker.uid}`, { owner: owner.uid, code, name: "pc", os: "linux" }));
await ok("claimer registers under the pairing owner", db(dev, "PUT", `devices/${dev.uid}`, { owner: owner.uid, code, name: "pc", os: "linux" }));
await ok("device adds itself to the owner's index", db(dev, "PUT", `users/${owner.uid}/devices/${dev.uid}`, { name: "pc", os: "linux" }));
await denied("device cannot add itself to another user's index", db(dev, "PUT", `users/${other.uid}/devices/${dev.uid}`, { name: "pc", os: "linux" }));

console.log("-- writing samples");
await ok("device writes latest", db(dev, "PUT", `latest/${dev.uid}`, { ...sample, ts: now }));
await denied("negative values rejected", db(dev, "PUT", `latest/${dev.uid}`, { ...sample, cpu: -1, ts: now }));
await denied("unknown fields rejected", db(dev, "PUT", `latest/${dev.uid}`, { ...sample, ts: now, junk: 1 }));
await denied("non-numeric values rejected", db(dev, "PUT", `latest/${dev.uid}`, { ...sample, cpu: "x", ts: now }));
await ok("latest may carry the upload interval", db(dev, "PUT", `latest/${dev.uid}`, { ...sample, ts: now, iv: 180 }));
await denied("a non-numeric upload interval is rejected", db(dev, "PUT", `latest/${dev.uid}`, { ...sample, ts: now, iv: "x" }));
await ok("device writes a minute bucket", db(dev, "PUT", `m/${dev.uid}/20261029/1793280000`, { ...sample, t: 1793280000 }));
await ok("device writes an hour bucket", db(dev, "PUT", `h/${dev.uid}/202610/1793278800`, { ...sample, t: 1793278800 }));
await denied("malformed day key rejected", db(dev, "PUT", `m/${dev.uid}/today/1793280000`, { ...sample, t: 1793280000 }));
await denied("device cannot write another device's data", db(dev, "PUT", `latest/${dev2.uid}`, { ...sample, ts: now }));
await denied("unpaired device cannot write", db(dev2, "PUT", `latest/${dev2.uid}`, { ...sample, ts: now }));
await denied("owner cannot forge samples", db(owner, "PUT", `latest/${dev.uid}`, { ...sample, ts: now }));

console.log("-- reading");
await ok("owner reads latest", db(owner, "GET", `latest/${dev.uid}`));
await ok("owner reads a day with a key range", db(owner, "GET", `m/${dev.uid}/20261029`, undefined, '&orderBy="$key"&startAt="1793000000"'));
await ok("owner lists own devices", db(owner, "GET", `users/${owner.uid}/devices`));
await ok("owner reads own device record", db(owner, "GET", `devices/${dev.uid}`));
await ok("a device can list its own history", db(dev, "GET", `m/${dev.uid}`, undefined, "&shallow=true"));
await denied("a device cannot read another device's history", db(dev2, "GET", `m/${dev.uid}`));
await denied("other allowed user cannot read latest", db(other, "GET", `latest/${dev.uid}`));
await denied("other allowed user cannot read history", db(other, "GET", `m/${dev.uid}`));
await denied("other allowed user cannot read device record", db(other, "GET", `devices/${dev.uid}`));
await denied("other user cannot read the owner's index", db(other, "GET", `users/${owner.uid}/devices`));
await denied("a stranger cannot read latest", db(stranger, "GET", `latest/${dev.uid}`));
await denied("anonymous cannot read latest", db(hacker, "GET", `latest/${dev.uid}`));
await denied("unauthenticated cannot read latest", db(null, "GET", `latest/${dev.uid}`));
await denied("root cannot be read", db(owner, "GET", ""));

console.log("-- removal");
await denied("other user cannot delete the device", db(other, "DELETE", `devices/${dev.uid}`));
await ok("owner deletes history", db(owner, "DELETE", `m/${dev.uid}`));
await ok("owner deletes latest", db(owner, "DELETE", `latest/${dev.uid}`));
await ok("owner deletes index entry", db(owner, "DELETE", `users/${owner.uid}/devices/${dev.uid}`));
await ok("owner deletes the device", db(owner, "DELETE", `devices/${dev.uid}`));
await denied("a removed device can no longer write", db(dev, "PUT", `latest/${dev.uid}`, { ...sample, ts: now }));

console.log(failed ? `\n${failed} FAILED` : "\nall rule checks passed");
process.exit(failed ? 1 : 0);
