// NDA Guard Mini App. Every value coming from the server is written with
// textContent, never parsed as HTML: names and notes are user-controlled.
(() => {
  "use strict";

  const tg = window.Telegram && window.Telegram.WebApp;
  const API = "../api/miniapp/";
  const POLL_MS = 1000;
  const AUDIT_PAGE = 100;

  // ---------- strings ----------

  const STRINGS = {
    ru: {
      openInTelegram: "Откройте приложение из Telegram: команда /app в чате с ботом.",
      notEmployee: "Приложение доступно только сотрудникам. Если вы сотрудник, проверьте, что ваш Telegram указан в профиле.",
      authFailed: "Не удалось войти: ",
      error: "Ошибка: ",
      channelsTitle: "Ваши каналы",
      channelsHint: "Защищённые каналы и группы, где вы администратор.",
      noChannels: "Пока нет каналов",
      noChannelsHint: "Подключите канал ниже или добавьте его через бота.",
      kindChannel: "Канал", kindGroup: "Группа",
      join: "Присоединиться",
      joinHint: "Вы админ, но ещё не управляете каналом в боте",
      joined: (t) => `Вы присоединились к «${t}»`,
      availableTitle: "Доступны для подключения",
      availableHint: "Бот уже админ в этих чатах, но защита не включена. Отчёты будут приходить вам в личку.",
      connect: "Подключить",
      connectConfirm: (t) => `Включить защиту «${t}»? Отчёты будут приходить вам в личку с ботом.`,
      addViaBot: "Добавить канал или группу через бота",
      health: {
        ok: (d) => `всё в порядке · проверен ${d}`,
        never_checked: () => "ещё не проверялся",
        stale: (d) => `не проверялся больше 3 дней · ${d}`,
        very_stale: (d) => `не проверялся больше недели · ${d}`,
        violations: (d, n) => `нарушения: ${n} · ${d}`,
      },
      tabScan: "Участники", tabJoins: "Заявки", tabWhitelist: "Белый список", tabSettings: "Настройки",
      warnNotAdmin: "Бот не администратор. Добавьте его в администраторы, иначе сканировать нельзя.",
      warnCannotClean: "У бота нет права блокировать участников — удалять никого не получится.",
      warnCannotInvite: "У бота нет права приглашать — одобрять заявки не получится.",
      scanIntro: "Бот получит список участников и проверит каждого. Потом вы отметите, кого удалить или внести в белый список.",
      scanStart: "Запустить сканирование",
      scanListing: "Получаю список участников…",
      scanChecking: (c, n) => `Проверено ${c} из ${n}`,
      scanFailed: "Сканирование не удалось: ",
      partial: (f, t) => `Telegram отдал только ${f} из ${t} участников. Остальных бот не видит и не проверял.`,
      noClean: "Ручная чистка выключена. Включите «Разрешить ручную чистку» в настройках, чтобы удалять участников.",
      legend: { good: "проходит проверку", whitelisted: "белый список", unknown: "не удалось проверить", bad: "не проходит", kicked: "удалён" },
      filterAll: "Все", filterBad: "Не проходят", filterUnknown: "Не проверены", filterWhitelisted: "Белый список", filterGood: "Проходят", filterKicked: "Удалены",
      search: "Поиск по имени или @username",
      selectBad: "Отметить всех, кто не проходит", selectNone: "Снять отметки", rescan: "Сканировать заново",
      nothingFound: "Никого не нашлось",
      recheck: "Перепроверить",
      checkResult: { good: "чекер: проходит", bad: "чекер: не проходит", unknown: "чекер: не удалось проверить" },
      protectedAdmin: "администратор", protectedBot: "этот бот",
      wlUntil: (d) => `в белом списке, проверка ${d}`,
      wlOverdue: "в белом списке · нужна повторная проверка",
      wlDeleteAt: (d) => `до ${d}`,
      wlTempUntil: (d) => `в белом списке до ${d}, потом удалится`,
      wlTemporaryLeft: (d, left) => `временно, удалится ${d} · через ${left} дн.`,
      kickButton: (n) => `Удалить выбранных (${n})`,
      kickConfirm: (n, t) => `Удалить ${n} участн. из «${t}»?`,
      kickDone: (ok, all) => `Удалено ${ok} из ${all}.`,
      failedSome: "Не получилось:",
      wlAddButton: (n) => `В белый список (${n})`,
      wlFormTitle: (n) => `Добавить ${n} в белый список`,
      wlNote: "Заметка: почему им можно (необязательно)",
      wlTerm: "Срок",
      wlTermReview: (days) => `Бессрочно, повторная проверка каждые ${days} дн.`,
      wlTermDays: (n) => `${n} дн., потом удалить`,
      wlTermCustom: "Своё число дней",
      wlDays: "Дней",
      wlSubmit: "Добавить", cancel: "Отмена",
      wlAdded: (n) => `Добавлено: ${n}.`,
      whitelistIntro: (days) => `Люди из белого списка не проверяются и не удаляются. Раз в ${days} дн. их нужно проверить заново: до этого бот напомнит за 3 дня, а после — будет напоминать каждый день. Просроченные записи продолжают защищать. Временные записи удаляются в конце срока.`,
      whitelistEmpty: "Белый список пуст",
      whitelistEmptyHint: "Запустите сканирование, отметьте людей и нажмите «В белый список».",
      wlActive: (d, left) => `проверка ${d} · через ${left} дн.`,
      wlExpired: (d) => `проверка просрочена с ${d}`,
      wlTemporary: (d) => `удалится ${d}`,
      approvedBy: (who) => `одобрил ${who}`,
      overdue: "нужна проверка",
      renew: "Продлить", remove: "Убрать",
      renewTitle: (name) => `Продлить для ${name}`,
      renewNote: "Новая заметка (необязательно)",
      renewTermKeep: "Срок не менять",
      removeConfirm: (name) => `Убрать ${name} из белого списка? Со следующего сканирования его снова будут проверять.`,
      joinsIntroAuto: "Автоприём: бот сам одобряет тех, кто проходит проверку. Остальные ждут здесь и перепроверяются раз в сутки.",
      joinsIntroManual: "Ручной приём: одобрите или отклоните заявки. Цвет — результат проверки.",
      joinsEmpty: "Нет ожидающих заявок",
      joinsEmptyHint: "Бот видит заявки, поданные после его подключения.",
      requested: (d) => `подана ${d}`,
      approveButton: (n) => `Одобрить (${n})`,
      declineButton: (n) => `Отклонить (${n})`,
      approveConfirm: (n) => `Одобрить ${n} заявк.?`,
      declineConfirm: (n) => `Отклонить ${n} заявк.?`,
      resolved: (ok, all) => `Готово: ${ok} из ${all}.`,
      selectPassing: "Отметить прошедших проверку",
      groupAutomation: "Автоматизация", groupCleaning: "Как удалять", groupJoins: "Заявки на вступление", groupAudit: "Журнал действий",
      autoScan: "Автосканирование", autoScanHint: "Периодически проверять участников и присылать отчёт в управляющий чат.",
      autoClean: "Автоочистка", autoCleanHint: "Периодически удалять тех, кто не проходит проверку, без подтверждения.",
      allowClean: "Разрешить ручную чистку", allowCleanHint: "Разрешить /clean и удаление из этого приложения.",
      keepBanned: "Оставлять в бане", keepBannedHint: "Удалённый не сможет вернуться по ссылке-приглашению.",
      cleanMessages: "Удалять сообщения", cleanMessagesHint: "Удалить сообщения удаляемого участника.",
      cleanUnknown: "Удалять непроверенных", cleanUnknownHint: "Автоочистка удаляет и тех, кого не удалось проверить.",
      joinModes: { off: "Выкл", auto: "Авто", manual: "Вручную" },
      joinModeHint: {
        off: "Заявки разбирают админы в Telegram, бот их не трогает.",
        auto: "Бот одобряет прошедших проверку, остальных перепроверяет раз в сутки.",
        manual: "Заявки ждут решения менеджера в этом приложении.",
      },
      joinModeNeedsApproval: "Режимы работают, если в настройках чата включено одобрение заявок.",
      defaultsNote: "Сейчас действуют настройки по умолчанию. После сохранения у канала будут свои.",
      save: "Сохранить",
      auditEmpty: "Пока пусто",
      bot: "бот",
      source: { schedule: "по расписанию", manual: "вручную", miniapp: "в приложении" },
      audit: {
        "channel.added": (e) => `подключил канал`,
        "channel.joined": () => "присоединился к управлению",
        "settings.changed": () => "изменил настройки",
        "scan.completed": (e) => `скан ${src(e)}: ${counts(e)}`,
        "clean.completed": (e) => `чистка ${src(e)}: ${counts(e)}`,
        "users.kicked": (e) => `удалил ${e.counts ? e.counts.kicked : 0} из ${e.counts ? e.counts.selected : 0}: ${names(e)}`,
        "user.rechecked": (e) => `перепроверил ${names(e)} → ${check(e.details && e.details.result)}`,
        "whitelist.added": (e) => `внёс в белый список ${names(e)}${note(e)}`,
        "whitelist.renewed": (e) => `продлил белый список для ${names(e)}${note(e)}`,
        "whitelist.removed": (e) => e.details && e.details.reason === "term_ended" ? `срок истёк, убраны из белого списка: ${names(e)}` : `убрал из белого списка ${names(e)}`,
        "whitelist.expired": (e) => `просрочена проверка белого списка: ${names(e)}`,
        "join.requested": (e) => `заявка от ${names(e)} → ${check(e.details && e.details.check)}`,
        "join.approved": (e) => `${e.details && e.details.auto ? "автоматически одобрил" : "одобрил"} заявку ${names(e)}`,
        "join.declined": (e) => `отклонил заявку ${names(e)}`,
        "join.rechecked": (e) => `перепроверил заявку ${names(e)} → ${check(e.details && e.details.check)}`,
      },
      countWords: { good: "проходят", bad: "не проходят", unknown: "не проверены", whitelisted: "белый список" },
    },
    en: {
      openInTelegram: "Open the app from Telegram: send /app to the bot.",
      notEmployee: "The app is available to employees only. If you are one, make sure your Telegram is in your profile.",
      authFailed: "Sign-in failed: ",
      error: "Error: ",
      channelsTitle: "Your channels",
      channelsHint: "Protected channels and groups you administer.",
      noChannels: "No channels yet",
      noChannelsHint: "Connect one below or add it via the bot.",
      kindChannel: "Channel", kindGroup: "Group",
      join: "Join",
      joinHint: "You're an admin but don't manage it in the bot yet",
      joined: (t) => `You joined “${t}”`,
      availableTitle: "Available to connect",
      availableHint: "The bot is already an admin here, but protection is off. Reports will come to your private chat.",
      connect: "Connect",
      connectConfirm: (t) => `Protect “${t}”? Reports will come to your private chat with the bot.`,
      addViaBot: "Add a channel or group via the bot",
      health: {
        ok: (d) => `all good · checked ${d}`,
        never_checked: () => "never checked",
        stale: (d) => `not checked for 3+ days · ${d}`,
        very_stale: (d) => `not checked for a week · ${d}`,
        violations: (d, n) => `violations: ${n} · ${d}`,
      },
      tabScan: "Members", tabJoins: "Requests", tabWhitelist: "Whitelist", tabSettings: "Settings",
      warnNotAdmin: "The bot is not an administrator. Promote it, otherwise it can't scan.",
      warnCannotClean: "The bot can't ban members, so removing will fail.",
      warnCannotInvite: "The bot can't invite users, so approving requests will fail.",
      scanIntro: "The bot fetches the member list and checks everyone. Then you tick who to remove or whitelist.",
      scanStart: "Start scan",
      scanListing: "Fetching members…",
      scanChecking: (c, n) => `Checked ${c} of ${n}`,
      scanFailed: "Scan failed: ",
      partial: (f, t) => `Telegram returned only ${f} of ${t} members. The bot can't see or check the rest.`,
      noClean: "Manual cleaning is off. Turn on “Allow manual clean” in Settings to remove members.",
      legend: { good: "passes", whitelisted: "whitelist", unknown: "couldn't check", bad: "fails", kicked: "removed" },
      filterAll: "All", filterBad: "Failing", filterUnknown: "Unchecked", filterWhitelisted: "Whitelist", filterGood: "Passing", filterKicked: "Removed",
      search: "Search by name or @username",
      selectBad: "Tick everyone failing", selectNone: "Clear", rescan: "Scan again",
      nothingFound: "Nobody found",
      recheck: "Recheck",
      checkResult: { good: "checker: passes", bad: "checker: fails", unknown: "checker: couldn't check" },
      protectedAdmin: "administrator", protectedBot: "this bot",
      wlUntil: (d) => `whitelisted, review ${d}`,
      wlOverdue: "whitelisted · review overdue",
      wlDeleteAt: (d) => `until ${d}`,
      wlTempUntil: (d) => `whitelisted until ${d}, then removed`,
      wlTemporaryLeft: (d, left) => `temporary, removed on ${d} · in ${left} days`,
      kickButton: (n) => `Remove selected (${n})`,
      kickConfirm: (n, t) => `Remove ${n} member(s) from “${t}”?`,
      kickDone: (ok, all) => `Removed ${ok} of ${all}.`,
      failedSome: "Failed:",
      wlAddButton: (n) => `Add to whitelist (${n})`,
      wlFormTitle: (n) => `Whitelist ${n}`,
      wlNote: "Note: why they are allowed (optional)",
      wlTerm: "Term",
      wlTermReview: (days) => `No end, review every ${days} days`,
      wlTermDays: (n) => `${n} days, then remove`,
      wlTermCustom: "Custom number of days",
      wlDays: "Days",
      wlSubmit: "Add", cancel: "Cancel",
      wlAdded: (n) => `Added: ${n}.`,
      whitelistIntro: (days) => `Whitelisted people are not checked or removed. Every ${days} days they must be reviewed: the bot reminds 3 days ahead and daily once overdue. Overdue entries keep protecting. Temporary entries are removed at the end of their term.`,
      whitelistEmpty: "The whitelist is empty",
      whitelistEmptyHint: "Run a scan, tick people and press “Add to whitelist”.",
      wlActive: (d, left) => `review ${d} · in ${left} days`,
      wlExpired: (d) => `review overdue since ${d}`,
      wlTemporary: (d) => `removed on ${d}`,
      approvedBy: (who) => `approved by ${who}`,
      overdue: "review needed",
      renew: "Renew", remove: "Remove",
      renewTitle: (name) => `Renew for ${name}`,
      renewNote: "New note (optional)",
      renewTermKeep: "Keep the term",
      removeConfirm: (name) => `Remove ${name} from the whitelist? They will be checked again from the next scan.`,
      joinsIntroAuto: "Auto: the bot approves those who pass. The rest wait here and are rechecked daily.",
      joinsIntroManual: "Manual: approve or decline requests. The color is the check result.",
      joinsEmpty: "No pending requests",
      joinsEmptyHint: "The bot sees requests made after it was connected.",
      requested: (d) => `requested ${d}`,
      approveButton: (n) => `Approve (${n})`,
      declineButton: (n) => `Decline (${n})`,
      approveConfirm: (n) => `Approve ${n} request(s)?`,
      declineConfirm: (n) => `Decline ${n} request(s)?`,
      resolved: (ok, all) => `Done: ${ok} of ${all}.`,
      selectPassing: "Tick everyone passing",
      groupAutomation: "Automation", groupCleaning: "How to remove", groupJoins: "Join requests", groupAudit: "Action log",
      autoScan: "Auto scan", autoScanHint: "Check members periodically and report to the control chat.",
      autoClean: "Auto clean", autoCleanHint: "Periodically remove members who fail the check, without asking.",
      allowClean: "Allow manual clean", allowCleanHint: "Allow /clean and removing from this app.",
      keepBanned: "Keep banned", keepBannedHint: "Removed members can't come back via an invite link.",
      cleanMessages: "Delete messages", cleanMessagesHint: "Delete the removed member's messages.",
      cleanUnknown: "Remove unchecked", cleanUnknownHint: "Auto clean also removes members whose check failed.",
      joinModes: { off: "Off", auto: "Auto", manual: "Manual" },
      joinModeHint: {
        off: "Admins handle requests in Telegram; the bot leaves them alone.",
        auto: "The bot approves those who pass and rechecks the rest daily.",
        manual: "Requests wait for a manager's decision in this app.",
      },
      joinModeNeedsApproval: "Modes work when the chat approves new members.",
      defaultsNote: "The defaults apply now. Saving gives the channel its own settings.",
      save: "Save",
      auditEmpty: "Nothing yet",
      bot: "bot",
      source: { schedule: "on schedule", manual: "manual", miniapp: "in the app" },
      audit: {
        "channel.added": () => "connected the channel",
        "channel.joined": () => "joined as a manager",
        "settings.changed": () => "changed settings",
        "scan.completed": (e) => `scan ${src(e)}: ${counts(e)}`,
        "clean.completed": (e) => `clean ${src(e)}: ${counts(e)}`,
        "users.kicked": (e) => `removed ${e.counts ? e.counts.kicked : 0} of ${e.counts ? e.counts.selected : 0}: ${names(e)}`,
        "user.rechecked": (e) => `rechecked ${names(e)} → ${check(e.details && e.details.result)}`,
        "whitelist.added": (e) => `whitelisted ${names(e)}${note(e)}`,
        "whitelist.renewed": (e) => `renewed ${names(e)}${note(e)}`,
        "whitelist.removed": (e) => e.details && e.details.reason === "term_ended" ? `term ended, removed from whitelist: ${names(e)}` : `removed from whitelist ${names(e)}`,
        "whitelist.expired": (e) => `whitelist review overdue: ${names(e)}`,
        "join.requested": (e) => `request from ${names(e)} → ${check(e.details && e.details.check)}`,
        "join.approved": (e) => `${e.details && e.details.auto ? "auto-approved" : "approved"} ${names(e)}`,
        "join.declined": (e) => `declined ${names(e)}`,
        "join.rechecked": (e) => `rechecked request of ${names(e)} → ${check(e.details && e.details.check)}`,
      },
      countWords: { good: "passing", bad: "failing", unknown: "unchecked", whitelisted: "whitelist" },
    },
  };
  const lang = ((tg && tg.initDataUnsafe && tg.initDataUnsafe.user && tg.initDataUnsafe.user.language_code) || navigator.language || "en").slice(0, 2);
  const T = STRINGS[lang] || STRINGS.en;
  document.documentElement.lang = STRINGS[lang] ? lang : "en";

  // Helpers used by the audit strings above.
  function names(e) {
    return (e.users || []).map((u) => u.name || (u.username ? "@" + u.username : String(u.id))).join(", ");
  }
  function note(e) {
    return e.note ? ` — «${e.note}»` : "";
  }
  function src(e) {
    return T.source[(e.details && e.details.source) || "manual"] || "";
  }
  function counts(e) {
    const c = e.counts || {};
    return ["bad", "unknown", "whitelisted", "good"].filter((k) => c[k]).map((k) => `${T.countWords[k]} ${c[k]}`).join(", ") || "—";
  }
  function check(v) {
    return T.checkResult[v] || v || "";
  }

  // ---------- state & Telegram chrome ----------

  const app = document.getElementById("app");
  let session = null; // {token, userId, privileged, botUsername}
  let mainHandler = null;
  let backHandler = null;
  let pollTimer = null;
  let pollGen = 0; // bumped on every stop: late responses of older polls are dropped

  function setMainButton(text, handler, { destructive = false } = {}) {
    if (!tg) return;
    const mb = tg.MainButton;
    if (mainHandler) mb.offClick(mainHandler);
    mainHandler = null;
    if (!text) {
      mb.hide();
      return;
    }
    const tp = tg.themeParams || {};
    mb.setParams({
      text,
      is_visible: true,
      is_active: true,
      color: destructive && tp.destructive_text_color ? tp.destructive_text_color : tp.button_color,
      text_color: destructive && tp.destructive_text_color ? "#ffffff" : tp.button_text_color,
    });
    mainHandler = handler;
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

  const alertMsg = (text) => new Promise((resolve) => (tg ? tg.showAlert(text, resolve) : (window.alert(text), resolve())));
  const confirmMsg = (text) => new Promise((resolve) => (tg ? tg.showConfirm(text, resolve) : resolve(window.confirm(text))));
  const haptic = (kind) => tg && tg.HapticFeedback && tg.HapticFeedback.notificationOccurred(kind);

  // Lets the bot message the user in private (reports, reminders). Never
  // blocks the action: old clients don't support it and some never answer.
  const requestWriteAccess = () => new Promise((resolve) => {
    if (!tg || !tg.requestWriteAccess || !tg.isVersionAtLeast || !tg.isVersionAtLeast("6.9")) return resolve(false);
    const timer = setTimeout(() => resolve(false), 8000);
    try {
      tg.requestWriteAccess((granted) => {
        clearTimeout(timer);
        resolve(granted);
      });
    } catch (_) {
      clearTimeout(timer);
      resolve(false);
    }
  });

  // ---------- API ----------

  class ApiError extends Error {
    constructor(message, status, code) {
      super(message);
      this.status = status;
      this.code = code;
    }
  }

  async function signIn() {
    const res = await fetch(API + "auth", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ initData: tg.initData }),
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new ApiError(data.error || res.statusText, res.status, data.code);
    session = data;
  }

  async function api(path, { method = "GET", body } = {}, retried = false) {
    const res = await fetch(API + path, {
      method,
      headers: { "Content-Type": "application/json", Authorization: "Bearer " + session.token },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    if (res.status === 401 && !retried) {
      // Sessions are short (the employee check repeats on sign-in).
      await signIn();
      return api(path, { method, body }, true);
    }
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new ApiError(data.error || res.statusText, res.status, data.code);
    return data;
  }

  // ---------- DOM helpers ----------

  function el(tag, props = {}, children = []) {
    const node = document.createElement(tag);
    for (const [k, v] of Object.entries(props)) {
      if (k === "attrs") Object.entries(v).forEach(([a, val]) => node.setAttribute(a, val));
      else node[k] = v;
    }
    for (const child of children) if (child != null) node.append(child);
    return node;
  }
  const text = (t, cls) => el("span", { className: cls || "", textContent: t });
  const dot = (color, label) => el("span", { className: `dot ${color}`, attrs: label ? { role: "img", "aria-label": label } : { "aria-hidden": "true" } });
  const notice = (t, warn) => el("p", { className: warn ? "notice warn" : "notice", textContent: t });
  const button = (label, cls, onClick) => {
    const b = el("button", { type: "button", className: cls, textContent: label });
    b.addEventListener("click", onClick);
    return b;
  };

  function showError(err) {
    stopPolling();
    app.replaceChildren(notice(T.error + err.message, true));
  }
  function stopPolling() {
    clearTimeout(pollTimer);
    pollTimer = null;
    pollGen++;
  }

  function formatDate(iso) {
    return new Date(iso).toLocaleDateString(document.documentElement.lang, { day: "2-digit", month: "2-digit", year: "numeric" });
  }
  function formatDateTime(iso) {
    return new Date(iso).toLocaleString(document.documentElement.lang, { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" });
  }
  function daysLeft(iso) {
    return Math.max(0, Math.ceil((new Date(iso) - Date.now()) / 86400000));
  }
  function personName(u) {
    return [u.firstName, u.lastName].filter(Boolean).join(" ") || String(u.id || u.userId);
  }
  function actorName(id, name) {
    if (!id) return T.bot;
    return name || `id ${id}`;
  }
  const isChannel = (ch) => ch.chatType === "channel" || ch.type === "channel";

  // Traffic light colors of a member or requester.
  const STATUS_COLOR = { good: "green", whitelisted: "blue", unknown: "orange", bad: "red", kicked: "grey" };

  function legend(keys) {
    return el("div", { className: "legend" }, keys.map((k) => el("span", {}, [dot(STATUS_COLOR[k]), text(T.legend[k])])));
  }

  // ---------- channels ----------

  async function showChannels() {
    stopPolling();
    setBack(null);
    setMainButton(null);
    const [channels, available] = await Promise.all([api("channels"), api("available").catch(() => [])]);
    // Joined channels first, the ones needing attention on top.
    const severity = { red: 0, yellow: 1, green: 2 };
    channels.sort((a, b) => (Number(!a.joined) - Number(!b.joined))
      || ((severity[a.health.status] ?? 3) - (severity[b.health.status] ?? 3))
      || a.title.localeCompare(b.title));

    const list = el("ul", { className: "list" });
    for (const ch of channels) list.append(channelRow(ch));

    const nodes = [
      el("h1", { textContent: T.channelsTitle }),
      el("p", { className: "muted", textContent: T.channelsHint }),
    ];
    if (channels.length) nodes.push(list);
    else nodes.push(el("div", { className: "empty" }, [el("p", { textContent: T.noChannels }), el("p", { className: "muted", textContent: T.noChannelsHint })]));

    if (available.length) {
      const avail = el("ul", { className: "list" });
      for (const c of available) {
        const connect = button(T.connect, "link", async () => {
          if (!(await confirmMsg(T.connectConfirm(c.title)))) return;
          await requestWriteAccess();
          try {
            await api(`available/${c.id}/connect`, { method: "POST" });
            haptic("success");
          } catch (err) {
            haptic("error");
            await alertMsg(T.error + err.message);
          }
          showChannels().catch(showError);
        });
        avail.append(el("li", { className: "row" }, [
          dot("grey"),
          el("span", { className: "row-main" }, [text(c.title, "row-title"), text(isChannel(c) ? T.kindChannel : T.kindGroup, "row-sub")]),
          connect,
        ]));
      }
      nodes.push(el("h2", { textContent: T.availableTitle }), el("p", { className: "muted", textContent: T.availableHint }), avail);
    }

    if (session.botUsername && tg && tg.openTelegramLink) {
      nodes.push(el("div", { className: "stack", style: "margin-top:16px" }, [
        button(T.addViaBot, "secondary", () => {
          tg.openTelegramLink(`https://t.me/${session.botUsername}?start=add`);
          tg.close();
        }),
      ]));
    }
    app.replaceChildren(el("section", {}, nodes));
  }

  function healthText(h) {
    const last = h.lastCheck;
    const when = last ? formatDate(last.at) : "";
    const fn = T.health[h.reason] || T.health.ok;
    return fn(when, last ? last.bad : 0);
  }

  function channelRow(ch) {
    const sub = [isChannel(ch) ? T.kindChannel : T.kindGroup];
    if (!ch.joined) sub.push(T.joinHint);
    else if (!ch.botOnChannel) sub.push(T.warnNotAdmin);
    else sub.push(healthText(ch.health));

    const main = el("span", { className: "row-main" }, [text(ch.title, "row-title"), text(sub.join(" · "), "row-sub")]);
    if (!ch.joined) {
      const joinBtn = button(T.join, "link", async () => {
        await requestWriteAccess();
        try {
          await api(`channels/${ch.id}/join`, { method: "POST" });
          haptic("success");
          await alertMsg(T.joined(ch.title));
        } catch (err) {
          haptic("error");
          await alertMsg(T.error + err.message);
        }
        showChannels().catch(showError);
      });
      return el("li", { className: "row disabled" }, [dot("grey"), main, joinBtn]);
    }
    const item = el("button", { type: "button", className: "item" }, [
      el("span", { className: "row" }, [dot(ch.health.status, healthText(ch.health)), main, text("›", "muted")]),
    ]);
    item.addEventListener("click", () => showChannel(ch.id).catch(showError));
    return el("li", {}, [item]);
  }

  // ---------- channel ----------

  async function showChannel(channelId, tab = "scan") {
    stopPolling();
    setMainButton(null);
    setBack(() => showChannels().catch(showError));
    const channel = await api(`channels/${channelId}`);

    const tabs = [["scan", T.tabScan]];
    const joinsOn = channel.joinRequestsEnabled && channel.joinRequests && channel.joinRequests !== "off";
    if (joinsOn) tabs.push(["joins", T.tabJoins]);
    if (channel.whitelistEnabled) tabs.push(["whitelist", T.tabWhitelist]);
    tabs.push(["settings", T.tabSettings]);

    const warnings = [];
    if (!channel.botOnChannel) warnings.push(notice(T.warnNotAdmin, true));
    else if (!channel.botCanClean) warnings.push(notice(T.warnCannotClean, true));
    if (joinsOn && channel.botOnChannel && !channel.botCanInvite) warnings.push(notice(T.warnCannotInvite, true));

    const panel = el("div", { attrs: { role: "tabpanel" } });
    const tabBar = el("div", { className: "tabs", attrs: { role: "tablist" } });
    const tabButtons = tabs.map(([key, label]) => {
      const b = el("button", { type: "button", textContent: label, attrs: { role: "tab" } });
      b.addEventListener("click", () => select(key));
      tabBar.append(b);
      return [key, b];
    });
    function select(key) {
      stopPolling();
      setMainButton(null);
      for (const [k, b] of tabButtons) {
        b.setAttribute("aria-selected", String(k === key));
        b.tabIndex = k === key ? 0 : -1;
      }
      const render = { scan: renderScanIdle, joins: renderJoins, whitelist: renderWhitelist, settings: renderSettings }[key];
      Promise.resolve(render(panel, channel)).catch((err) => panel.replaceChildren(notice(T.error + err.message, true)));
    }
    tabBar.addEventListener("keydown", (e) => {
      if (e.key !== "ArrowLeft" && e.key !== "ArrowRight") return;
      const i = tabButtons.findIndex(([, b]) => b === document.activeElement);
      if (i < 0) return;
      const [key, b] = tabButtons[(i + (e.key === "ArrowRight" ? 1 : tabButtons.length - 1)) % tabButtons.length];
      b.focus();
      select(key);
    });

    const h = channel.health;
    app.replaceChildren(el("section", {}, [
      el("header", { className: "channel-head" }, [
        el("h1", {}, [dot(h.status, healthText(h)), text(channel.title)]),
        text(isChannel(channel) ? T.kindChannel : T.kindGroup, "badge"),
      ]),
      el("p", { className: "muted", textContent: healthText(h) }),
      ...warnings,
      tabBar,
      panel,
    ]));
    select(tabs.some(([k]) => k === tab) ? tab : "scan");
  }

  // ---------- members (scan) ----------

  function renderScanIdle(panel, channel) {
    const start = button(T.scanStart, "primary", () => startScan(panel, channel));
    start.disabled = !channel.botOnChannel;
    panel.replaceChildren(el("div", { className: "stack" }, [el("p", { className: "muted", textContent: T.scanIntro }), start]));
  }

  async function startScan(panel, channel) {
    try {
      const scan = await api(`channels/${channel.id}/scans`, { method: "POST" });
      pollScan(panel, channel, scan.id);
    } catch (err) {
      panel.replaceChildren(notice(T.scanFailed + err.message, true));
    }
  }

  function renderScanRunning(panel, scan) {
    let bar = panel.querySelector("progress");
    let label = panel.querySelector(".progress-label");
    if (!bar) {
      label = el("span", { className: "progress-label" });
      bar = el("progress", { attrs: { "aria-label": T.scanListing } });
      panel.replaceChildren(el("div", { className: "stack" }, [label, bar]));
    }
    const total = scan.stats && scan.stats.fetched;
    if (total) {
      bar.max = total;
      bar.value = scan.checked;
      label.textContent = T.scanChecking(scan.checked, total);
    } else {
      bar.removeAttribute("value");
      label.textContent = T.scanListing;
    }
  }

  async function pollScan(panel, channel, scanId) {
    stopPolling();
    const gen = pollGen;
    try {
      const scan = await api(`channels/${channel.id}/scans/${scanId}`);
      if (gen !== pollGen) return; // the user moved on meanwhile
      if (scan.state === "running") {
        renderScanRunning(panel, scan);
        pollTimer = setTimeout(() => pollScan(panel, channel, scanId), POLL_MS);
        return;
      }
      if (scan.state === "failed") {
        haptic("error");
        panel.replaceChildren(notice(T.scanFailed + scan.error, true));
        return;
      }
      renderScanResult(panel, channel, scan);
    } catch (err) {
      if (gen !== pollGen) return;
      panel.replaceChildren(notice(T.scanFailed + err.message, true));
    }
  }

  function memberSub(u) {
    const sub = [];
    if (u.username) sub.push("@" + u.username);
    if (u.whitelist) {
      const w = u.whitelist;
      if (w.deleteAt && new Date(w.deleteAt) <= new Date(w.until)) sub.push(T.wlTempUntil(formatDate(w.deleteAt)));
      else {
        sub.push(w.expired ? T.wlOverdue : T.wlUntil(formatDate(w.until)));
        if (w.deleteAt) sub.push(T.wlDeleteAt(formatDate(w.deleteAt)));
      }
    } else if (u.note === "administrator") sub.push(T.protectedAdmin);
    else if (u.note === "this bot") sub.push(T.protectedBot);
    if (u.check) sub.push(`${T.checkResult[u.check]} (${formatDateTime(u.checkedAt)})`);
    return sub.join(" · ");
  }

  function renderScanResult(panel, channel, scan) {
    const users = scan.users || [];
    const selected = new Set();
    const canKick = channel.allowClean && channel.botCanClean;
    const canWhitelist = Boolean(channel.whitelistEnabled);
    const selectable = (u) => (canKick || canWhitelist) && !u.protected && u.status !== "kicked";
    let filter = users.some((u) => u.status === "bad") ? "bad" : "all";
    let query = "";

    const counts = { all: users.length };
    for (const u of users) counts[u.status] = (counts[u.status] || 0) + 1;

    const filters = el("div", { className: "filters", attrs: { role: "radiogroup" } });
    for (const [key, label] of [["all", T.filterAll], ["bad", T.filterBad], ["unknown", T.filterUnknown], ["whitelisted", T.filterWhitelisted], ["good", T.filterGood], ["kicked", T.filterKicked]]) {
      if (key !== "all" && !counts[key]) continue;
      const input = el("input", { type: "radio", name: "filter", value: key, checked: key === filter });
      input.addEventListener("change", () => {
        filter = key;
        renderList();
      });
      filters.append(el("label", {}, [input, text(`${label} · ${counts[key] || 0}`)]));
    }

    const search = el("input", { type: "search", className: "search", placeholder: T.search, autocomplete: "off", attrs: { "aria-label": T.search } });
    search.addEventListener("input", () => {
      query = search.value.trim().toLowerCase().replace(/^@/, "");
      renderList();
    });

    const wlButton = button("", "secondary", () => openWhitelistForm());
    const formHolder = el("div");
    const list = el("ul", { className: "list" });
    const nothing = el("p", { className: "muted center", textContent: T.nothingFound, hidden: true });

    function row(u) {
      const color = STATUS_COLOR[u.status] || "grey";
      const box = el("input", { type: "checkbox", checked: selected.has(u.id), disabled: !selectable(u), attrs: { "aria-label": personName(u) } });
      box.addEventListener("change", () => {
        if (box.checked) selected.add(u.id);
        else selected.delete(u.id);
        updateActions();
      });
      const recheck = el("button", { type: "button", className: "icon", textContent: "↻", title: T.recheck, attrs: { "aria-label": `${T.recheck}: ${personName(u)}` } });
      recheck.hidden = u.status === "kicked";
      recheck.addEventListener("click", async () => {
        recheck.setAttribute("aria-busy", "true");
        try {
          const updated = await api(`channels/${channel.id}/scans/${scan.id}/users/${u.id}/recheck`, { method: "POST" });
          Object.assign(u, updated);
          haptic(updated.check === "good" ? "success" : "warning");
        } catch (err) {
          await alertMsg(T.error + err.message);
        }
        renderList();
      });
      const main = el("span", { className: "row-main" }, [
        el("span", { className: "row-lead" }, [dot(color, T.legend[u.status]), text(personName(u), "row-title")]),
        text(memberSub(u), "row-sub"),
      ]);
      if (u.whitelist && u.whitelist.note) main.append(text(u.whitelist.note, "row-note"));
      const label = el("label", { className: "row" + (selectable(u) ? "" : " disabled") }, [box, main, recheck]);
      return el("li", {}, [label]);
    }

    function renderList() {
      const visible = users.filter((u) =>
        (filter === "all" || u.status === filter) &&
        (!query || personName(u).toLowerCase().includes(query) || (u.username || "").toLowerCase().includes(query)));
      list.replaceChildren(...visible.map(row));
      nothing.hidden = visible.length > 0;
      updateActions();
    }

    function updateActions() {
      wlButton.hidden = !canWhitelist || selected.size === 0;
      wlButton.textContent = T.wlAddButton(selected.size);
      if (!canKick || selected.size === 0) setMainButton(null);
      else setMainButton(T.kickButton(selected.size), kick, { destructive: true });
    }

    function openWhitelistForm() {
      const ids = [...selected];
      const ttl = termSelect(false);
      const noteInput = el("textarea", { maxLength: 500, attrs: { "aria-label": T.wlNote } });
      const submit = button(T.wlSubmit, "primary", async () => {
        submit.disabled = true;
        try {
          const added = await api(`channels/${channel.id}/whitelist`, {
            method: "POST",
            body: { scanId: scan.id, userIds: ids, note: noteInput.value, ttlDays: ttl.days() },
          });
          haptic("success");
          await alertMsg(T.wlAdded(added.length));
          selected.clear();
          pollScan(panel, channel, scan.id);
        } catch (err) {
          haptic("error");
          submit.disabled = false;
          await alertMsg(T.error + err.message);
        }
      });
      formHolder.replaceChildren(el("div", { className: "panel" }, [
        el("strong", { textContent: T.wlFormTitle(ids.length) }),
        el("label", {}, [text(T.wlNote), noteInput]),
        el("label", {}, [text(T.wlTerm), ttl.node]),
        el("div", { className: "buttons" }, [button(T.cancel, "secondary", () => formHolder.replaceChildren()), submit]),
      ]));
      noteInput.focus();
    }

    async function kick() {
      const ids = [...selected];
      if (!(await confirmMsg(T.kickConfirm(ids.length, channel.title)))) return;
      if (tg) tg.MainButton.showProgress(false);
      try {
        const { results } = await api(`channels/${channel.id}/kick`, { method: "POST", body: { scanId: scan.id, userIds: ids } });
        await reportResults(results, T.kickDone, users);
      } catch (err) {
        haptic("error");
        await alertMsg(T.error + err.message);
      } finally {
        if (tg) tg.MainButton.hideProgress();
      }
      selected.clear();
      pollScan(panel, channel, scan.id);
    }

    const bar = el("div", { className: "bar" }, [
      button(T.selectBad, "link", () => {
        users.filter((u) => u.status === "bad" && selectable(u)).forEach((u) => selected.add(u.id));
        renderList();
      }),
      button(T.selectNone, "link", () => {
        selected.clear();
        renderList();
      }),
      button(T.rescan, "link", () => startScan(panel, channel)),
    ]);
    bar.children[0].hidden = bar.children[1].hidden = !canKick && !canWhitelist;

    panel.replaceChildren(el("div", { className: "stack" }, [
      scan.partial ? notice(T.partial(scan.stats.fetched, scan.stats.total), true) : null,
      channel.allowClean ? null : notice(T.noClean),
      legend(["good", "whitelisted", "unknown", "bad"]),
      filters,
      search,
      bar,
      wlButton,
      formHolder,
      list,
      nothing,
    ]));
    renderList();
  }

  async function reportResults(results, doneText, people) {
    const ok = results.filter((r) => r.ok).length;
    const failed = results.filter((r) => !r.ok);
    const byId = new Map((people || []).map((p) => [p.id || p.userId, p]));
    let msg = doneText(ok, results.length);
    if (failed.length) {
      msg += "\n\n" + T.failedSome + "\n" + failed.slice(0, 10)
        .map((r) => `• ${byId.has(r.userId) ? personName(byId.get(r.userId)) : r.userId}: ${r.error}`).join("\n");
    }
    haptic(failed.length ? "warning" : "success");
    await alertMsg(msg);
  }

  // Whitelist term picker: review cadence (no end) or a fixed number of days.
  function termSelect(allowKeep) {
    const reviewDays = 30;
    const sel = el("select");
    if (allowKeep) sel.append(el("option", { value: "keep", textContent: T.renewTermKeep }));
    sel.append(el("option", { value: "0", textContent: T.wlTermReview(reviewDays) }));
    for (const n of [1, 3, 7, 14]) sel.append(el("option", { value: String(n), textContent: T.wlTermDays(n) }));
    sel.append(el("option", { value: "custom", textContent: T.wlTermCustom }));
    const custom = el("input", { type: "number", min: 1, max: 365, value: 21, hidden: true, attrs: { "aria-label": T.wlDays } });
    sel.addEventListener("change", () => {
      custom.hidden = sel.value !== "custom";
    });
    return {
      node: el("span", { className: "stack" }, [sel, custom]),
      days: () => {
        if (sel.value === "custom") return Math.max(1, Math.min(365, parseInt(custom.value, 10) || 0));
        if (sel.value === "keep") return 0;
        return parseInt(sel.value, 10) || 0;
      },
    };
  }

  // ---------- join requests ----------

  async function renderJoins(panel, channel) {
    const requests = await api(`channels/${channel.id}/joins`);
    const selected = new Set();
    const colorOf = (r) => (r.whitelisted ? "blue" : STATUS_COLOR[r.check] || "orange");
    const list = el("ul", { className: "list" });
    const declineBtn = button("", "secondary danger", () => resolve(false));

    function row(r) {
      const box = el("input", { type: "checkbox", checked: selected.has(r.userId), attrs: { "aria-label": personName(r) } });
      box.addEventListener("change", () => {
        if (box.checked) selected.add(r.userId);
        else selected.delete(r.userId);
        update();
      });
      const recheck = el("button", { type: "button", className: "icon", textContent: "↻", title: T.recheck, attrs: { "aria-label": `${T.recheck}: ${personName(r)}` } });
      recheck.addEventListener("click", async () => {
        recheck.setAttribute("aria-busy", "true");
        try {
          await api(`channels/${channel.id}/joins/${r.userId}/recheck`, { method: "POST" });
        } catch (err) {
          await alertMsg(T.error + err.message);
        }
        renderJoins(panel, channel).catch(showError);
      });
      const sub = [];
      if (r.username) sub.push("@" + r.username);
      sub.push(r.whitelisted ? T.legend.whitelisted : T.checkResult[r.check] || r.check);
      sub.push(T.requested(formatDateTime(r.requestedAt)));
      const main = el("span", { className: "row-main" }, [
        el("span", { className: "row-lead" }, [dot(colorOf(r), sub[sub.length - 2]), text(personName(r), "row-title")]),
        text(sub.join(" · "), "row-sub"),
      ]);
      if (r.bio) main.append(text(r.bio, "row-note"));
      return el("li", {}, [el("label", { className: "row" }, [box, main, recheck])]);
    }

    function update() {
      declineBtn.hidden = selected.size === 0;
      declineBtn.textContent = T.declineButton(selected.size);
      if (selected.size === 0) setMainButton(null);
      else setMainButton(T.approveButton(selected.size), () => resolve(true));
    }

    async function resolve(approve) {
      const ids = [...selected];
      if (!(await confirmMsg((approve ? T.approveConfirm : T.declineConfirm)(ids.length)))) return;
      try {
        const { results } = await api(`channels/${channel.id}/joins/${approve ? "approve" : "decline"}`, { method: "POST", body: { userIds: ids } });
        await reportResults(results, T.resolved, requests);
      } catch (err) {
        haptic("error");
        await alertMsg(T.error + err.message);
      }
      setMainButton(null);
      renderJoins(panel, channel).catch(showError);
    }

    list.replaceChildren(...requests.map(row));
    const selectPassing = button(T.selectPassing, "link", () => {
      requests.filter((r) => r.check === "good" || r.whitelisted).forEach((r) => selected.add(r.userId));
      list.replaceChildren(...requests.map(row));
      update();
    });
    panel.replaceChildren(el("div", { className: "stack" }, [
      el("p", { className: "muted", textContent: channel.joinRequests === "auto" ? T.joinsIntroAuto : T.joinsIntroManual }),
      legend(["good", "whitelisted", "unknown", "bad"]),
      requests.length ? el("div", { className: "bar" }, [selectPassing]) : null,
      declineBtn,
      requests.length ? list : el("div", { className: "empty" }, [el("p", { textContent: T.joinsEmpty }), el("p", { className: "muted", textContent: T.joinsEmptyHint })]),
    ]));
    update();
  }

  // ---------- whitelist ----------

  async function renderWhitelist(panel, channel) {
    const entries = await api(`channels/${channel.id}/whitelist`);
    const list = el("ul", { className: "list" });
    const days = channel.whitelistTtlDays || 30;

    for (const e of entries) {
      const name = personName(e) + (e.username ? ` (@${e.username})` : "");
      const endsFirst = e.deleteAt && new Date(e.deleteAt) <= new Date(e.expiresAt);
      const sub = [];
      if (endsFirst) sub.push(T.wlTemporaryLeft(formatDate(e.deleteAt), daysLeft(e.deleteAt)));
      else {
        sub.push(e.expired ? T.wlExpired(formatDate(e.expiresAt)) : T.wlActive(formatDate(e.expiresAt), daysLeft(e.expiresAt)));
        if (e.deleteAt) sub.push(T.wlTemporary(formatDate(e.deleteAt)));
      }
      sub.push(T.approvedBy(`id ${e.approvedBy}`));
      const formHolder = el("div", { style: "grid-column: 1 / -1" });

      const renew = button(T.renew, "link", () => {
        const noteInput = el("textarea", { maxLength: 500, attrs: { "aria-label": T.renewNote } });
        const ttl = termSelect(true);
        const submit = button(T.renew, "primary", async () => {
          submit.disabled = true;
          try {
            await api(`channels/${channel.id}/whitelist/${e.userId}/renew`, { method: "POST", body: { note: noteInput.value, ttlDays: ttl.days() } });
            haptic("success");
          } catch (err) {
            haptic("error");
            await alertMsg(T.error + err.message);
          }
          renderWhitelist(panel, channel).catch(showError);
        });
        formHolder.replaceChildren(el("div", { className: "panel" }, [
          el("strong", { textContent: T.renewTitle(name) }),
          el("label", {}, [text(T.renewNote), noteInput]),
          el("label", {}, [text(T.wlTerm), ttl.node]),
          el("div", { className: "buttons" }, [button(T.cancel, "secondary", () => formHolder.replaceChildren()), submit]),
        ]));
      });
      const remove = button(T.remove, "link danger", async () => {
        if (!(await confirmMsg(T.removeConfirm(name)))) return;
        try {
          await api(`channels/${channel.id}/whitelist/${e.userId}`, { method: "DELETE" });
          haptic("success");
        } catch (err) {
          haptic("error");
          await alertMsg(T.error + err.message);
        }
        renderWhitelist(panel, channel).catch(showError);
      });

      const main = el("span", { className: "row-main" }, [text(name, "row-title"), text(sub.join(" · "), "row-sub")]);
      if (e.note) main.append(text(e.note, "row-note"));
      list.append(el("li", { className: "row" }, [
        dot("blue", T.legend.whitelisted),
        main,
        e.expired ? text(T.overdue, "badge warn") : text(""),
        el("span", { className: "row-actions" }, [renew, remove]),
        formHolder,
      ]));
    }

    panel.replaceChildren(el("div", { className: "stack" }, [
      el("p", { className: "muted", textContent: T.whitelistIntro(days) }),
      entries.length ? list : el("div", { className: "empty" }, [el("p", { textContent: T.whitelistEmpty }), el("p", { className: "muted", textContent: T.whitelistEmptyHint })]),
    ]));
  }

  // ---------- settings & audit ----------

  const SWITCHES = [
    ["automation", "autoScan"], ["automation", "autoClean"], ["automation", "allowClean"],
    ["cleaning", "keepBanned"], ["cleaning", "cleanMessages"], ["cleaning", "cleanUnknown"],
  ];

  async function renderSettings(panel, channel) {
    const initial = Object.fromEntries(SWITCHES.map(([, k]) => [k, Boolean(channel[k])]));
    initial.joinRequests = channel.joinRequests || "off";
    const current = { ...initial };
    const groups = { automation: el("div"), cleaning: el("div") };

    for (const [group, key] of SWITCHES) {
      const input = el("input", { type: "checkbox", checked: current[key], name: key, attrs: { role: "switch" } });
      input.addEventListener("change", () => {
        current[key] = input.checked;
        updateSave();
      });
      groups[group].append(el("label", { className: "switch" }, [text(T[key]), input, el("small", { textContent: T[key + "Hint"] })]));
    }

    const fieldsets = [
      el("fieldset", {}, [el("legend", { textContent: T.groupAutomation }), groups.automation]),
      el("fieldset", {}, [el("legend", { textContent: T.groupCleaning }), groups.cleaning]),
    ];

    if (channel.joinRequestsEnabled) {
      const hint = el("div", { className: "hint-row" });
      const seg = el("div", { className: "segmented", attrs: { role: "radiogroup", "aria-label": T.groupJoins } });
      const setHint = () => {
        hint.textContent = `${T.joinModeHint[current.joinRequests]} ${current.joinRequests === "off" ? "" : T.joinModeNeedsApproval}`;
      };
      for (const mode of ["off", "auto", "manual"]) {
        const input = el("input", { type: "radio", name: "joinRequests", value: mode, checked: current.joinRequests === mode });
        input.addEventListener("change", () => {
          current.joinRequests = mode;
          setHint();
          updateSave();
        });
        seg.append(el("label", {}, [input, text(T.joinModes[mode])]));
      }
      setHint();
      fieldsets.push(el("fieldset", {}, [el("legend", { textContent: T.groupJoins }), el("div", {}, [seg, hint])]));
    }

    const auditList = el("ul", { className: "list audit" });
    const nodes = [
      el("form", { className: "stack" }, fieldsets),
      channel.customCleanOptions ? null : el("p", { className: "muted", textContent: T.defaultsNote }),
      el("h2", { textContent: T.groupAudit }),
      auditList,
    ];
    panel.replaceChildren(el("div", { className: "stack" }, nodes));

    const dirty = () => Object.keys(current).some((k) => current[k] !== initial[k]) || !channel.customCleanOptions;
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
        // The Requests tab depends on the mode: rebuild the channel screen.
        await showChannel(channel.id, "settings");
      } catch (err) {
        haptic("error");
        await alertMsg(T.error + err.message);
      } finally {
        if (tg) tg.MainButton.hideProgress();
      }
    }
    updateSave();

    const events = await api(`channels/${channel.id}/audit?limit=${AUDIT_PAGE}`);
    if (!events.length) auditList.replaceWith(el("p", { className: "muted", textContent: T.auditEmpty }));
    for (const e of events) {
      const describe = T.audit[e.action];
      auditList.append(el("li", {}, [
        el("time", { textContent: formatDateTime(e.at), attrs: { datetime: e.at } }),
        text(`${actorName(e.actorId, e.actorName)}: ${describe ? describe(e) : e.action}`),
      ]));
    }
  }

  // ---------- boot ----------

  async function boot() {
    if (!tg || !tg.initData) {
      app.replaceChildren(notice(T.openInTelegram));
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
      await signIn();
    } catch (err) {
      app.replaceChildren(notice(err.code === "not_employee" ? T.notEmployee : T.authFailed + err.message, true));
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
