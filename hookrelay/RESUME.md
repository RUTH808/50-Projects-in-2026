# HookRelay — Reliable Webhook Delivery & Replay Service

**Technologies:** Go, PostgreSQL, SQL, React, TypeScript, Python, Docker, GitHub Actions, REST APIs, HMAC-SHA256.

Choose two or three bullets based on the role and the work you can explain:

- Developed a Go webhook delivery service with transactional event ingestion, idempotency keys, and a PostgreSQL-backed durable job queue exposed through authenticated REST APIs.
- Implemented concurrent workers using SKIP LOCKED, expiring leases, and lease-token fencing, with exponential retries, dead-letter handling, and controlled replay for failed deliveries.
- Built a React/TypeScript operations console and Python receiver demo with HMAC-SHA256 verification and SQLite deduplication; added Docker Compose and CI checks for crash recovery, duplicate ingestion, and replay behavior.

**One-line version:** Durable webhook infrastructure with concurrent Go workers, signed delivery, retry/replay workflows, and a real-time polling operations console.

## Be ready to explain

1. Why event ingestion and job creation happen in one database transaction.
2. What SKIP LOCKED changes when multiple workers compete.
3. Why leases need ownership tokens, and how stale completion is rejected.
4. The crash window between receiver acceptance and sender completion, and why delivery is at least once.
5. Why replay keeps the signed event ID stable and why receivers must deduplicate business effects.
6. Why PostgreSQL was sufficient here and when an external broker could be justified.

Use these bullets after running and understanding the project. Do not claim Kafka, RabbitMQ, AWS, Kubernetes, production traffic, or latency/throughput numbers unless you add and verify them. Measure actual load-test results before adding metrics.
