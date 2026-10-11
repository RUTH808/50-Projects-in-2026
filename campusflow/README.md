# CampusFlow

**Shared equipment booking, fair waitlists, and maintenance coordination for a campus.**

Campus labs and clubs need more than a spreadsheet of equipment. CampusFlow coordinates who can borrow an item, when it is reserved, whether it is serviceable, and who approved or completed the handover.

## What works

- React/TypeScript dashboard with equipment search, reservation forms, booking status, and generated resource QR tags.
- Student, staff, and administrator roles enforced by Spring Security and backend authorization.
- Staff approval/rejection, checkout during the reserved time window, and return tracking.
- Automatic waitlisting of conflicting reservations and promotion of the oldest eligible request after cancellation, rejection, return, or repair completion.
- Maintenance tickets block new approvals and checkout; all open tickets must be resolved to restore serviceability.
- PostgreSQL transactions, equipment row locks, and a database exclusion constraint prevent overlapping active reservations.
- Audit trail and a 30-day report of approved bookings, scheduled reserved hours, and waitlist counts.
- Flyway schema migration, Docker Compose, health endpoint, automated tests, and GitHub Actions checks.

## Stack

| Layer | Technology |
|---|---|
| Backend | Java 17, Spring Boot 3.5, Spring Security, Spring JDBC, Bean Validation |
| Frontend | React 19, TypeScript, Vite, qrcode |
| Data | PostgreSQL 16, Flyway, GiST exclusion constraint |
| Delivery | Docker Compose, Nginx, GitHub Actions, Maven, npm lockfile |
| Tests | JUnit 5, MockMvc, PostgreSQL integration tests |

## Fastest way to run

**Requirements:** Git; Docker Engine/Desktop with Docker Compose v2; an internet connection for the first image/build download. Java, Maven, Node, and PostgreSQL do not need to be installed separately for this path. Docker Desktop must be running on Windows/macOS. Allow roughly 4 GB of free RAM for the combined project stack.

```bash
git clone https://github.com/RUTH808/50-Projects-in-2026.git
cd 50-Projects-in-2026/campusflow
cp .env.example .env
docker compose up --build -d
docker compose logs -f backend
```

On PowerShell, replace `cp .env.example .env` with `Copy-Item .env.example .env`. The other Docker commands are the same. Wait for the backend startup message, then open **http://localhost:3000**.

| Account | Email | Local demo password |
|---|---|---|
| Student | student@campusflow.local | CampusFlow-local-2026! |
| Staff | staff@campusflow.local | CampusFlow-local-2026! |
| Administrator | admin@campusflow.local | CampusFlow-local-2026! |

Change `DEMO_PASSWORD` in `.env` before first startup if desired. Demo users are created only when missing; changing the variable after seeding does not change existing passwords. There is no registration or account-management UI in this version.

- Web: http://localhost:3000
- API: http://localhost:8080/api
- Health: http://localhost:8080/actuator/health
- Database: localhost:5432

Stop with `docker compose down`; data remains in the named volume. `docker compose down -v` **deletes the local database**, including bookings and accounts.

## Try the complete workflow

1. Sign in as student, choose equipment, and request a future interval with a purpose. It becomes `PENDING` if available.
2. Sign in as staff in another browser/private window and request the same interval. It becomes `WAITLISTED`.
3. As staff, approve the first booking. As its student owner, cancel it. The waitlisted request becomes `PENDING` and requires staff approval.
4. Report an issue on the equipment. Staff see a maintenance ticket, and checkout/approval is blocked. Resolve every open ticket to restore serviceability.
5. For checkout, create a reservation starting one or two minutes in the future, approve it, and wait for its start. Staff can then check out and return the item from Bookings.
6. Open a resource booking dialog to generate its QR tag. Scanning opens the resource page after login; scanning itself does not perform a handover.
7. As staff, inspect Reports and Activity. As admin, add a new resource.

Optional automated HTTP demonstration, with Python 3.10+ and no extra dependencies:

```bash
python3 examples/demo.py
```

Use `python` on Windows. If you changed the demo password, export `DEMO_PASSWORD` for the script. The script creates a new demo resource and verifies waitlisting, approval, cancellation, promotion, and repair recovery.

## Run without application containers

**Requirements:** JDK 17+, Maven 3.9+, Node.js 22+, npm, and PostgreSQL 16. The database user must be allowed to create the `btree_gist` extension, or a database administrator must preinstall it.

You can use the included database container while running Java and React locally:

```bash
# From campusflow/
docker compose up -d db

# Terminal 1
cd backend
mvn spring-boot:run

# Terminal 2, from campusflow/
cd frontend
npm ci
npm run dev
```

Open **http://localhost:5173**. Vite proxies API calls to port 8080. If database credentials differ from the defaults, set `DB_URL`, `DB_USER`, and `DB_PASSWORD` in the backend shell. `.env` is read by Docker Compose; Maven does not automatically load it.

For your own PostgreSQL installation, create a database/user named `campusflow`, set the password, then point `DB_URL=jdbc:postgresql://localhost:5432/campusflow` at it. Use a separate disposable database for tests.

## Tests

```bash
# Pure validation test; PostgreSQL tests are skipped unless explicitly enabled.
cd backend
mvn test

# Full suite: the named database must already exist and be disposable.
TEST_DATABASE_URL=jdbc:postgresql://localhost:5432/campusflow_test \
TEST_DB_USER=campusflow TEST_DB_PASSWORD=campusflow-local mvn verify

# Frontend type check and production build, from campusflow/frontend
npm ci
npm run build
```

PowerShell equivalent: `$env:TEST_DATABASE_URL='jdbc:postgresql://localhost:5432/campusflow_test'`, set `TEST_DB_USER`/`TEST_DB_PASSWORD`, then run `mvn verify`. Integration tests truncate project tables; **never point them at a database you want to keep**.

The full suite checks 12 competing reservation requests, database-level overlap rejection, adjacent intervals, ownership, invalid transitions, CSRF, role restrictions, waitlist promotion, and multiple repair tickets. The repository-level `CampusFlow checks` workflow provisions real PostgreSQL and builds the frontend and containers.

## API outline

Requests use HTTP Basic authentication. Mutating requests also need a CSRF token and cookie: fetch `GET /api/csrf` immediately before each mutation, retain its cookie, and send the response token as `X-XSRF-TOKEN`. Authentication can rotate the cookie, so do not reuse an old token across requests. The frontend and demo script handle this automatically. Use TLS if deploying beyond localhost.

| Method | Path | Purpose |
|---|---|---|
| GET | /api/me | Current user and role |
| GET | /api/equipment | Shared inventory |
| GET / POST | /api/bookings | List accessible bookings / request a reservation |
| POST | /api/bookings/{id}/{action} | approve, reject, cancel, checkout, return |
| POST | /api/maintenance | Report an issue |
| GET | /api/staff/maintenance | Repair queue |
| POST | /api/staff/maintenance/{id}/resolve | Resolve one ticket |
| GET | /api/staff/reports | 30-day scheduled reservation report |
| GET | /api/staff/audit | Latest 200 audit events |
| POST | /api/admin/equipment | Add inventory |

Errors use HTTP status codes and problem details. Listing limits are 1,000 resources, 500 bookings/tickets, and 200 audit entries. Pagination is future work.

## Design and tradeoffs

- Each booking or maintenance mutation first locks the relevant equipment row. The transaction re-reads state before deciding a transition. This keeps decisions consistent across concurrent requests and application instances.
- A PostgreSQL GiST exclusion constraint provides a second guarantee: `PENDING`, `CONFIRMED`, and `CHECKED_OUT` intervals cannot overlap for the same resource. Intervals are half-open, so adjacent bookings are allowed.
- Pending requests reserve their interval. Waitlist promotion is oldest **eligible** first; an older incompatible interval does not block a later compatible request. Expired waitlist entries are cancelled when promotion runs.
- Reporting an issue preserves existing reservations but blocks approval/checkout. Resolving only one of several tickets does not make the resource available.
- Checkout also checks that no previous borrower still holds the item, even if their scheduled interval ended.
- This is a modular monolith: separate security, controller, service, and data boundaries, without artificial microservices.

## Deployment boundaries and next improvements

The included Compose setup binds ports to localhost and seeds known demo accounts. It is a portfolio/local-demo deployment, not a university production rollout. Before a public pilot, disable demo seeding, provision real accounts/SSO, terminate TLS, add request throttling and pagination, introduce approval-expiry reminders, and establish backups. Account provisioning currently requires administration outside the UI. Audit rows are append-only through the application, but are not tamper-proof against a database administrator. Reports measure scheduled hours, not actual elapsed borrowing time.

If a port is busy, change the left side of its mapping in `compose.yaml`. If the backend cannot authenticate to an existing volume after a password change, restore the previous password or deliberately reset only your disposable demo volume. Inspect `docker compose logs backend` for migration/connection errors.

## Resume description

See [RESUME.md](RESUME.md) for concise, feature-backed bullets and interview talking points. No production adoption or performance figures are claimed.
