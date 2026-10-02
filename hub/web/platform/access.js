'use strict';
const byId = id => document.getElementById(id);
const actions = [
  ['node.read', '节点信息', true], ['file.read', '读取文件', true],
  ['command.read', '命令结果', true], ['docker.read', 'Docker（预留）', true], ['terminal.open', '终端', false],
  ['file.write', '修改文件', false], ['command.exec', '执行命令', false],
];
let users = [], nodes = [], grants = new Set();
let loading = false, policyReady = false, generation = 0;

function text(tag, value) { const node = document.createElement(tag); node.textContent = value; return node; }
function announce(value, error = false) { byId('message').textContent = value; byId('message').className = error ? 'error' : ''; }
async function request(path, method = 'GET', body) {
  const headers = { 'Content-Type': 'application/json' };
  const otp = byId('otp').value.trim();
  if (otp && method !== 'GET') headers['X-2FA-Code'] = otp;
  const response = await fetch(path, { method, credentials: 'same-origin', headers, body: body === undefined ? undefined : JSON.stringify(body) });
  const result = await response.json();
  if (!response.ok || result.status === 'error') throw new Error(result.message || '请求失败，请检查登录状态和权限。');
  return result.status === 'success' ? result.data : result;
}
function renderScope() {
  const head = byId('scope').tHead, body = byId('scope').tBodies[0]; head.replaceChildren(); body.replaceChildren();
  const row = document.createElement('tr'); row.append(text('th', '节点'));
  actions.forEach(([, title]) => row.append(text('th', title))); head.append(row);
  const role = byId('role').value;
  for (const node of nodes) {
    const row = document.createElement('tr'), title = document.createElement('td');
    title.append(text('span', node.name || node.uuid), text('small', node.uuid)); row.append(title);
    for (const [action, label, readOnly] of actions) {
      const cell = document.createElement('td'), boxLabel = document.createElement('label'), input = document.createElement('input');
      const key = `${node.uuid}|${action}`;
      input.type = 'checkbox'; input.setAttribute('aria-label', `${node.name || node.uuid}：${label}`);
      input.disabled = loading || role === 'disabled' || (role === 'viewer' && !readOnly);
      input.checked = !input.disabled && grants.has(key);
      input.addEventListener('change', () => input.checked ? grants.add(key) : grants.delete(key));
      boxLabel.append(input); cell.append(boxLabel); row.append(cell);
    }
    body.append(row);
  }
  if (!nodes.length) { const row = document.createElement('tr'), cell = text('td', '暂无节点，请先在控制台添加节点。'); cell.colSpan = actions.length + 1; row.append(cell); body.append(row); }
}
async function selectUser() {
  const version = ++generation, id = byId('user').value;
  policyReady = false; byId('save').disabled = true;
  grants = new Set();
  const user = users.find(item => item.uuid === id), owner = user?.access_role === 'owner';
  byId('ownerHint').hidden = !owner; byId('policyForm').hidden = owner;
  byId('username').readOnly = Boolean(id); byId('username').value = user?.username || '';
  byId('passwordField').hidden = Boolean(id); byId('password').required = !id; byId('password').value = '';
  byId('role').value = owner ? 'viewer' : (user?.access_role || 'viewer');
  byId('save').textContent = id ? '保存完整策略' : '创建账户';
  if (id && !owner) {
    try {
      const policy = await request(`/api/admin/access/users/${encodeURIComponent(id)}`);
      if (version !== generation) return;
      grants = new Set(policy.grants.map(item => `${item.client_uuid}|${item.action}`));
      policyReady = true;
    } catch (error) { if (version === generation) announce(error.message, true); }
  }
  if (!id) policyReady = true;
  if (version === generation) { byId('save').disabled = loading || !policyReady; renderScope(); }
}
async function refreshAudit() {
  const events = await request('/api/admin/access/audit', 'POST', { limit: 30 });
  const body = byId('audit').tBodies[0]; body.replaceChildren();
  const names = new Map(users.map(user => [user.uuid, user.username]));
  for (const event of events) {
    const row = document.createElement('tr');
    const values = [new Date(event.time).toLocaleString(), names.get(event.actor) || event.actor, event.action, nodes.find(node => node.uuid === event.client_uuid)?.name || event.client_uuid || '—', { allowed: '已授权', denied: '已拒绝', changed: '策略已更新' }[event.outcome] || event.outcome];
    values.forEach(value => row.append(text('td', value))); body.append(row);
  }
}
async function load(selected = byId('user').value) {
  loading = true; policyReady = false; byId('save').disabled = true; byId('reload').disabled = true;
  try {
    [users, nodes] = await Promise.all([request('/api/admin/access/users'), request('/api/admin/client/list')]);
    const select = byId('user'); select.replaceChildren(); const option = text('option', '新建受限账户'); option.value = ''; select.append(option);
    for (const user of users) { const option = text('option', `${user.username} · ${user.access_role}`); option.value = user.uuid; select.append(option); }
    select.value = users.some(user => user.uuid === selected) ? selected : '';
    await selectUser(); await refreshAudit();
  } catch (error) { announce(error.message, true); }
  finally { loading = false; byId('save').disabled = !policyReady; byId('reload').disabled = false; renderScope(); }
}
byId('user').addEventListener('change', () => { announce(''); selectUser(); });
byId('role').addEventListener('change', renderScope);
byId('reload').addEventListener('click', () => { announce(''); load(); });
byId('policyForm').addEventListener('submit', async event => {
  event.preventDefault(); if (loading || !policyReady) return;
  const id = byId('user').value, role = byId('role').value;
  const allowed = new Set(actions.filter(([, , readOnly]) => role === 'operator' || (role === 'viewer' && readOnly)).map(([action]) => action));
  const entries = [...grants].map(key => { const [client_uuid, action] = key.split('|'); return { client_uuid, action }; }).filter(item => allowed.has(item.action));
  const body = { role, grants: role === 'disabled' ? [] : entries };
  if (!id) { body.username = byId('username').value.trim(); body.password = byId('password').value; const length = new TextEncoder().encode(body.password).length; if (length < 12 || length > 72) { announce('密码长度须为 12–72 字节。', true); return; } }
  loading = true; byId('save').disabled = true;
  try {
    const result = await request(id ? `/api/admin/access/users/${encodeURIComponent(id)}` : '/api/admin/access/users', id ? 'PUT' : 'POST', body);
    byId('password').value = ''; byId('otp').value = ''; announce(id ? '策略已保存。' : '受限账户已创建。');
    await load(id || result.uuid);
  } catch (error) { announce(error.message, true); }
  finally { loading = false; byId('save').disabled = !policyReady; }
});
load();
