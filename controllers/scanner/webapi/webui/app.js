// Telegram NDA Guard — minimal SPA dashboard. Vanilla JS, no build step.
// Talks to the REST API exposed by webapi.Server. Auth is via a session cookie
// set by the Telegram Login Widget callback (/api/auth/login).

const api = (p, opts = {}) => fetch(p, { credentials: "same-origin", headers: { "Content-Type": "application/json" }, ...opts }).then(async r => {
  if (r.status === 401) { location.hash = "#login"; throw new Error("unauthorized"); }
  if (!r.ok) { const e = await r.json().catch(() => ({})); throw new Error(e.error || ("HTTP " + r.status)); }
  return r.status === 204 ? null : r.json();
});

function toast(msg) {
  const t = document.getElementById("toast");
  t.textContent = msg; t.classList.add("show");
  setTimeout(() => t.classList.remove("show"), 2500);
}

// --- Telegram Login Widget ---
// Rendered into #login-widget. The widget redirects back with query params that
// /api/auth/login verifies server-side. On success the cookie is set and we
// reload to the dashboard.
async function ensureAuth() {
  try { await api("/api/auth/me"); return true; } catch { return false; }
}

function tgLoginUrl(botUsername) {
  const u = new URL("https://oauth.telegram.org/embed/" + botUsername);
  u.searchParams.set("origin", location.origin + location.pathname);
  u.searchParams.set("embed", "1");
  return u.toString();
}

async function renderLogin() {
  // We don't know the bot username client-side; the login page uses Telegram's
  // redirect widget which needs the bot username. As a fallback, instruct the
  // operator to use the direct callback URL. In a wired deployment the page is
  // served under the bot's configured domain and the widget renders.
  document.getElementById("app").innerHTML = `
    <div class="login">
      <h2>Sign in with Telegram</h2>
      <p class="center">Use the Telegram Login Widget configured for this bot.<br>
      After authorizing, you'll be redirected back automatically.</p>
      <a class="ghost" style="padding:7px 14px" href="${location.origin}/api/auth/login?__demo=1">Retry</a>
    </div>`;
  // The real flow: Telegram redirects to ?id=...&hash=... which /api/auth/login
  // consumes. If such params are present, call login.
  if (new URLSearchParams(location.search).get("hash")) {
    try {
      await api("/api/auth/login" + location.search);
      location.search = "";
      return;
    } catch (e) { toast("Login failed: " + e.message); }
  }
}

// --- Dashboard ---
function flag(label, on) {
  return `<span class="flag ${on ? "on" : "off"}">${on ? "✓" : "○"} ${label}</span>`;
}
function right(label, ok) {
  return `<span class="${ok ? "ok" : "no"}">${label}: ${ok ? "✓" : "✗"}</span>`;
}

async function loadAndRender() {
  if (!(await ensureAuth())) { await renderLogin(); return; }
  const status = await api("/api/status");
  const who = await api("/api/auth/me");
  document.getElementById("app").innerHTML = `
    <header>
      <h1>🛡️ NDA Guard</h1>
      <span class="who">user #${who.userId} · bot @${status.botUsername} · userbot @${status.userBotUsername}</span>
      <button class="ghost" onclick="refreshRights()">↻ Refresh rights</button>
    </header>
    <div class="wrap">
      <div class="card">
        <h2>Channels (${status.channels.length})</h2>
        <div id="channels"></div>
      </div>
    </div>`;

  const box = document.getElementById("channels");
  if (status.channels.length === 0) {
    box.innerHTML = '<p class="center">No channels yet. Add one via the bot /add command.</p>';
    return;
  }
  for (const c of status.channels) {
    box.insertAdjacentHTML("beforeend", channelCard(c));
  }
}

function channelCard(c) {
  return `
    <div class="card" id="ch-${c.id}">
      <h2>${escapeHtml(c.title || "#" + c.id)} <span style="color:var(--muted);font-weight:400">#${c.id}</span></h2>
      <div class="row" style="margin-bottom:10px">
        ${flag("Auto Scan", c.autoScan)}
        ${flag("Auto Clean", c.autoClean)}
        ${flag("Allow Clean", c.allowClean)}
      </div>
      <div class="right" style="margin-bottom:12px">
        ${right("member", c.botOnChannel)} ${right("invite", c.botCanInvite)} ${right("clean", c.botCanClean)}
      </div>
      <div class="row">
        <button class="ghost" onclick="toggleFlag(${c.id}, 'autoScan', ${!c.autoScan})">${c.autoScan ? "Disable" : "Enable"} scan</button>
        <button class="ghost" onclick="toggleFlag(${c.id}, 'autoClean', ${!c.autoClean})">${c.autoClean ? "Disable" : "Enable"} clean</button>
        <button class="ghost" onclick="toggleFlag(${c.id}, 'allowClean', ${!c.allowClean})">${c.allowClean ? "Disable" : "Enable"} allow-clean</button>
        <button onclick="trigger(${c.id}, 'scan')">Run scan</button>
        <button onclick="trigger(${c.id}, 'clean')">Run clean</button>
        <button class="ghost" onclick="showUsers(${c.id})">Users</button>
        <button class="ghost" onclick="removeCh(${c.id})" style="border-color:var(--red);color:var(--red)">Remove</button>
      </div>
      <div id="users-${c.id}"></div>
    </div>`;
}

async function toggleFlag(id, flag, val) {
  const c = await api(`/api/channels/${id}`);
  const body = { autoScan: c.autoScan, autoClean: c.autoClean, allowClean: c.allowClean };
  body[flag] = val;
  await api(`/api/channels/${id}/flags`, { method: "PATCH", body: JSON.stringify(body) });
  toast(`${flag} ${val ? "enabled" : "disabled"}`);
  await loadAndRender();
}

async function trigger(id, kind) {
  const chat = prompt("Control chat ID for the report:", "");
  if (!chat) return;
  await api(`/api/channels/${id}/${kind}?chat=${encodeURIComponent(chat)}`, { method: "POST" });
  toast(`${kind} triggered (check the control chat for the report)`);
}

async function showUsers(id) {
  const box = document.getElementById(`users-${id}`);
  box.innerHTML = '<p class="center">Loading…</p>';
  const u = await api(`/api/channels/${id}/users`);
  const row = (label, users, cls) => users.length
    ? `<tr><th>${label}</th><td><span class="pill ${cls}">${users.length}</span></td><td>${users.map(x => escapeHtml(x.firstName || ("#" + x.id))).join(", ")}</td></tr>` : "";
  box.innerHTML = `<table>
    ${row("Good", u.good, "g")}${row("Unknown", u.unknown, "u")}${row("Bad", u.bad, "b")}
  </table>`;
}

async function removeCh(id) {
  if (!confirm(`Remove channel #${id}?`)) return;
  const chat = prompt("Control chat ID to remove from:", "");
  if (!chat) return;
  await api(`/api/channels/${id}?chat=${encodeURIComponent(chat)}`, { method: "DELETE" });
  toast("Channel removed");
  await loadAndRender();
}

async function refreshRights() {
  await api("/api/refresh-rights", { method: "POST" });
  toast("Rights refreshed");
  await loadAndRender();
}

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, m => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[m]));
}

window.toggleFlag = toggleFlag;
window.trigger = trigger;
window.showUsers = showUsers;
window.removeCh = removeCh;
window.refreshRights = refreshRights;

loadAndRender().catch(e => { document.getElementById("app").innerHTML = `<div class="center">Error: ${escapeHtml(e.message)}</div>`; });
