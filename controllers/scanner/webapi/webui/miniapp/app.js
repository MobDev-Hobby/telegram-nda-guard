// NDA Guard Mini App: pick a channel, scan it, tick who to remove, configure
// cleaning. Every value coming from the server is written with textContent,
// never parsed as HTML: names are chosen by channel members.
(() => {
  "use strict";

  const tg = window.Telegram && window.Telegram.WebApp;
  const API = "../api/miniapp/";
  const POLL_MS = 1000;

  const STRINGS = {
    ru: {
      openInTelegram: "Откройте приложение из Telegram: команда /app в чате с ботом.",
      authFailed: "Не удалось войти: ",
      channelsTitle: "Ваши каналы",
      channelsHint: "Каналы и группы, где вы администратор и которые защищает бот.",
      noChannels: "Пока нет каналов",
      noChannelsHint: "Отправьте боту /add и выберите канал или группу. Бот станет администратором с правом блокировать участников.",
      kindChannel: "Канал", kindGroup: "Группа",
      tabScan: "Сканирование", tabSettings: "Настройки",
      warnNotAdmin: "Бот не администратор. Добавьте его в администраторы, иначе сканировать нельзя.",
      warnCannotClean: "У бота нет права блокировать участников — удалять никого не получится.",
      scanIntro: "Бот получит список участников и проверит каждого. Потом вы отметите, кого удалить.",
      scanStart: "Запустить сканирование",
      scanListing: "Получаю список участников…",
      scanChecking: (c, n) => `Проверено ${c} из ${n}`,
      scanFailed: "Сканирование не удалось: ",
      partial: (f, t) => `Telegram отдал только ${f} из ${t} участников. Остальных бот не видит и не проверял.`,
      noClean: "Ручная чистка выключена. Включите «Разрешить ручную чистку» в настройках, чтобы удалять участников.",
      filterAll: "Все", filterBad: "Нет доступа", filterUnknown: "Не проверены", filterGood: "Есть доступ", filterKicked: "Удалены",
      search: "Поиск по имени или @username",
      selectBad: "Отметить всех без доступа", selectNone: "Снять отметки", rescan: "Сканировать заново",
      nothingFound: "Никого не нашлось",
      status: { bad: "нет доступа", unknown: "не проверен", good: "есть доступ", kicked: "удалён" },
      protectedAdmin: "администратор", protectedBot: "этот бот",
      kickButton: (n) => `Удалить выбранных (${n})`,
      kickConfirm: (n, title) => `Удалить ${n} участн. из «${title}»?`,
      kickDone: (ok, all) => `Удалено ${ok} из ${all}.`,
      kickFailedSome: "Не получилось:",
      groupAutomation: "Автоматизация", groupCleaning: "Как удалять",
      autoScan: "Автосканирование", autoScanHint: "Периодически проверять участников и присылать отчёт в управляющий чат.",
      autoClean: "Автоочистка", autoCleanHint: "Периодически удалять участников без доступа без подтверждения.",
      allowClean: "Разрешить ручную чистку", allowCleanHint: "Разрешить /clean и удаление из этого приложения.",
      keepBanned: "Оставлять в бане", keepBannedHint: "Удалённый не сможет вернуться по ссылке-приглашению.",
      cleanMessages: "Удалять сообщения", cleanMessagesHint: "Удалить сообщения удаляемого участника.",
      cleanUnknown: "Удалять непроверенных", cleanUnknownHint: "Автоочистка удаляет и тех, кого не удалось проверить.",
      defaultsNote: "Сейчас действуют настройки по умолчанию. После сохранения у канала будут свои.",
      save: "Сохранить", saved: "Сохранено",
      error: "Ошибка: ",
    },
    en: {
      openInTelegram: "Open the app from Telegram: send /app to the bot.",
      authFailed: "Sign-in failed: ",
      channelsTitle: "Your channels",
      channelsHint: "Channels and groups you administer that the bot protects.",
      noChannels: "No channels yet",
      noChannelsHint: "Send /add to the bot and pick a channel or group. The bot becomes an admin with the right to ban members.",
      kindChannel: "Channel", kindGroup: "Group",
      tabScan: "Scan", tabSettings: "Settings",
      warnNotAdmin: "The bot is not an administrator. Promote it, otherwise it can't scan.",
      warnCannotClean: "The bot can't ban members, so removing will fail.",
      scanIntro: "The bot fetches the member list and checks everyone. Then you tick who to remove.",
      scanStart: "Start scan",
      scanListing: "Fetching members…",
      scanChecking: (c, n) => `Checked ${c} of ${n}`,
      scanFailed: "Scan failed: ",
      partial: (f, t) => `Telegram returned only ${f} of ${t} members. The bot can't see or check the rest.`,
      noClean: "Manual cleaning is off. Turn on “Allow manual clean” in Settings to remove members.",
      filterAll: "All", filterBad: "No access", filterUnknown: "Unchecked", filterGood: "Has access", filterKicked: "Removed",
      search: "Search by name or @username",
      selectBad: "Tick everyone without access", selectNone: "Clear", rescan: "Scan again",
      nothingFound: "Nobody found",
      status: { bad: "no access", unknown: "unchecked", good: "has access", kicked: "removed" },
      protectedAdmin: "administrator", protectedBot: "this bot",
      kickButton: (n) => `Remove selected (${n})`,
      kickConfirm: (n, title) => `Remove ${n} member(s) from “${title}”?`,
      kickDone: (ok, all) => `Removed ${ok} of ${all}.`,
      kickFailedSome: "Failed:",
      groupAutomation: "Automation", groupCleaning: "How to remove",
      autoScan: "Auto scan", autoScanHint: "Check members periodically and report to the control chat.",
      autoClean: "Auto clean", autoCleanHint: "Periodically remove members without access, without asking.",
      allowClean: "Allow manual clean", allowCleanHint: "Allow /clean and removing from this app.",
      keepBanned: "Keep banned", keepBannedHint: "Removed members can't come back via an invite link.",
      cleanMessages: "Delete messages", cleanMessagesHint: "Delete the removed member's messages.",
      cleanUnknown: "Remove unchecked", cleanUnknownHint: "Auto clean also removes members whose check failed.",
      defaultsNote: "The defaults apply now. Saving gives the channel its own settings.",
      save: "Save", saved: "Saved",
      error: "Error: ",
    },
  };
  const lang = ((tg && tg.initDataUnsafe && tg.initDataUnsafe.user && tg.initDataUnsafe.user.language_code) || navigator.language || "en").slice(0, 2);
  const T = STRINGS[lang] || STRINGS.en;
  document.documentElement.lang = STRINGS[lang] ? lang : "en";

  const app = document.getElementById("app");
  let token = null;
  let mainButtonHandler = null;
  let backHandler = null;
  let pollTimer = null;

  // ---------- Telegram chrome ----------

  function setMainButton(text, handler, { destructive = false } = {}) {
    if (!tg) return;
    const mb = tg.MainButton;
    if (mainButtonHandler) mb.offClick(mainButtonHandler);
    mainButtonHandler = null;
    if (!text) {
      mb.hide();
      return;
    }
    const params = { text, is_visible: true, is_active: true };
    if (destructive && tg.themeParams.destructive_text_color) {
      params.color = tg.themeParams.destructive_text_color;
      params.text_color = "#ffffff";
    } else {
      params.color = tg.themeParams.button_color;
      params.text_color = tg.themeParams.button_text_color;
    }
    mb.setParams(params);
    mainButtonHandler = handler;
    mb.onClick(handler);
  }

  function setBack(handler) {
    if (!tg) return;
    if (backHandler) tg.BackButton.offClick(backHandler);
    backHandler = handler;
    if (handler) {
      tg.BackButton.onClick(handler);
      tg.BackButton.show();
    } else {
      tg.BackButton.hide();
    }
  }

  function alertMsg(text) {
    return new Promise((resolve) => (tg ? tg.showAlert(text, resolve) : (window.alert(text), resolve())));
  }

  function confirmMsg(text) {
    return new Promise((resolve) => (tg ? tg.showConfirm(text, resolve) : resolve(window.confirm(text))));
  }

  function haptic(kind) {
    if (tg && tg.HapticFeedback) tg.HapticFeedback.notificationOccurred(kind);
  }

  // ---------- helpers ----------

  async function api(path, { method = "GET", body } = {}) {
    const res = await fetch(API + path, {
      method,
      headers: {
        "Content-Type": "application/json",
        ...(token ? { Authorization: "Bearer " + token } : {}),
      },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || res.statusText);
    return data;
  }

  function fromTemplate(id) {
    const node = document.getElementById(id).content.cloneNode(true);
    node.querySelectorAll("[data-i18n]").forEach((el) => {
      el.textContent = T[el.dataset.i18n];
    });
    return node;
  }

  const $ = (root, role) => root.querySelector(`[data-role="${role}"]`);

  function el(tag, props = {}, children = []) {
    const node = document.createElement(tag);
    Object.assign(node, props);
    for (const child of children) node.append(child);
    return node;
  }

  function isChannel(ch) {
    return ch.chatType === "channel";
  }

  function stopPolling() {
    clearTimeout(pollTimer);
    pollTimer = null;
  }

  function showError(err) {
    app.replaceChildren(el("p", { className: "notice warn", textContent: T.error + err.message }));
  }

  // ---------- screens ----------

  async function showChannels() {
    stopPolling();
    setBack(null);
    setMainButton(null);
    const view = fromTemplate("tpl-channels");
    const list = $(view, "list");
    const empty = $(view, "empty");
    app.replaceChildren(view);

    const channels = await api("channels");
    channels.sort((a, b) => a.title.localeCompare(b.title));
    empty.hidden = channels.length > 0;
    for (const ch of channels) {
      const sub = [isChannel(ch) ? T.kindChannel : T.kindGroup];
      if (!ch.botOnChannel) sub.push(T.warnNotAdmin);
      const button = el("button", { className: "item", type: "button" }, [
        el("span", { className: "item-title", textContent: ch.title }),
        el("span", { className: "item-sub", textContent: sub.join(" · ") }),
      ]);
      button.addEventListener("click", () => showChannel(ch.id).catch(showError));
      list.append(el("li", {}, [button]));
    }
  }

  async function showChannel(channelId, tab = "scan") {
    stopPolling();
    setMainButton(null);
    setBack(() => showChannels().catch(showError));

    const channel = await api(`channels/${channelId}`);
    const view = fromTemplate("tpl-channel");
    $(view, "title").textContent = channel.title;
    $(view, "kind").textContent = isChannel(channel) ? T.kindChannel : T.kindGroup;

    const warnings = $(view, "warnings");
    if (!channel.botOnChannel) warnings.append(el("li", { textContent: T.warnNotAdmin }));
    else if (!channel.botCanClean) warnings.append(el("li", { textContent: T.warnCannotClean }));

    const root = view.querySelector(".screen");
    const tabs = [...view.querySelectorAll("[role=tab]")];
    const selectTab = (name) => {
      stopPolling();
      setMainButton(null);
      for (const t of tabs) {
        const on = t.dataset.tab === name;
        t.setAttribute("aria-selected", String(on));
        t.tabIndex = on ? 0 : -1;
      }
      for (const p of root.querySelectorAll("[role=tabpanel]")) p.hidden = p.dataset.panel !== name;
      const panel = root.querySelector(`[data-panel="${name}"]`);
      if (name === "scan") renderScanIdle(panel, channel);
      else renderSettings(panel, channel);
    };
    tabs.forEach((t, i) => {
      t.addEventListener("click", () => selectTab(t.dataset.tab));
      t.addEventListener("keydown", (e) => {
        if (e.key !== "ArrowLeft" && e.key !== "ArrowRight") return;
        const next = tabs[(i + (e.key === "ArrowRight" ? 1 : tabs.length - 1)) % tabs.length];
        next.focus();
        selectTab(next.dataset.tab);
      });
    });
    app.replaceChildren(view);
    selectTab(tab);
  }

  // ---------- scan ----------

  function renderScanIdle(panel, channel) {
    const view = fromTemplate("tpl-scan-idle");
    const start = $(view, "start");
    start.disabled = !channel.botOnChannel;
    start.addEventListener("click", () => startScan(panel, channel));
    panel.replaceChildren(view);
  }

  async function startScan(panel, channel) {
    try {
      const scan = await api(`channels/${channel.id}/scans`, { method: "POST" });
      pollScan(panel, channel, scan.id);
    } catch (err) {
      panel.replaceChildren(el("p", { className: "notice warn", textContent: T.scanFailed + err.message }));
    }
  }

  function renderScanRunning(panel, scan) {
    let view = panel.querySelector("#scan-progress");
    if (!view) {
      panel.replaceChildren(fromTemplate("tpl-scan-running"));
      view = panel.querySelector("#scan-progress");
    }
    const label = panel.querySelector('[data-role="label"]');
    const total = scan.stats && scan.stats.fetched;
    if (total) {
      view.max = total;
      view.value = scan.checked;
      label.textContent = T.scanChecking(scan.checked, total);
    } else {
      view.removeAttribute("value");
      label.textContent = T.scanListing;
    }
  }

  async function pollScan(panel, channel, scanId) {
    stopPolling();
    try {
      const scan = await api(`channels/${channel.id}/scans/${scanId}`);
      if (scan.state === "running") {
        renderScanRunning(panel, scan);
        pollTimer = setTimeout(() => pollScan(panel, channel, scanId), POLL_MS);
        return;
      }
      if (scan.state === "failed") {
        panel.replaceChildren(el("p", { className: "notice warn", textContent: T.scanFailed + scan.error }));
        haptic("error");
        return;
      }
      haptic("success");
      renderScanResult(panel, channel, scan);
    } catch (err) {
      panel.replaceChildren(el("p", { className: "notice warn", textContent: T.scanFailed + err.message }));
    }
  }

  function renderScanResult(panel, channel, scan) {
    const view = fromTemplate("tpl-scan-result");
    const users = scan.users || [];
    const selected = new Set();
    const canKick = channel.allowClean && channel.botCanClean;
    let filter = users.some((u) => u.status === "bad") ? "bad" : "all";
    let query = "";

    if (scan.partial) {
      const p = $(view, "partial");
      p.hidden = false;
      p.textContent = T.partial(scan.stats.fetched, scan.stats.total);
    }
    if (!channel.allowClean) {
      const p = $(view, "noclean");
      p.hidden = false;
      p.textContent = T.noClean;
    }

    const counts = { all: users.length, bad: 0, unknown: 0, good: 0, kicked: 0 };
    for (const u of users) counts[u.status] = (counts[u.status] || 0) + 1;

    const filters = $(view, "filters");
    const filterDefs = [
      ["all", T.filterAll], ["bad", T.filterBad], ["unknown", T.filterUnknown],
      ["good", T.filterGood], ["kicked", T.filterKicked],
    ];
    for (const [key, text] of filterDefs) {
      if (key === "kicked" && !counts.kicked) continue;
      const input = el("input", { type: "radio", name: "filter", value: key, checked: key === filter });
      input.addEventListener("change", () => {
        filter = key;
        renderList();
      });
      filters.append(el("label", {}, [input, el("span", { textContent: `${text} · ${counts[key] || 0}` })]));
    }

    const search = $(view, "search");
    search.placeholder = T.search;
    search.setAttribute("aria-label", T.search);
    search.addEventListener("input", () => {
      query = search.value.trim().toLowerCase().replace(/^@/, "");
      renderList();
    });

    const selectable = (u) => canKick && !u.protected && u.status !== "kicked";
    $(view, "select-bad").hidden = !canKick;
    $(view, "select-none").hidden = !canKick;
    $(view, "select-bad").addEventListener("click", () => {
      users.filter((u) => u.status === "bad" && selectable(u)).forEach((u) => selected.add(u.id));
      renderList();
    });
    $(view, "select-none").addEventListener("click", () => {
      selected.clear();
      renderList();
    });
    $(view, "rescan").addEventListener("click", () => startScan(panel, channel));

    const list = $(view, "users");
    const nothing = $(view, "nothing");

    function userName(u) {
      return [u.firstName, u.lastName].filter(Boolean).join(" ") || String(u.id);
    }

    function renderList() {
      const visible = users.filter((u) =>
        (filter === "all" || u.status === filter) &&
        (!query || userName(u).toLowerCase().includes(query) || (u.username || "").toLowerCase().includes(query)));
      list.replaceChildren(...visible.map((u) => {
        const box = el("input", { type: "checkbox", checked: selected.has(u.id), disabled: !selectable(u) });
        box.setAttribute("aria-label", userName(u));
        box.addEventListener("change", () => {
          if (box.checked) selected.add(u.id);
          else selected.delete(u.id);
          updateMainButton();
        });
        const sub = [];
        if (u.username) sub.push("@" + u.username);
        if (u.protected) sub.push(u.note === "administrator" ? T.protectedAdmin : T.protectedBot);
        const label = el("label", {}, [
          box,
          el("span", { className: "user-name", textContent: userName(u) }, [
            el("span", { className: "user-sub", textContent: sub.join(" · ") }),
          ]),
          el("span", { className: `badge ${u.status}`, textContent: T.status[u.status] || u.status }),
        ]);
        return el("li", { className: selectable(u) ? "" : "disabled" }, [label]);
      }));
      nothing.hidden = visible.length > 0;
      updateMainButton();
    }

    function updateMainButton() {
      if (!canKick || selected.size === 0) {
        setMainButton(null);
        return;
      }
      setMainButton(T.kickButton(selected.size), kick, { destructive: true });
    }

    async function kick() {
      const ids = [...selected];
      if (!(await confirmMsg(T.kickConfirm(ids.length, channel.title)))) return;
      if (tg) tg.MainButton.showProgress(false);
      try {
        const { results } = await api(`channels/${channel.id}/kick`, {
          method: "POST",
          body: { scanId: scan.id, userIds: ids },
        });
        const ok = results.filter((r) => r.ok).length;
        const failed = results.filter((r) => !r.ok);
        const byId = new Map(users.map((u) => [u.id, u]));
        let text = T.kickDone(ok, ids.length);
        if (failed.length) {
          text += "\n\n" + T.kickFailedSome + "\n" + failed.slice(0, 10)
            .map((r) => `• ${byId.has(r.userId) ? userName(byId.get(r.userId)) : r.userId}: ${r.error}`).join("\n");
        }
        haptic(failed.length ? "warning" : "success");
        await alertMsg(text);
      } catch (err) {
        haptic("error");
        await alertMsg(T.error + err.message);
      } finally {
        if (tg) tg.MainButton.hideProgress();
      }
      // Reload the scan: kicked users are now marked as such.
      pollScan(panel, channel, scan.id);
    }

    panel.replaceChildren(view);
    renderList();
  }

  // ---------- settings ----------

  const SETTINGS = [
    ["automation", "autoScan"], ["automation", "autoClean"], ["automation", "allowClean"],
    ["cleaning", "keepBanned"], ["cleaning", "cleanMessages"], ["cleaning", "cleanUnknown"],
  ];

  function renderSettings(panel, channel) {
    const view = fromTemplate("tpl-settings");
    const initial = Object.fromEntries(SETTINGS.map(([, key]) => [key, Boolean(channel[key])]));
    const current = { ...initial };

    for (const [group, key] of SETTINGS) {
      const input = el("input", { type: "checkbox", checked: current[key], name: key });
      input.setAttribute("role", "switch");
      input.addEventListener("change", () => {
        current[key] = input.checked;
        updateSave();
      });
      $(view, group).append(el("label", { className: "switch" }, [
        el("span", { textContent: T[key] }),
        input,
        el("small", { textContent: T[key + "Hint"] }),
      ]));
    }
    $(view, "defaults").textContent = channel.customCleanOptions ? "" : T.defaultsNote;

    const dirty = () => SETTINGS.some(([, key]) => current[key] !== initial[key]) || !channel.customCleanOptions;

    function updateSave() {
      if (dirty()) setMainButton(T.save, save);
      else setMainButton(null);
    }

    async function save() {
      if (tg) tg.MainButton.showProgress(false);
      try {
        const updated = await api(`channels/${channel.id}/settings`, { method: "PUT", body: current });
        Object.assign(channel, updated);
        haptic("success");
        renderSettings(panel, channel);
      } catch (err) {
        haptic("error");
        await alertMsg(T.error + err.message);
      } finally {
        if (tg) tg.MainButton.hideProgress();
      }
    }

    panel.replaceChildren(view);
    updateSave();
  }

  // ---------- boot ----------

  async function boot() {
    if (!tg || !tg.initData) {
      app.replaceChildren(el("p", { className: "notice", textContent: T.openInTelegram }));
      return;
    }
    document.documentElement.dataset.tg = "1";
    // Native controls follow color-scheme; tie it to Telegram's theme, not the
    // OS, or a light Telegram theme on a dark OS gets dark checkboxes.
    const syncScheme = () => { document.documentElement.style.colorScheme = tg.colorScheme; };
    syncScheme();
    tg.onEvent("themeChanged", syncScheme);
    tg.ready();
    tg.expand();
    try {
      const auth = await api("auth", { method: "POST", body: { initData: tg.initData } });
      token = auth.token;
    } catch (err) {
      app.replaceChildren(el("p", { className: "notice warn", textContent: T.authFailed + err.message }));
      return;
    }
    // t.me/<bot>/<app>?startapp=<channel id> opens a channel directly.
    const start = tg.initDataUnsafe && tg.initDataUnsafe.start_param;
    if (start && /^-?\d+$/.test(start)) {
      await showChannel(start).catch(() => showChannels());
      return;
    }
    await showChannels();
  }

  boot().catch(showError);
})();
