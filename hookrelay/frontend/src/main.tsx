import React, { useState, useEffect, FormEvent } from "react";
import { createRoot } from "react-dom/client";
import "./style.css";
type Endpoint = { id: string; name: string; url: string; secret?: string };
type Delivery = {
  id: string;
  event_id: string;
  endpoint_name: string;
  event_type: string;
  status: string;
  attempt_count: number;
  run: number;
  last_error: string;
  created_at: string;
  next_attempt_at: string;
};
type Attempt = {
  id: number;
  run: number;
  number: number;
  status_code: number;
  duration_ms: number;
  error: string;
  response_excerpt: string;
  created_at: string;
};
const date = (value: string) =>
  new Date(value).toLocaleString([], {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
function App() {
  const [token, setToken] = useState(""),
    [inputToken, setInputToken] = useState(""),
    [endpoints, setEndpoints] = useState<Endpoint[]>([]),
    [deliveries, setDeliveries] = useState<Delivery[]>([]);
  const [tab, setTab] = useState("Deliveries"),
    [filter, setFilter] = useState("all"),
    [error, setError] = useState(""),
    [notice, setNotice] = useState(""),
    [busy, setBusy] = useState(false);
  const [name, setName] = useState(""),
    [url, setUrl] = useState("http://receiver:8091/webhook"),
    [secret, setSecret] = useState(""),
    [revealed, setRevealed] = useState("");
  const [endpointId, setEndpointId] = useState(""),
    [type, setType] = useState("booking.confirmed"),
    [payload, setPayload] = useState(
      '{\n  "booking_id": 42,\n  "resource": "Camera kit"\n}',
    ),
    [key, setKey] = useState<string>(crypto.randomUUID());
  const [selected, setSelected] = useState<Delivery | null>(null),
    [attempts, setAttempts] = useState<Attempt[]>([]);
  async function api(
    path: string,
    method = "GET",
    body?: unknown,
    key?: string,
    auth = token,
  ) {
    const response = await fetch("/api" + path, {
      method,
      headers: {
        Authorization: "Bearer " + auth,
        ...(body ? { "Content-Type": "application/json" } : {}),
        ...(key ? { "Idempotency-Key": key } : {}),
      },
      body: body ? JSON.stringify(body) : undefined,
    });
    const data = await response.json().catch(() => ({}));
    if (!response.ok)
      throw new Error(data.error || `Request failed (${response.status})`);
    return data;
  }
  async function load(auth = token) {
    const [e, d] = await Promise.all([
      api("/endpoints", "GET", undefined, undefined, auth),
      api("/deliveries", "GET", undefined, undefined, auth),
    ]);
    setEndpoints(e);
    setDeliveries(d);
    setEndpointId((previous) => previous || e[0]?.id || "");
  }
  useEffect(() => {
    if (!token) return;
    const timer = setInterval(() => {
      load().catch((e) => setError(e.message));
    }, 2500);
    return () => clearInterval(timer);
  }, [token]);
  async function connect(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await load(inputToken);
      setToken(inputToken);
      setInputToken("");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function mutate(work: () => Promise<unknown>, message: string) {
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const result = await work();
      await load();
      setNotice(typeof result === "string" ? result : message);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function inspect(d: Delivery) {
    setError("");
    try {
      const a = await api(`/deliveries/${d.id}/attempts`);
      setAttempts(a);
      setSelected(d);
    } catch (e) {
      setError((e as Error).message);
    }
  }
  const notices = (
    <>
      {error && (
        <div className="alert error" role="alert">
          {error}
        </div>
      )}
      {notice && (
        <div className="alert" role="status">
          {notice}
        </div>
      )}
    </>
  );
  if (!token)
    return (
      <main className="login">
        <section className="intro">
          <span className="eyebrow">EVENT INFRASTRUCTURE / 02</span>
          <h1>
            Every event.
            <br />
            Accounted for.
          </h1>
          <p>
            Durable delivery, visible retries, and a clear path back from
            failure.
          </p>
          <div className="flow">
            <span>Enqueue</span>
            <span>→</span>
            <span>Deliver</span>
            <span>→</span>
            <span>Recover</span>
          </div>
        </section>
        <section className="login-panel">
          <div className="brand">⇢ HookRelay</div>
          <h2>Connect your workspace</h2>
          <p className="muted">
            Use the API token configured for your local server.
          </p>
          {notices}
          <form onSubmit={connect}>
            <label>
              API token
              <input
                type="password"
                required
                minLength={24}
                value={inputToken}
                onChange={(e) => setInputToken(e.target.value)}
                placeholder="Enter API_TOKEN"
              />
            </label>
            <button disabled={busy}>
              {busy ? "Connecting…" : "Open delivery console →"}
            </button>
          </form>
          <p className="hint">
            The token stays in memory and is cleared when you close or reload
            this page.
          </p>
        </section>
      </main>
    );
  return (
    <div className="shell">
      <aside>
        <div className="brand">⇢ HookRelay</div>
        <p className="eyebrow">DELIVERY CONSOLE</p>
        <nav>
          {["Deliveries", "Endpoints", "Send event"].map((t) => (
            <button
              className={t === tab ? "active" : ""}
              key={t}
              onClick={() => {
                setTab(t);
                setNotice("");
                setError("");
              }}
            >
              {t}
              <span>↗</span>
            </button>
          ))}
        </nav>
        <div className="account">
          <span className="live">●</span> Workers poll continuously
          <small>At-least-once delivery</small>
          <button
            className="ghost"
            onClick={() => {
              setToken("");
              setRevealed("");
              setSelected(null);
            }}
          >
            Disconnect
          </button>
        </div>
      </aside>
      <main className="workspace">
        <header>
          <div>
            <span className="eyebrow">
              RELIABLE EVENTS / VISIBLE OPERATIONS
            </span>
            <h1>{tab}</h1>
          </div>
          <span className="refresh">↻ Refreshes every 2.5s</span>
        </header>
        {notices}
        <section className="stats">
          <article>
            <span>Latest deliveries</span>
            <strong>{deliveries.length}</strong>
          </article>
          <article>
            <span>Delivered</span>
            <strong className="green">
              {deliveries.filter((d) => d.status === "delivered").length}
            </strong>
          </article>
          <article>
            <span>In flight</span>
            <strong className="purple">
              {
                deliveries.filter((d) =>
                  ["pending", "processing", "retrying"].includes(d.status),
                ).length
              }
            </strong>
          </article>
          <article>
            <span>Dead letters</span>
            <strong className="orange">
              {deliveries.filter((d) => d.status === "dead").length}
            </strong>
          </article>
        </section>
        {tab === "Deliveries" && (
          <section className="panel">
            <div className="section-top">
              <div>
                <h2>Event delivery log</h2>
                <p className="muted">
                  Latest 200 jobs. Inspect attempts or replay a terminal
                  delivery.
                </p>
              </div>
              <select
                aria-label="Filter status"
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
              >
                {[
                  "all",
                  "pending",
                  "processing",
                  "retrying",
                  "delivered",
                  "dead",
                ].map((s) => (
                  <option key={s}>{s}</option>
                ))}
              </select>
            </div>
            {!deliveries.length ? (
              <div className="empty">
                <h3>Your first event starts here</h3>
                <p>
                  Register an endpoint, then send an event to see its full
                  delivery lifecycle.
                </p>
                <button onClick={() => setTab("Endpoints")}>
                  Add an endpoint →
                </button>
              </div>
            ) : (
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>Event / destination</th>
                      <th>Status</th>
                      <th>Attempts</th>
                      <th>Created</th>
                      <th>Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {deliveries
                      .filter((d) => filter === "all" || d.status === filter)
                      .map((d) => (
                        <tr key={d.id}>
                          <td>
                            <strong>{d.event_type}</strong>
                            <small>
                              {d.endpoint_name} · {d.id.slice(0, 8)}
                            </small>
                            {d.last_error && (
                              <small className="orange">{d.last_error}</small>
                            )}
                          </td>
                          <td>
                            <span className={"badge " + d.status}>
                              {d.status}
                            </span>
                          </td>
                          <td>
                            {d.attempt_count}
                            <small>run {d.run}</small>
                          </td>
                          <td>{date(d.created_at)}</td>
                          <td>
                            <div className="row-actions">
                              <button
                                className="secondary"
                                onClick={() => void inspect(d)}
                              >
                                Inspect
                              </button>
                              {["delivered", "dead"].includes(d.status) && (
                                <button
                                  disabled={busy}
                                  className="secondary"
                                  onClick={() =>
                                    void mutate(
                                      () =>
                                        api(
                                          `/deliveries/${d.id}/replay`,
                                          "POST",
                                        ),
                                      "Delivery queued for replay. Its event ID remains unchanged.",
                                    )
                                  }
                                >
                                  Replay
                                </button>
                              )}
                            </div>
                          </td>
                        </tr>
                      ))}
                  </tbody>
                </table>
              </div>
            )}
          </section>
        )}
        {tab === "Endpoints" && (
          <div className="endpoint-layout">
            <section className="panel">
              <h2>Registered destinations</h2>
              {endpoints.length ? (
                endpoints.map((e) => (
                  <article className="list-item" key={e.id}>
                    <h3>{e.name}</h3>
                    <code>{e.url}</code>
                    <small>ID: {e.id}</small>
                  </article>
                ))
              ) : (
                <p className="empty">No endpoints registered yet.</p>
              )}
            </section>
            <section className="panel">
              <span className="eyebrow">NEW DESTINATION</span>
              <h2>Register an endpoint</h2>
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  void mutate(async () => {
                    const result = await api("/endpoints", "POST", {
                      name,
                      url,
                      secret,
                    });
                    setRevealed(result.secret);
                    setSecret("");
                    setName("");
                    setEndpointId(result.id);
                  }, "Endpoint registered. Save its signing secret before leaving this page.");
                }}
              >
                <label>
                  Display name
                  <input
                    required
                    maxLength={120}
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    placeholder="Campus booking receiver"
                  />
                </label>
                <label>
                  Receiver URL
                  <input
                    required
                    type="url"
                    maxLength={2048}
                    value={url}
                    onChange={(e) => setUrl(e.target.value)}
                  />
                </label>
                <label>
                  Signing secret (optional)
                  <input
                    type="password"
                    minLength={32}
                    maxLength={256}
                    value={secret}
                    onChange={(e) => setSecret(e.target.value)}
                    placeholder="Leave blank to generate a secret"
                  />
                </label>
                <p className="hint">
                  For the included receiver demo, enter DEMO_WEBHOOK_SECRET from
                  .env. HTTPS and public destinations are required outside local
                  mode.
                </p>
                <button disabled={busy}>Register destination →</button>
              </form>
              {revealed && (
                <div className="secret-box">
                  <strong>Signing secret · shown only after creation</strong>
                  <code>{revealed}</code>
                  <button className="ghost" onClick={() => setRevealed("")}>
                    Dismiss
                  </button>
                </div>
              )}
            </section>
          </div>
        )}
        {tab === "Send event" && (
          <section className="panel narrow">
            <span className="eyebrow">DURABLE INGESTION</span>
            <h2>Send a test event</h2>
            <p className="muted">
              Reuse the same idempotency key and request to verify duplicate
              ingestion is prevented.
            </p>
            <form
              onSubmit={(e) => {
                e.preventDefault();
                void mutate(async () => {
                  let value;
                  try {
                    value = JSON.parse(payload);
                  } catch {
                    throw new Error("Payload must be valid JSON.");
                  }
                  const result = await api(
                    "/events",
                    "POST",
                    { endpoint_id: endpointId, type, payload: value },
                    key,
                  );
                  return result.duplicate
                    ? "Existing delivery returned; no duplicate job created."
                    : `Queued delivery ${result.delivery_id}`;
                }, "Event accepted. Check Deliveries for its status.");
              }}
            >
              <label>
                Destination
                <select
                  required
                  value={endpointId}
                  onChange={(e) => setEndpointId(e.target.value)}
                >
                  <option value="">Select an endpoint</option>
                  {endpoints.map((e) => (
                    <option value={e.id} key={e.id}>
                      {e.name}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                Event type
                <input
                  required
                  maxLength={120}
                  value={type}
                  onChange={(e) => setType(e.target.value)}
                />
              </label>
              <label>
                Idempotency key
                <div className="input-row">
                  <input
                    required
                    maxLength={200}
                    value={key}
                    onChange={(e) => setKey(e.target.value)}
                  />
                  <button
                    type="button"
                    className="secondary"
                    onClick={() => setKey(crypto.randomUUID())}
                  >
                    New key
                  </button>
                </div>
              </label>
              <label>
                JSON payload
                <textarea
                  className="code-editor"
                  required
                  value={payload}
                  onChange={(e) => setPayload(e.target.value)}
                />
              </label>
              <button disabled={busy || !endpoints.length}>
                Enqueue event →
              </button>
            </form>
          </section>
        )}
        <footer>
          HookRelay · Persist first. Deliver reliably. Recover visibly.
        </footer>
      </main>
      {selected && (
        <div className="overlay" onClick={() => setSelected(null)}>
          <section
            className="modal"
            role="dialog"
            aria-modal="true"
            aria-labelledby="attempt-title"
            onClick={(e) => e.stopPropagation()}
          >
            <button
              className="close ghost"
              aria-label="Close attempts"
              onClick={() => setSelected(null)}
            >
              ✕
            </button>
            <span className="eyebrow">DELIVERY DETAIL</span>
            <h2 id="attempt-title">{selected.event_type}</h2>
            <code>{selected.id}</code>
            <p className="muted">
              A worker crash before completion can leave an unrecorded attempt.
              Lease recovery still retries the job.
            </p>
            {!attempts.length ? (
              <p className="empty">No completed attempts recorded yet.</p>
            ) : (
              attempts.map((a) => (
                <article className="attempt" key={a.id}>
                  <div className="section-top">
                    <strong>
                      Run {a.run} / attempt {a.number}
                    </strong>
                    <span
                      className={
                        "badge " +
                        (a.status_code >= 200 && a.status_code < 300
                          ? "delivered"
                          : "dead")
                      }
                    >
                      {a.status_code || "Network error"} · {a.duration_ms}ms
                    </span>
                  </div>
                  <small>{date(a.created_at)}</small>
                  {a.error && <p className="orange">{a.error}</p>}
                  <pre>{a.response_excerpt || "No response body"}</pre>
                </article>
              ))
            )}
            <button
              className="secondary"
              onClick={() => void inspect(selected)}
            >
              Refresh attempts
            </button>
          </section>
        </div>
      )}
    </div>
  );
}
createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
