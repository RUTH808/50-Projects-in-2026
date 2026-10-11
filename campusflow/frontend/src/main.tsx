import React, { useEffect, useState, FormEvent } from "react";
import { createRoot } from "react-dom/client";
import QRCode from "qrcode";
import "./style.css";

type User = { id: number; name: string; email: string; role: string };
type Equipment = {
  id: number;
  name: string;
  category: string;
  location: string;
  description: string;
  maintenance: boolean;
};
type Booking = {
  id: number;
  equipment_id: number;
  equipment_name: string;
  user_name: string;
  starts_at: string;
  ends_at: string;
  purpose: string;
  status: string;
};
type Ticket = {
  id: number;
  equipment_name: string;
  description: string;
  status: string;
};
type Audit = {
  id: number;
  action: string;
  actor_name: string;
  created_at: string;
};
type Report = {
  id: number;
  name: string;
  waiting: number;
  approved_bookings: number;
  reserved_hours: number;
};
const date = (s: string) =>
  new Date(s).toLocaleString([], {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
const local = (d: Date) =>
  new Date(d.getTime() - d.getTimezoneOffset() * 60000)
    .toISOString()
    .slice(0, 16);

function App() {
  const [auth, setAuth] = useState(""),
    [user, setUser] = useState<User | null>(null),
    [csrf, setCsrf] = useState("");
  const [email, setEmail] = useState("student@campusflow.local"),
    [password, setPassword] = useState("");
  const [equipment, setEquipment] = useState<Equipment[]>([]),
    [bookings, setBookings] = useState<Booking[]>([]),
    [tickets, setTickets] = useState<Ticket[]>([]),
    [audit, setAudit] = useState<Audit[]>([]),
    [reports, setReports] = useState<Report[]>([]);
  const [tab, setTab] = useState("Equipment"),
    [search, setSearch] = useState(""),
    [selected, setSelected] = useState<Equipment | null>(null),
    [qr, setQr] = useState("");
  const [start, setStart] = useState(local(new Date(Date.now() + 3600000))),
    [end, setEnd] = useState(local(new Date(Date.now() + 7200000))),
    [purpose, setPurpose] = useState("");
  const [notice, setNotice] = useState(""),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  const [reportItem, setReportItem] = useState<Equipment | null>(null),
    [issue, setIssue] = useState("");
  const [newName, setNewName] = useState(""),
    [category, setCategory] = useState("Electronics"),
    [location, setLocation] = useState(""),
    [description, setDescription] = useState("");
  const staff = user && user.role !== "STUDENT";
  async function api(
    path: string,
    method = "GET",
    body?: unknown,
    credentials = auth,
  ) {
    // Fetch immediately before mutations: authentication rotates the CSRF cookie.
    let currentToken = csrf;
    if (method !== "GET") {
      const tokenResponse = await fetch("/api/csrf", {
        headers: { Authorization: "Basic " + credentials },
      });
      if (!tokenResponse.ok)
        throw new Error("Unable to refresh request security token");
      currentToken = (await tokenResponse.json()).token;
      setCsrf(currentToken);
    }
    const response = await fetch("/api" + path, {
      method,
      headers: {
        Authorization: "Basic " + credentials,
        ...(body ? { "Content-Type": "application/json" } : {}),
        ...(method !== "GET" ? { "X-XSRF-TOKEN": currentToken } : {}),
      },
      body: body ? JSON.stringify(body) : undefined,
    });
    if (!response.ok) {
      const problem = await response.json().catch(() => ({}));
      throw new Error(problem.detail || `Request failed (${response.status})`);
    }
    return response.json();
  }
  async function load(credentials = auth, who = user) {
    const [items, reservations] = await Promise.all([
      api("/equipment", "GET", undefined, credentials),
      api("/bookings", "GET", undefined, credentials),
    ]);
    setEquipment(items);
    setBookings(reservations);
    if (who?.role !== "STUDENT") {
      const [t, a, r] = await Promise.all([
        api("/staff/maintenance", "GET", undefined, credentials),
        api("/staff/audit", "GET", undefined, credentials),
        api("/staff/reports", "GET", undefined, credentials),
      ]);
      setTickets(t);
      setAudit(a);
      setReports(r);
    }
  }
  useEffect(() => {
    fetch("/api/csrf")
      .then((r) => r.json())
      .then((r) => setCsrf(r.token))
      .catch(() =>
        setError("Cannot reach the server. Check that the backend is running."),
      );
  }, []);
  async function login(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const value = btoa(email + ":" + password);
      const who = await api("/me", "GET", undefined, value);
      await load(value, who);
      setAuth(value);
      setUser(who);
      setPassword("");
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
      await work();
      await load();
      setNotice(message);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function select(item: Equipment) {
    setSelected(item);
    setPurpose("");
    const url = new URL(window.location.href);
    url.searchParams.set("equipment", String(item.id));
    setQr(await QRCode.toDataURL(url.toString(), { width: 180, margin: 1 }));
  }
  useEffect(() => {
    if (user) {
      const id = Number(
        new URLSearchParams(window.location.search).get("equipment"),
      );
      const item = equipment.find((e) => e.id === id);
      if (item) void select(item);
    }
  }, [user]);
  const noticeView = (
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
  if (!user)
    return (
      <main className="login">
        <section className="intro">
          <span className="eyebrow">CAMPUS OPERATIONS / 01</span>
          <h1>
            Good equipment.
            <br />
            Better access.
          </h1>
          <p>
            Book shared resources, keep repairs moving, and give everyone a fair
            turn.
          </p>
          <div className="intro-line">
            One campus. One coordinated workspace.
          </div>
        </section>
        <section className="login-panel">
          <div className="brand">◈ CampusFlow</div>
          <h2>Welcome to your campus</h2>
          <p className="muted">Sign in to coordinate your next project.</p>
          {noticeView}
          <form onSubmit={login}>
            <label>
              Email
              <input
                type="email"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </label>
            <label>
              Password
              <input
                type="password"
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </label>
            <button disabled={busy}>
              {busy ? "Signing in…" : "Enter workspace →"}
            </button>
          </form>
          <p className="hint">
            Local demo accounts: student, staff, or admin @campusflow.local. Use
            the password from the README or your DEMO_PASSWORD setting.
          </p>
        </section>
      </main>
    );
  return (
    <div className="shell">
      <aside>
        <div className="brand">◈ CampusFlow</div>
        <p className="eyebrow">YOUR WORKSPACE</p>
        <nav>
          {[
            "Equipment",
            "Bookings",
            ...(staff ? ["Maintenance", "Reports", "Activity"] : []),
            ...(user.role === "ADMIN" ? ["Add equipment"] : []),
          ].map((t) => (
            <button
              key={t}
              className={tab === t ? "active" : ""}
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
          <strong>{user.name}</strong>
          <small>{user.role.toLowerCase()}</small>
          <button
            className="ghost"
            onClick={() => {
              setUser(null);
              setAuth("");
              setSelected(null);
              setTickets([]);
              setAudit([]);
            }}
          >
            Sign out
          </button>
        </div>
      </aside>
      <main className="workspace">
        <header>
          <div>
            <span className="eyebrow">SHARED RESOURCES, COORDINATED</span>
            <h1>{tab}</h1>
          </div>
          <button
            className="secondary"
            disabled={busy}
            onClick={() => void mutate(async () => {}, "Workspace refreshed.")}
          >
            Refresh ↻
          </button>
        </header>
        {noticeView}
        <section className="stats">
          <article>
            <span>Resources</span>
            <strong>{equipment.length}</strong>
          </article>
          <article>
            <span>Awaiting approval</span>
            <strong>
              {bookings.filter((b) => b.status === "PENDING").length}
            </strong>
          </article>
          <article>
            <span>On the waitlist</span>
            <strong>
              {bookings.filter((b) => b.status === "WAITLISTED").length}
            </strong>
          </article>
          <article>
            <span>In maintenance</span>
            <strong>{equipment.filter((e) => e.maintenance).length}</strong>
          </article>
        </section>
        {tab === "Equipment" && (
          <>
            <div className="section-top">
              <div>
                <h2>Find your next resource</h2>
                <p className="muted">
                  Conflicting requests join the waitlist automatically.
                </p>
              </div>
              <input
                aria-label="Search equipment"
                placeholder="Search name, category, location…"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
            </div>
            <div className="grid">
              {equipment
                .filter((e) =>
                  (e.name + e.category + e.location)
                    .toLowerCase()
                    .includes(search.toLowerCase()),
                )
                .map((item) => (
                  <article className="card" key={item.id}>
                    <div className="card-top">
                      <span className="category">{item.category}</span>
                      <span
                        className={
                          "badge " +
                          (item.maintenance ? "WAITLISTED" : "CONFIRMED")
                        }
                      >
                        {item.maintenance ? "Maintenance" : "Serviceable"}
                      </span>
                    </div>
                    <h3>{item.name}</h3>
                    <p>{item.description}</p>
                    <div className="location">⌖ {item.location}</div>
                    <div className="card-actions">
                      <button onClick={() => void select(item)}>
                        Request booking
                      </button>
                      <button
                        className="ghost"
                        onClick={() => {
                          setReportItem(item);
                          setIssue("");
                        }}
                      >
                        Report issue
                      </button>
                    </div>
                  </article>
                ))}
            </div>
          </>
        )}
        {tab === "Bookings" && (
          <section className="panel">
            <h2>{staff ? "Campus reservations" : "Your reservations"}</h2>
            <p className="muted">
              Pending requests hold the interval until approved or rejected.
              Staff handle handovers.
            </p>
            {!bookings.length ? (
              <p className="empty">
                No reservations yet. Choose a resource to get started.
              </p>
            ) : (
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>Resource / purpose</th>
                      {staff && <th>Borrower</th>}
                      <th>Time window</th>
                      <th>Status</th>
                      <th>Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {bookings.map((b) => (
                      <tr key={b.id}>
                        <td>
                          <strong>{b.equipment_name}</strong>
                          <small>{b.purpose}</small>
                        </td>
                        {staff && <td>{b.user_name}</td>}
                        <td>
                          {date(b.starts_at)}
                          <small>to {date(b.ends_at)}</small>
                        </td>
                        <td>
                          <span className={"badge " + b.status}>
                            {b.status.replace("_", " ")}
                          </span>
                        </td>
                        <td>
                          <div className="row-actions">
                            {[
                              ...(staff && b.status === "PENDING"
                                ? ["approve", "reject"]
                                : []),
                              ...(staff && b.status === "CONFIRMED"
                                ? ["checkout"]
                                : []),
                              ...(staff && b.status === "CHECKED_OUT"
                                ? ["return"]
                                : []),
                              ...([
                                "PENDING",
                                "CONFIRMED",
                                "WAITLISTED",
                              ].includes(b.status)
                                ? ["cancel"]
                                : []),
                            ].map((action) => (
                              <button
                                className="secondary"
                                disabled={busy}
                                key={action}
                                onClick={() =>
                                  void mutate(
                                    () =>
                                      api(
                                        `/bookings/${b.id}/${action}`,
                                        "POST",
                                      ),
                                    `Booking #${b.id} updated.`,
                                  )
                                }
                              >
                                {action}
                              </button>
                            ))}
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
        {tab === "Maintenance" && (
          <section className="panel">
            <h2>Repair queue</h2>
            {!tickets.length ? (
              <p className="empty">No reported issues.</p>
            ) : (
              tickets.map((t) => (
                <article className="list-item" key={t.id}>
                  <div>
                    <h3>{t.equipment_name}</h3>
                    <p>{t.description}</p>
                    <span className="badge">{t.status}</span>
                  </div>
                  {t.status === "OPEN" && (
                    <button
                      disabled={busy}
                      onClick={() =>
                        void mutate(
                          () =>
                            api(`/staff/maintenance/${t.id}/resolve`, "POST"),
                          "Issue resolved. Eligible waitlisted requests promoted.",
                        )
                      }
                    >
                      Resolve issue
                    </button>
                  )}
                </article>
              ))
            )}
          </section>
        )}
        {tab === "Reports" && (
          <section className="panel">
            <h2>Last 30 days</h2>
            <p className="muted">
              Reserved hours are scheduled durations of approved bookings, not
              measured usage.
            </p>
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Resource</th>
                    <th>Approved bookings</th>
                    <th>Reserved hours</th>
                    <th>Waiting</th>
                  </tr>
                </thead>
                <tbody>
                  {reports.map((r) => (
                    <tr key={r.id}>
                      <td>{r.name}</td>
                      <td>{r.approved_bookings}</td>
                      <td>{Number(r.reserved_hours).toFixed(1)}</td>
                      <td>{r.waiting}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </section>
        )}
        {tab === "Activity" && (
          <section className="panel">
            <h2>Audit trail</h2>
            {audit.length ? (
              audit.map((a) => (
                <div className="list-item" key={a.id}>
                  <div>
                    <strong>
                      {a.action.replaceAll("_", " ").toLowerCase()}
                    </strong>
                    <small>{a.actor_name}</small>
                  </div>
                  <time>{date(a.created_at)}</time>
                </div>
              ))
            ) : (
              <p className="empty">
                Activity appears as your campus uses the workspace.
              </p>
            )}
          </section>
        )}
        {tab === "Add equipment" && (
          <section className="panel narrow">
            <h2>Expand the shared inventory</h2>
            <form
              onSubmit={(e) => {
                e.preventDefault();
                void mutate(
                  () =>
                    api("/admin/equipment", "POST", {
                      name: newName,
                      category,
                      location,
                      description,
                    }),
                  "Equipment added.",
                );
              }}
            >
              <label>
                Name
                <input
                  required
                  maxLength={120}
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                />
              </label>
              <label>
                Category
                <input
                  required
                  maxLength={80}
                  value={category}
                  onChange={(e) => setCategory(e.target.value)}
                />
              </label>
              <label>
                Location
                <input
                  required
                  maxLength={200}
                  value={location}
                  onChange={(e) => setLocation(e.target.value)}
                />
              </label>
              <label>
                Description
                <textarea
                  maxLength={1000}
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                />
              </label>
              <button disabled={busy}>Add resource</button>
            </form>
          </section>
        )}
        <footer>CampusFlow · Fair access. Clear accountability.</footer>
      </main>
      {selected && (
        <div className="overlay" onClick={() => setSelected(null)}>
          <section
            className="modal"
            role="dialog"
            aria-modal="true"
            aria-labelledby="booking-title"
            onClick={(e) => e.stopPropagation()}
          >
            <button
              className="close ghost"
              aria-label="Close booking"
              onClick={() => setSelected(null)}
            >
              ✕
            </button>
            <span className="eyebrow">NEW REQUEST</span>
            <h2 id="booking-title">{selected.name}</h2>
            <p className="muted">{selected.location}</p>
            {noticeView}
            <form
              onSubmit={(e) => {
                e.preventDefault();
                void mutate(async () => {
                  const result = await api("/bookings", "POST", {
                    equipmentId: selected.id,
                    startsAt: new Date(start).toISOString(),
                    endsAt: new Date(end).toISOString(),
                    purpose,
                  });
                  setSelected(null);
                  setTab("Bookings");
                  return result;
                }, "Request saved. Check its status in your bookings.");
              }}
            >
              <label>
                From
                <input
                  required
                  type="datetime-local"
                  value={start}
                  onChange={(e) => setStart(e.target.value)}
                />
              </label>
              <label>
                Until
                <input
                  required
                  type="datetime-local"
                  value={end}
                  onChange={(e) => setEnd(e.target.value)}
                />
              </label>
              <label>
                Purpose
                <textarea
                  required
                  maxLength={500}
                  value={purpose}
                  onChange={(e) => setPurpose(e.target.value)}
                  placeholder="What are you working on?"
                />
              </label>
              <button disabled={busy}>Submit request →</button>
            </form>
            <div className="qr">
              <img src={qr} alt={`Resource QR code for ${selected.name}`} />
              <p>
                Resource tag
                <br />
                <small>
                  Scan to open this resource. Staff complete checkout in
                  Bookings.
                </small>
              </p>
            </div>
          </section>
        </div>
      )}
      {reportItem && (
        <div className="overlay">
          <section
            className="modal"
            role="dialog"
            aria-modal="true"
            aria-labelledby="issue-title"
          >
            <button className="close ghost" onClick={() => setReportItem(null)}>
              ✕
            </button>
            <h2 id="issue-title">Report: {reportItem.name}</h2>
            <p className="muted">
              Reporting an issue immediately blocks new approvals and checkout.
            </p>
            {noticeView}
            <form
              onSubmit={(e) => {
                e.preventDefault();
                void mutate(async () => {
                  await api("/maintenance", "POST", {
                    equipmentId: reportItem.id,
                    description: issue,
                  });
                  setReportItem(null);
                }, "Issue reported. Equipment placed in maintenance.");
              }}
            >
              <label>
                Describe the problem
                <textarea
                  required
                  maxLength={2000}
                  value={issue}
                  onChange={(e) => setIssue(e.target.value)}
                />
              </label>
              <button disabled={busy}>Report issue</button>
            </form>
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
