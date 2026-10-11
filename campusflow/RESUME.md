# CampusFlow — Campus Equipment Booking & Maintenance Platform

**Technologies:** Java, Spring Boot, Spring Security, React, TypeScript, PostgreSQL, SQL, Flyway, Docker, GitHub Actions, JUnit, REST APIs.

Choose two or three bullets based on the role and the work you can explain:

- Developed a full-stack campus resource platform using Java/Spring Boot, React, TypeScript, and PostgreSQL, with role-based access control, staff approvals, equipment checkout, and maintenance workflows.
- Implemented transactional reservation handling with PostgreSQL row locks and GiST exclusion constraints to prevent overlapping active bookings, plus automatic promotion of eligible waitlisted requests.
- Added CSRF protection, BCrypt password hashing, audit logging, SQL reservation reports, Docker Compose deployment, and CI checks covering reservation concurrency and authorization.

**One-line version:** Full-stack equipment booking and maintenance platform with transactional conflict prevention, role-based approvals, fair waitlists, and containerized deployment.

## Be ready to explain

1. Why both application row locks and a database exclusion constraint are useful.
2. Why pending requests occupy an interval and how half-open intervals permit adjacent bookings.
3. Why waitlist promotion selects the oldest eligible request rather than blindly promoting the first row.
4. How multiple maintenance tickets and overdue borrowers affect availability.
5. Why scheduled reserved hours are different from actual utilization.

Use these bullets after running and understanding the project. Add measured throughput, latency, or adoption only after collecting reproducible evidence; include the test hardware, load, and method with any benchmark.
