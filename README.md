# service-platform
> A modular Go platform delivering APIs, WhatsApp/Twilio messaging, schedulers, monitoring, and automation services in one repo.

![Go](https://img.shields.io/badge/built%20with-Go%201.26-black?logo=go)
![Podman](https://img.shields.io/badge/containerized%20with-Podman-blue?logo=podman)
![License](https://img.shields.io/badge/license-Apache%202.0-lightgrey)
![CI](https://img.shields.io/github/actions/workflow/status/wecrazy/service-platform/ci.yml?label=GitHub%20CI)

## ⚙️ Live at a glance

| Service | Entry point | Type | Status (docs) |
|---------|-------------|------|---------------|
| API + Admin | `cmd/api/main.go` | HTTP + GRPC | [REST API docs](docs/swagger.yaml)
| WhatsApp worker | `cmd/whatsapp/main.go` | Messaging | 💬 Handles WhatsApp chats
| Twilio WhatsApp | `cmd/twilio/whatsapp/main.go` | gRPC + Twilio SDK | ✅ Rich media + rate limits (see docs)
| Scheduler | `cmd/scheduler/main.go` | Cron jobs | ⏱️ Cron-based jobs from `internal/scheduler`
| Monitoring | `cmd/monitoring/main.go` | Grafana/Grafana/Tempo | [monitoring guide](docs/MONITORING.md)
| n8n | `cmd/n8n/main.go` | Workflow automation | Integrations via [docs/N8N_TESTING_GUIDE.md](docs/N8N_TESTING_GUIDE.md)

## 🧭 Getting started (5 mins)

1. Clone and install Go 1.26 + Podman/Docker.
2. Copy `internal/config/service-platform.dev.yaml` → `service-platform.<env>.yaml`, then fill secrets (DB, Twilio, monitoring ports, etc.).
3. Bring up dependencies: `make mongo-up`, `make monitoring-start` (or call `scripts/monitoring-quickstart.sh`).
4. Run `make build` or `go run` the service you need (`cmd/api`, `cmd/whatsapp`, etc.).
5. Run `make test` or targeted suites (`make test-twilio`, `make test-mongo`, k6 targets).

## 💡 Highlighted features

### 📦 Modular binaries
- Each `cmd/<service>` builds its own binary so you can deploy just what you need.
- Shared logic in `internal/` (config loader, Twilio & WhatsApp clients, middleware, rate limiter).

### 🧵 Twilio WhatsApp stack
- Config: `internal/config/service-platform.dev.yaml` (overridden by ENV + `config_mode`/`conf.yaml`).
- Client + gRPC: `internal/twilio/client.go` + `cmd/twilio/whatsapp/main.go`. See [docs/TWILIO_WHATSAPP_SETUP.md](docs/TWILIO_WHATSAPP_SETUP.md).
- Integration tests: `tests/twilio/client_test.go` uses real Twilio credentials, media payloads, rich text, and mentions.

### 🧠 Monitoring & observability
- Podman-first monitoring stack managed via `scripts/monitoring-quickstart.sh` (fallback to Docker Compose).
- Grafana dashboards auto-provisioned from `monitoring/grafana/`, Prometheus scrapes each service, Loki & Tempo ingest logs/traces.
- Reference: [docs/MONITORING.md](docs/MONITORING.md), [docs/LOKI_TEMPO_SETUP.md](docs/LOKI_TEMPO_SETUP.md), [docs/LOKI_QUICKSTART.md](docs/LOKI_QUICKSTART.md).

### 🧪 Testing & performance
- `make test` → all Go tests (`./tests/...`, migrations).
- Twilio focused: `make test-twilio`, Twilio sandbox helper `scripts/test-twilio-whatsapp.sh`.
- k6 scripts under `tests/k6/` with `make k6-*` helpers; metrics flow into Grafana via Prometheus (configuration described in [docs/K6_INTEGRATION.md](docs/K6_INTEGRATION.md)).

### ⚙️ Automation + migration
- Migrations: `cmd/migrate`, `make migrate-up/down/reset/status` with GORM.
- Seeders: `cmd/seed`, targeted helpers like `make seed-whatsapp`, `make seed-users`.

### 📘 Documentation hub
- Read the rest: `docs/TWILIO_WHATSAPP_SETUP.md`, `docs/TWILIO_WHATSAPP_GUIDE.md`, `docs/TWILIO_SANDBOX_TROUBLESHOOTING.md`, `docs/MONITORING_SETUP_COMPLETE.md`, `docs/N8N_TESTING_GUIDE.md`, `docs/K6_INTEGRATION.md`.
- Swagger specs live in `docs/swagger.yaml`/`.json`; run `make swagger` to regenerate.

## 🎛️ Config & secrets

- Default config mode set via `internal/config/conf.yaml` (dev/prod detection via `ENV`, `GO_ENV`).
- Always edit `internal/config/service-platform.<env>.yaml` and keep secrets out of Git.
- Example sections: Twilio credentials, metrics ports, rate limiting, observability endpoints, k6 thresholds. Search `service-platform.dev.yaml` for templates.

## 🚀 Deploy & CI

- Multi-stage Dockerfiles assemble statically linked binaries, copy `internal/config/service-platform.dev.yaml`, and expose service ports.
- CI frameworks in `.github/workflows/` run lint, test, build, security scans, and releases (see README under `.github/` for workflow details).
- Dependabot lives in `.github/dependabot.yml` for dependency hygiene.

## 🧩 Need help?

- `make help` lists all targets (build, run, monitoring, tests, k6, etc.).
- `scripts/` contains helper tooling (monitoring stack, monitoring-cleanup, k6 runner, Twilio sandbox tests).
- For Twilio messaging questions consult `docs/TWILIO_WHATSAPP_GUIDE.md` and `docs/TWILIO_SANDBOX_TROUBLESHOOTING.md`.

## 📜 License

Licensed under the Apache License 2.0. See [LICENSE](LICENSE).