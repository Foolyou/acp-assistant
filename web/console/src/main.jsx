import React, { useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import "./styles.css";

const apiBase = new URL("api/", window.location.href);

function apiURL(path) {
  return new URL(String(path).replace(/^\/+/, ""), apiBase).toString();
}

async function api(path, options = {}) {
  const response = await fetch(apiURL(path), {
    headers: { "Content-Type": "application/json", ...(options.headers || {}) },
    ...options,
  });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(data.error || response.statusText);
  }
  return data;
}

function compactPath(value) {
  if (!value) return "No workspace";
  const parts = String(value).split("/").filter(Boolean);
  if (parts.length <= 3) return value;
  return `.../${parts.slice(-3).join("/")}`;
}

function formatTime(date) {
  if (!date) return "Never";
  return new Intl.DateTimeFormat(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(date);
}

function statusTone(value) {
  if (value === "running" || value === "pass") return "ok";
  if (value === "warn" || value === "stopped") return "warn";
  return "bad";
}

function App() {
  const [status, setStatus] = useState(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [lastRefresh, setLastRefresh] = useState(null);
  const [sheet, setSheet] = useState(null);
  const [selectedID, setSelectedID] = useState("");

  const assistants = status?.assistants || [];
  const selected = assistants.find((item) => item.id === selectedID) || assistants[0] || null;
  const attention = assistants.filter((assistant) => assistant.last_error || !assistant.running);

  async function refresh() {
    setLoading(true);
    setError("");
    try {
      const next = await api("status");
      setStatus(next);
      setLastRefresh(new Date());
      if (!selectedID && next.assistants?.length) setSelectedID(next.assistants[0].id);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    refresh();
  }, []);

  async function runAction(id, action, body) {
    setBusy(`${id}:${action}`);
    setError("");
    setMessage("");
    try {
      await api(`assistants/${id}/${action}`, {
        method: "POST",
        body: body ? JSON.stringify(body) : undefined,
      });
      setMessage(`${action} submitted for ${id}`);
      await refresh();
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy("");
      setSheet(null);
    }
  }

  return (
    <div className="app-shell">
      <header className="app-bar">
        <div>
          <p className="eyebrow">Local control plane</p>
          <h1>ACPA Console</h1>
        </div>
        <button className="icon-button" type="button" onClick={refresh} aria-label="Refresh console">
          ↻
        </button>
      </header>

      <main className="layout">
        <section className="dashboard" aria-label="Console dashboard">
          <HealthStrip status={status} loading={loading} lastRefresh={lastRefresh} />
          {error && <Alert tone="bad" title="Request failed" text={error} />}
          {message && <Alert tone="ok" title="Updated" text={message} />}
          {!loading && attention.length > 0 && (
            <Attention assistants={attention} onSelect={(assistant) => setSelectedID(assistant.id)} />
          )}
          <div className="section-head">
            <div>
              <h2>Assistants</h2>
              <p>{assistants.length ? `${assistants.length} configured` : "Create an assistant to start"}</p>
            </div>
            <button className="primary" type="button" onClick={() => setSheet({ type: "create" })}>
              + New
            </button>
          </div>
          {loading ? (
            <SkeletonList />
          ) : assistants.length ? (
            <div className="card-grid">
              {assistants.map((assistant) => (
                <AssistantCard
                  key={assistant.id}
                  assistant={assistant}
                  active={selected?.id === assistant.id}
                  busy={busy.startsWith(`${assistant.id}:`)}
                  onSelect={() => setSelectedID(assistant.id)}
                  onAction={(action) => {
                    if (action === "stop" || action === "restart") {
                      setSheet({ type: "confirm", assistant, action });
                    } else if (action === "feishu") {
                      setSheet({ type: "feishu", assistant });
                    } else if (action === "doctor") {
                      setSheet({ type: "doctor", assistant });
                    } else {
                      runAction(assistant.id, action);
                    }
                  }}
                  onAutostart={(enabled) => runAction(assistant.id, "autostart", { enabled })}
                />
              ))}
            </div>
          ) : (
            <EmptyState onCreate={() => setSheet({ type: "create" })} />
          )}
        </section>

        <aside className="detail-panel" aria-label="Selected assistant details">
          {selected ? (
            <AssistantDetail
              assistant={selected}
              onAction={(action) => {
                if (action === "doctor") setSheet({ type: "doctor", assistant: selected });
                if (action === "feishu") setSheet({ type: "feishu", assistant: selected });
                if (action === "restart" || action === "stop") setSheet({ type: "confirm", assistant: selected, action });
                if (action === "start") runAction(selected.id, "start");
              }}
            />
          ) : (
            <div className="panel-empty">Select an assistant to inspect lifecycle details.</div>
          )}
        </aside>
      </main>

      <button className="fab" type="button" onClick={() => setSheet({ type: "create" })} aria-label="Create assistant">
        +
      </button>

      {sheet && (
        <Sheet title={sheetTitle(sheet)} onClose={() => setSheet(null)}>
          {sheet.type === "create" && <CreateAssistantForm onDone={refresh} onClose={() => setSheet(null)} onError={setError} />}
          {sheet.type === "feishu" && <FeishuSetup assistant={sheet.assistant} onDone={refresh} onError={setError} />}
          {sheet.type === "confirm" && (
            <ConfirmAction
              assistant={sheet.assistant}
              action={sheet.action}
              busy={busy !== ""}
              onCancel={() => setSheet(null)}
              onConfirm={() => runAction(sheet.assistant.id, sheet.action)}
            />
          )}
          {sheet.type === "doctor" && <DoctorPanel assistant={sheet.assistant} />}
        </Sheet>
      )}
    </div>
  );
}

function HealthStrip({ status, loading, lastRefresh }) {
  const running = status?.running_count ?? 0;
  const total = status?.assistant_count ?? 0;
  const healthy = status?.reachable && running === total;
  const tone = loading ? "warn" : healthy ? "ok" : total === 0 ? "warn" : "bad";
  return (
    <section className={`health-strip ${tone}`} aria-live="polite">
      <div>
        <span className={`status-dot ${tone}`} />
        <strong>{loading ? "Checking daemon" : status?.reachable ? "Daemon running" : "Daemon unavailable"}</strong>
        <p>{status?.endpoint || "Local endpoint not available"}</p>
      </div>
      <div className="health-metrics">
        <Metric label="Running" value={`${running}/${total}`} />
        <Metric label="Last check" value={formatTime(lastRefresh)} />
      </div>
    </section>
  );
}

function Metric({ label, value }) {
  return (
    <span>
      <small>{label}</small>
      <strong>{value}</strong>
    </span>
  );
}

function Alert({ tone, title, text }) {
  return (
    <div className={`alert ${tone}`} role={tone === "bad" ? "alert" : "status"}>
      <strong>{title}</strong>
      <span>{text}</span>
    </div>
  );
}

function Attention({ assistants, onSelect }) {
  return (
    <section className="attention" aria-label="Attention required">
      <div>
        <h2>Needs attention</h2>
        <p>{assistants.length} assistant{assistants.length > 1 ? "s" : ""} stopped or reporting errors.</p>
      </div>
      <div className="attention-list">
        {assistants.slice(0, 3).map((assistant) => (
          <button key={assistant.id} type="button" onClick={() => onSelect(assistant)}>
            <span>{assistant.name}</span>
            <small>{assistant.last_error || (assistant.running ? "Running" : "Stopped")}</small>
          </button>
        ))}
      </div>
    </section>
  );
}

function AssistantCard({ assistant, active, busy, onSelect, onAction, onAutostart }) {
  const lifecycle = assistant.running ? "running" : assistant.last_error ? "failed" : "stopped";
  const primary = assistant.running ? "Run Doctor" : "Start";
  return (
    <article className={`assistant-card ${active ? "active" : ""}`} onClick={onSelect}>
      <div className="card-top">
        <div>
          <h3>{assistant.name}</h3>
          <p>{assistant.id}</p>
        </div>
        <span className={`pill ${statusTone(lifecycle)}`}>{lifecycle}</span>
      </div>
      <dl className="facts">
        <div><dt>Provider</dt><dd>{assistant.harness || "configured"}</dd></div>
        <div><dt>Workspace</dt><dd title={assistant.workspace_path}>{compactPath(assistant.workspace_path)}</dd></div>
      </dl>
      <label className="toggle-row" onClick={(event) => event.stopPropagation()}>
        <input type="checkbox" checked={!!assistant.autostart} onChange={(event) => onAutostart(event.target.checked)} />
        <span>Autostart</span>
      </label>
      <div className="card-actions" onClick={(event) => event.stopPropagation()}>
        <button className="primary" type="button" disabled={busy} onClick={() => onAction(assistant.running ? "doctor" : "start")}>
          {primary}
        </button>
        <button type="button" disabled={busy} onClick={() => onAction("restart")}>Restart</button>
        {assistant.running && <button className="danger" type="button" disabled={busy} onClick={() => onAction("stop")}>Stop</button>}
        <button type="button" disabled={busy} onClick={() => onAction("feishu")}>Feishu</button>
        {!assistant.running && <button type="button" disabled={busy} onClick={() => onAction("doctor")}>Doctor</button>}
      </div>
    </article>
  );
}

function AssistantDetail({ assistant, onAction }) {
  return (
    <section className="detail-card">
      <p className="eyebrow">Selected assistant</p>
      <h2>{assistant.name}</h2>
      <p className="detail-id">{assistant.id}</p>
      <dl className="detail-list">
        <div><dt>Status</dt><dd>{assistant.running ? `running pid ${assistant.pid}` : "stopped"}</dd></div>
        <div><dt>Autostart</dt><dd>{assistant.autostart ? "enabled" : "disabled"}</dd></div>
        <div><dt>Workspace</dt><dd>{assistant.workspace_path}</dd></div>
        <div><dt>Configspace</dt><dd>{assistant.configspace_path}</dd></div>
      </dl>
      {assistant.last_error && <Alert tone="bad" title="Last error" text={assistant.last_error} />}
      <div className="panel-actions">
        <button className="primary" type="button" onClick={() => onAction(assistant.running ? "doctor" : "start")}>{assistant.running ? "Run Doctor" : "Start"}</button>
        <button type="button" onClick={() => onAction("restart")}>Restart</button>
        <button type="button" onClick={() => onAction("feishu")}>Setup Feishu</button>
        {assistant.running && <button className="danger" type="button" onClick={() => onAction("stop")}>Stop</button>}
      </div>
    </section>
  );
}

function EmptyState({ onCreate }) {
  return (
    <section className="empty-state">
      <h2>No assistants yet</h2>
      <p>Create a local assistant, connect a harness, then add Feishu when you are ready to test.</p>
      <button className="primary" type="button" onClick={onCreate}>Create Assistant</button>
    </section>
  );
}

function SkeletonList() {
  return (
    <div className="card-grid">
      {[0, 1].map((item) => <div key={item} className="skeleton-card" />)}
    </div>
  );
}

function Sheet({ title, children, onClose }) {
  useEffect(() => {
    function onKey(event) {
      if (event.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);
  return (
    <div className="sheet-backdrop" role="presentation" onMouseDown={onClose}>
      <section className="sheet" role="dialog" aria-modal="true" aria-label={title} onMouseDown={(event) => event.stopPropagation()}>
        <div className="sheet-head">
          <h2>{title}</h2>
          <button className="icon-button" type="button" onClick={onClose} aria-label="Close">×</button>
        </div>
        {children}
      </section>
    </div>
  );
}

function sheetTitle(sheet) {
  if (sheet.type === "create") return "Create assistant";
  if (sheet.type === "feishu") return `Setup Feishu for ${sheet.assistant.name}`;
  if (sheet.type === "doctor") return `Doctor: ${sheet.assistant.name}`;
  if (sheet.type === "confirm") return `${sheet.action} ${sheet.assistant.name}`;
  return "Console action";
}

function CreateAssistantForm({ onDone, onClose, onError }) {
  const [advanced, setAdvanced] = useState(false);
  const [saving, setSaving] = useState(false);
  async function submit(event) {
    event.preventDefault();
    setSaving(true);
    const form = new FormData(event.currentTarget);
    try {
      await api("assistants", {
        method: "POST",
        body: JSON.stringify({
          name: form.get("name"),
          id: form.get("id"),
          root_path: form.get("root_path"),
          workspace_path: form.get("workspace_path"),
          configspace_path: form.get("configspace_path"),
          harness: form.get("harness"),
          autostart: form.get("autostart") === "on",
        }),
      });
      await onDone();
      onClose();
    } catch (err) {
      onError(err.message);
    } finally {
      setSaving(false);
    }
  }
  return (
    <form className="sheet-form" onSubmit={submit}>
      <label>Name<input name="name" required placeholder="local ops assistant" /></label>
      <label>Harness<select name="harness" defaultValue="codex"><option value="codex">Codex</option><option value="claude">Claude</option></select></label>
      <label className="check"><input name="autostart" type="checkbox" defaultChecked /> Start with daemon</label>
      <button className="link-button" type="button" onClick={() => setAdvanced(!advanced)}>{advanced ? "Hide" : "Show"} advanced paths</button>
      {advanced && (
        <div className="advanced">
          <label>ID<input name="id" placeholder="derived from name" /></label>
          <label>Root<input name="root_path" placeholder="default ACPA assistant root" /></label>
          <label>Workspace<input name="workspace_path" placeholder="optional" /></label>
          <label>Configspace<input name="configspace_path" placeholder="optional" /></label>
        </div>
      )}
      <button className="primary wide" type="submit" disabled={saving}>{saving ? "Creating..." : "Create Assistant"}</button>
    </form>
  );
}

function FeishuSetup({ assistant, onDone, onError }) {
  const [mode, setMode] = useState("qr");
  const [status, setStatus] = useState("");
  const [saving, setSaving] = useState(false);

  async function submit(event) {
    event.preventDefault();
    setSaving(true);
    setStatus("");
    const form = new FormData(event.currentTarget);
    const payload = Object.fromEntries(form.entries());
    payload.assistant_id = assistant.id;
    try {
      if (mode === "manual") {
        await api("setup/feishu/manual", { method: "POST", body: JSON.stringify(payload) });
        setStatus("Feishu app saved.");
      } else {
        setStatus("Starting QR registration...");
        const begin = await api("setup/feishu/qr/begin", { method: "POST", body: JSON.stringify(payload) });
        const qrURL = begin.QRURL || begin.qr_url;
        const userCode = begin.UserCode || begin.user_code;
        setStatus([qrURL && `Scan URL: ${qrURL}`, userCode && `User code: ${userCode}`, "Waiting for approval..."].filter(Boolean).join("\n"));
        const channel = await api("setup/feishu/qr/complete", { method: "POST", body: JSON.stringify({ ...payload, begin }) });
        setStatus(`Feishu channel saved: ${channel.id}`);
      }
      await onDone();
    } catch (err) {
      onError(err.message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <form className="sheet-form" onSubmit={submit}>
      <div className="segmented" role="tablist" aria-label="Feishu setup mode">
        <button type="button" className={mode === "qr" ? "active" : ""} onClick={() => setMode("qr")}>QR onboarding</button>
        <button type="button" className={mode === "manual" ? "active" : ""} onClick={() => setMode("manual")}>Existing app</button>
      </div>
      <label>Channel ID<input name="channel_id" defaultValue="feishu-main" /></label>
      <label>Domain<select name="domain" defaultValue="feishu"><option value="feishu">Feishu</option><option value="lark">Lark</option></select></label>
      {mode === "manual" && (
        <>
          <label>App ID<input name="app_id" required /></label>
          <label>App Secret<input name="app_secret" required type="password" /></label>
        </>
      )}
      <button className="primary wide" type="submit" disabled={saving}>{saving ? "Working..." : mode === "qr" ? "Start QR Setup" : "Save Feishu App"}</button>
      {status && <pre className="progress-box">{status}</pre>}
    </form>
  );
}

function ConfirmAction({ assistant, action, busy, onCancel, onConfirm }) {
  return (
    <div className="confirm-box">
      <p>This will {action} <strong>{assistant.name}</strong>. Active connector sessions may be interrupted while the worker changes state.</p>
      <div className="sheet-actions">
        <button type="button" onClick={onCancel}>Cancel</button>
        <button className={action === "stop" ? "danger" : "primary"} type="button" disabled={busy} onClick={onConfirm}>
          {busy ? "Working..." : action[0].toUpperCase() + action.slice(1)}
        </button>
      </div>
    </div>
  );
}

function DoctorPanel({ assistant }) {
  const [loading, setLoading] = useState(true);
  const [report, setReport] = useState(null);
  const [error, setError] = useState("");
  useEffect(() => {
    setLoading(true);
    api(`assistants/${assistant.id}/doctor`)
      .then((data) => setReport(data))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [assistant.id]);
  if (loading) return <div className="panel-empty">Running local checks...</div>;
  if (error) return <Alert tone="bad" title="Doctor failed" text={error} />;
  const checks = report?.checks || [];
  return (
    <div className="doctor-panel">
      <Alert tone={statusTone(report?.severity)} title={`Doctor ${String(report?.severity || "unknown").toUpperCase()}`} text={`${checks.length} checks completed`} />
      <div className="check-list">
        {checks.map((check) => (
          <details key={check.id} open={check.severity !== "pass"}>
            <summary><span className={`pill ${statusTone(check.severity)}`}>{check.severity}</span>{check.title}</summary>
            <p>{check.message}</p>
            {check.recommendation && <small>{check.recommendation}</small>}
          </details>
        ))}
      </div>
    </div>
  );
}

createRoot(document.getElementById("root")).render(<App />);
