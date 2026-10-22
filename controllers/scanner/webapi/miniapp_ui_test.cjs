// Run: node --test controllers/scanner/webapi/miniapp_ui_test.cjs
// Exercise the shipped client, with Telegram/fetch replaced by test doubles.
const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');

class Element {
  constructor(tag) { this.tag = tag; this.children = []; this.events = {}; this.attrs = {}; }
  append(...nodes) { this.children.push(...nodes); }
  replaceChildren(...nodes) { this.children = nodes; }
  replaceWith(node) { this.replacement = node; }
  setAttribute(key, value) { this.attrs[key] = value; }
  removeAttribute(key) { delete this.attrs[key]; }
  addEventListener(key, fn) { this.events[key] = fn; }
  querySelector() { return null; }
  focus() {}
}

function client(fetch) {
  const app = new Element('main');
  let main;
  const chrome = { hide() {}, show() {}, offClick() {}, onClick() {} };
  const telegram = {
    initDataUnsafe: { user: { language_code: 'ru' } },
    showConfirm(_text, done) { done(true); },
    MainButton: { ...chrome, setParams() {}, onClick(fn) { main = fn; }, offClick() { main = null; }, showProgress() {}, hideProgress() {} },
    BackButton: chrome,
    HapticFeedback: { notificationOccurred() {} },
  };
  const context = vm.createContext({ window: { Telegram: { WebApp: telegram } }, navigator: { language: 'ru' }, document: { documentElement: {}, getElementById: () => app, createElement: (tag) => new Element(tag) }, fetch, setTimeout, clearTimeout });
  const source = fs.readFileSync(path.join(__dirname, 'webui/miniapp/app.js'), 'utf8');
  assert.ok(source.includes('boot().catch(showError);'));
  vm.runInContext(source.replace('boot().catch(showError);', 'session = {token:"test"}; globalThis.testClient = {api, renderSettings, showChannels, stopPolling};'), context);
  return { ...context.testClient, app, main: () => main };
}
function response(status, body = {}) { return { status, ok: status >= 200 && status < 300, json: async () => body }; }

for (const method of ['GET', 'PUT']) test(`not_joined recovers ${method} once and preserves the request`, async () => {
  const calls = [];
  let joined = false;
  const c = client(async (url, req) => {
    calls.push({ url, ...req });
    if (url.endsWith('/join')) { joined = true; return response(200); }
    return joined ? response(200, { saved: true }) : response(403, { code: 'not_joined', error: 'join the channel in the bot first' });
  });
  const result = await c.api('channels/-100/settings', { method, body: { autoScan: true } });
  assert.equal(result.saved, true);
  assert.deepEqual(calls.map((r) => r.method), [method, 'POST', method]);
  assert.equal(calls[0].body, calls[2].body);
});

test('not_admin never attempts to join or retry the write', async () => {
  const calls = [];
  const c = client(async (url) => { calls.push(url); return response(403, { code: 'not_admin' }); });
  await assert.rejects(c.api('channels/-100/settings', { method: 'PUT' }), (e) => e.code === 'not_admin');
  assert.equal(calls.length, 1);
});

test('failed manager recovery is bounded and gives a Russian message', async () => {
  const calls = [];
  const c = client(async (url) => { calls.push(url); return url.endsWith('/join') ? response(200) : response(403, { code: 'not_joined' }); });
  await assert.rejects(c.api('channels/-100/settings'), (e) => e.code === 'not_joined' && e.message.includes('Вернитесь в список'));
  assert.equal(calls.length, 3);
});

test('parallel requests share the same manager recovery', async () => {
  let joined = false;
  let joins = 0;
  let release;
  const gate = new Promise((resolve) => { release = resolve; });
  const c = client(async (url) => {
    if (url.endsWith('/join')) { joins++; await gate; joined = true; return response(200); }
    return response(joined ? 200 : 403, joined ? { ok:true } : { code:'not_joined' });
  });
  const one = c.api('channels/-100/audit');
  const two = c.api('channels/-100/settings');
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(joins, 1);
  release();
  await Promise.all([one, two]);
  assert.equal(joins, 1);
});

function descendants(element) { return [element, ...(element.children || []).flatMap(descendants)]; }

test('notification controls submit the selected mode and interval via Telegram MainButton', async () => {
  let saved;
  const channel = { id:-100, title:'Test', chatType:'supergroup', health:{status:'green', reason:'ok'}, botOnChannel:true, botCanClean:true, allowClean:true, notifications:{mode:'violations_summary',intervalHours:24} };
  const c = client(async (url, req) => {
    if (url.includes('/audit')) return response(200, []);
    if (req.method === 'PUT') { saved = JSON.parse(req.body); return response(200, { ...channel, ...saved, customCleanOptions:true }); }
    return response(200, { ...channel, ...saved, customCleanOptions:true });
  });
  const panel = new Element('div');
  await c.renderSettings(panel, channel);
  const elements = descendants(panel);
  const mode = elements.find((el) => el.tag === 'select' && el.attrs['aria-label'] === 'Уведомления по этому чату');
  const period = elements.find((el) => el.tag === 'select' && el.attrs['aria-label'] === 'Периодичность сводок');
  const history = elements.find((el) => el.name === 'cleanMessages');
  assert.equal(history.disabled, true);
  assert.equal(history.checked, true);
  mode.value = 'violations_only'; mode.events.change();
  period.value = '72'; period.events.change();
  assert.equal(typeof c.main(), 'function');
  await c.main()();
  assert.deepEqual(saved.notifications, { mode:'violations_only', intervalHours:72 });
  assert.equal(saved.cleanMessages, true);
  assert.equal(saved.notificationMode, undefined);
});


test('connect immediately shows a disabled loading row while the API is pending', async () => {
  let release;
  const gate = new Promise((resolve) => { release = resolve; });
  const c = client(async (url) => {
    if (url.endsWith('/connect')) { await gate; return response(200); }
    if (url.endsWith('available')) return response(200, [{ id:-900, title:'New group', type:'supergroup', addedBy:5, addedByName:'Artem' }]);
    return response(200, []);
  });
  await c.showChannels();
  const connect = descendants(c.app).find((el) => el.tag === 'button' && el.textContent === 'Подключить');
  const pending = connect.events.click();
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(connect.disabled, true);
  assert.equal(connect.attrs['aria-busy'], 'true');
  assert.ok(connect.children.some((el) => el.className === 'spinner'));
  release();
  await pending;
  c.stopPolling();
});
