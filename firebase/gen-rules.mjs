// Builds database.rules.json. Rules cannot read env vars, so the allowed Google
// accounts are baked in at generation time:  ALLOWED_EMAILS=me@gmail.com node firebase/gen-rules.mjs
import { writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const FIELDS = ["cpu", "mem_used", "mem_total", "disk_used", "disk_total", "net_rx", "net_tx", "load1"];
const num = { ".validate": "newData.isNumber() && newData.val() >= 0" };

export function buildRules(emails) {
  if (!emails.length) throw new Error("at least one allowed email is required");
  for (const e of emails) {
    // Emails are spliced into a rules expression, so reject anything that could break out of the string.
    if (!/^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$/.test(e)) throw new Error(`invalid email: ${e}`);
  }
  const emailList = emails.map((e) => `auth.token.email === '${e.toLowerCase()}'`).join(" || ");
  const OWNER = `auth != null && auth.token.firebase.sign_in_provider === 'google.com' && auth.token.email_verified === true && (${emailList})`;

  const devOwner = "root.child('devices/' + $dev + '/owner')";
  const readByOwner = `auth != null && ${devOwner}.val() === auth.uid`;
  // A device may write under its own id once it has been paired; its owner may only delete.
  const writeRule = `auth != null && ((auth.uid === $dev && ${devOwner}.exists()) || (${devOwner}.val() === auth.uid && !newData.exists()))`;

  const shape = (timeKey) => ({
    ".validate": `newData.hasChildren(['${timeKey}', ${FIELDS.map((f) => `'${f}'`).join(", ")}])`,
    [timeKey]: { ".validate": "newData.isNumber()" },
    ...Object.fromEntries(FIELDS.map((f) => [f, num])),
    $other: { ".validate": false },
  });

  // A device may list its own history (shallow) so it can prune old buckets itself.
  const readByOwnerOrSelf = `auth != null && (auth.uid === $dev || ${devOwner}.val() === auth.uid)`;
  const series = (bucketRe, timeRe) => ({
    $dev: {
      ".read": readByOwnerOrSelf,
      ".write": writeRule,
      $bucket: {
        ".validate": `$bucket.matches(/^[0-9]{${bucketRe}}$/)`,
        $t: { ".validate": `$t.matches(/^[0-9]{${timeRe}}$/)`, ...shape("t") },
      },
    },
  });

  return {
    rules: {
      ".read": false,
      ".write": false,

      // The dashboard owner creates a short-lived pairing code; a new device claims it exactly once.
      pairings: {
        $code: {
          ".read": "auth != null",
          ".write": `(${OWNER}) && (!data.exists() || data.child('owner').val() === auth.uid)`,
          ".validate": "newData.hasChildren(['owner', 'exp'])",
          owner: { ".validate": "newData.isString() && newData.val() === auth.uid" },
          exp: { ".validate": "newData.isNumber() && newData.val() > now && newData.val() <= now + 3600000" },
          claimedBy: {
            ".write": "auth != null && !data.exists() && root.child('pairings/' + $code + '/exp').val() > now",
            ".validate": "newData.isString() && newData.val() === auth.uid",
          },
          $other: { ".validate": false },
        },
      },

      devices: {
        $dev: {
          ".read": "auth != null && data.child('owner').val() === auth.uid",
          ".write":
            "auth != null && (" +
            "(!data.exists() && auth.uid === $dev" +
            " && root.child('pairings/' + newData.child('code').val() + '/claimedBy').val() === auth.uid" +
            " && root.child('pairings/' + newData.child('code').val() + '/owner').val() === newData.child('owner').val())" +
            " || (data.child('owner').val() === auth.uid && !newData.exists()))",
          ".validate": "newData.hasChildren(['owner', 'code', 'name', 'os'])",
          owner: { ".validate": "newData.isString()" },
          code: { ".validate": "newData.isString() && newData.val().matches(/^[A-Z0-9]{12}$/)" },
          name: { ".validate": "newData.isString() && newData.val().length > 0 && newData.val().length <= 100" },
          os: { ".validate": "newData.isString() && newData.val().length <= 50" },
          createdAt: { ".validate": "newData.isNumber()" },
          $other: { ".validate": false },
        },
      },

      // users/<owner>/devices/<device> is the index the dashboard lists from.
      users: {
        $uid: {
          devices: {
            ".read": "auth != null && auth.uid === $uid",
            $dev: {
              ".write": "auth != null && ((auth.uid === $dev && root.child('devices/' + $dev + '/owner').val() === $uid) || (auth.uid === $uid && !newData.exists()))",
              ".validate": "newData.hasChildren(['name', 'os'])",
              name: { ".validate": "newData.isString() && newData.val().length <= 100" },
              os: { ".validate": "newData.isString() && newData.val().length <= 50" },
              createdAt: { ".validate": "newData.isNumber()" },
              $other: { ".validate": false },
            },
          },
        },
      },

      latest: {
        $dev: {
          ".read": readByOwner,
          ".write": writeRule,
          ".validate": `newData.hasChildren(['ts', ${FIELDS.map((f) => `'${f}'`).join(", ")}])`,
          ts: { ".validate": "newData.isNumber()" },
          iv: { ".validate": "newData.isNumber() && newData.val() >= 1 && newData.val() <= 86400" },
          ...Object.fromEntries(FIELDS.map((f) => [f, num])),
          $other: { ".validate": false },
        },
      },
      // m: one-minute averages grouped by UTC day; h: one-hour averages grouped by UTC month.
      m: series(8, "9,10"),
      h: series(6, "9,10"),
    },
  };
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const emails = (process.argv[2] ?? process.env.ALLOWED_EMAILS ?? "").split(",").map((s) => s.trim()).filter(Boolean);
  const out = fileURLToPath(new URL("./database.rules.json", import.meta.url));
  writeFileSync(out, JSON.stringify(buildRules(emails), null, 2) + "\n");
  console.log(`wrote ${out} for ${emails.length} account(s)`);
}
