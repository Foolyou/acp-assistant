package daemon

const consoleHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>ACPA Console</title>
  <style>
    :root { color-scheme: light; font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
    body { margin: 0; background: #f7f8fa; color: #1f2933; }
    header { background: #102033; color: #fff; padding: 18px 24px; }
    main { display: grid; grid-template-columns: minmax(0, 1fr) 360px; gap: 20px; padding: 20px; max-width: 1280px; margin: 0 auto; }
    h1 { margin: 0; font-size: 22px; font-weight: 650; }
    h2 { margin: 0 0 12px; font-size: 16px; }
    section, form { background: #fff; border: 1px solid #d7dde5; border-radius: 8px; padding: 16px; }
    table { width: 100%; border-collapse: collapse; }
    th, td { padding: 10px 8px; border-bottom: 1px solid #e6eaf0; text-align: left; font-size: 14px; vertical-align: top; }
    th { color: #516070; font-weight: 600; }
    input, select { width: 100%; box-sizing: border-box; padding: 8px; border: 1px solid #c8d0da; border-radius: 6px; font: inherit; }
    label { display: grid; gap: 5px; margin: 10px 0; font-size: 13px; color: #445466; }
    button { border: 1px solid #b9c4d0; background: #fff; border-radius: 6px; padding: 7px 10px; cursor: pointer; }
    button.primary { background: #136f63; border-color: #136f63; color: #fff; }
    .actions { display: flex; gap: 6px; flex-wrap: wrap; }
    .stack { display: grid; gap: 20px; }
    .muted { color: #667789; }
    .error { color: #b42318; white-space: pre-wrap; }
    @media (max-width: 900px) { main { grid-template-columns: 1fr; padding: 12px; } }
  </style>
</head>
<body>
  <header><h1>ACPA Console</h1><div id="daemon" class="muted"></div></header>
  <main>
    <section>
      <h2>Assistants</h2>
      <table>
        <thead><tr><th>Name</th><th>Workspace</th><th>Status</th><th>Autostart</th><th></th></tr></thead>
        <tbody id="assistants"></tbody>
      </table>
    </section>
    <div class="stack">
      <form id="create-form">
        <h2>Create Assistant</h2>
        <label>Name <input name="name" required></label>
        <label>Root <input name="root_path" placeholder="default ACPA assistant root"></label>
        <label>Harness <select name="harness"><option value="codex">Codex</option><option value="claude">Claude</option></select></label>
        <label><input name="autostart" type="checkbox" checked> Autostart</label>
        <button class="primary">Create</button>
      </form>
      <form id="feishu-form">
        <h2>Manual Feishu Setup</h2>
        <label>Assistant ID <input name="assistant_id" required></label>
        <label>Channel ID <input name="channel_id" value="feishu-main"></label>
        <label>App ID <input name="app_id" required></label>
        <label>App Secret <input name="app_secret" required type="password"></label>
        <button class="primary">Save Feishu App</button>
      </form>
      <div id="message" class="error"></div>
    </div>
  </main>
  <script>
    const $ = (id) => document.getElementById(id);
    async function api(path, options = {}) {
      const res = await fetch(path, options);
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || res.statusText);
      return data;
    }
    async function refresh() {
      const status = await api('/api/status');
      $('daemon').textContent = status.endpoint + ' - ' + status.running_count + '/' + status.assistant_count + ' running';
      $('assistants').innerHTML = status.assistants.map(a =>
        '<tr>' +
        '<td><strong>' + a.name + '</strong><br><span class="muted">' + a.id + '</span></td>' +
        '<td>' + a.workspace_path + '</td>' +
        '<td>' + (a.running ? 'running pid ' + a.pid : 'stopped') + (a.last_error ? '<br><span class="error">' + a.last_error + '</span>' : '') + '</td>' +
        '<td><input type="checkbox" ' + (a.autostart ? 'checked' : '') + ' onchange="setAutostart(\'' + a.id + '\', this.checked)"></td>' +
        '<td><div class="actions"><button onclick="act(\'' + a.id + '\',\'start\')">Start</button><button onclick="act(\'' + a.id + '\',\'stop\')">Stop</button><button onclick="act(\'' + a.id + '\',\'restart\')">Restart</button></div></td>' +
        '</tr>').join('');
    }
    async function act(id, action) {
      try { await api('/api/assistants/' + id + '/' + action, { method: 'POST' }); await refresh(); } catch (e) { $('message').textContent = e.message; }
    }
    async function setAutostart(id, enabled) {
      try { await api('/api/assistants/' + id + '/autostart', { method: 'POST', body: JSON.stringify({ enabled }) }); await refresh(); } catch (e) { $('message').textContent = e.message; }
    }
    $('create-form').addEventListener('submit', async (event) => {
      event.preventDefault();
      const form = new FormData(event.target);
      try {
        await api('/api/assistants', { method: 'POST', body: JSON.stringify({ name: form.get('name'), root_path: form.get('root_path'), harness: form.get('harness'), autostart: event.target.autostart.checked }) });
        event.target.reset(); event.target.autostart.checked = true; await refresh();
      } catch (e) { $('message').textContent = e.message; }
    });
    $('feishu-form').addEventListener('submit', async (event) => {
      event.preventDefault();
      const form = new FormData(event.target);
      try {
        await api('/api/setup/feishu/manual', { method: 'POST', body: JSON.stringify(Object.fromEntries(form.entries())) });
        await refresh();
      } catch (e) { $('message').textContent = e.message; }
    });
    refresh();
  </script>
</body>
</html>`
