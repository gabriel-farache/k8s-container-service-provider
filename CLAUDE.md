# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

DCM (Data Center Management) is a service provider that manages containers in Kubernetes clusters via a REST API following AEP (API Enhancement Proposals) standards. It maps the container API to Kubernetes Deployments, Pods, and Services.

**Module:** `github.com/dcm-project/k8s-container-service-provider`

## DCM Ecosystem

This service provider is one component in the DCM platform. It directly interacts with two other DCM services at runtime:

| Component | Interaction | Config |
|---|---|---|
| [Service Provider Manager](https://github.com/dcm-project/service-provider-manager) | Registers itself on startup (name, endpoint, operations, schema version). Uses the SP Manager's Go client library (`pkg/client/provider`). | `DCM_REGISTRATION_URL` |
| NATS | Publishes container status change events as CloudEvents v1.0 to subject `dcm.container`. | `SP_NATS_URL` |

Related repositories:
- [**api-gateway**](https://github.com/dcm-project/api-gateway) — orchestrates the full DCM stack via `compose.yaml`; the recommended way to run all components together (see `README.md` for details)
- [**catalog-manager**](https://github.com/dcm-project/catalog-manager) — defines the canonical service type schemas in `api/v1alpha1/servicetypes/`. The `container/spec.yaml` and `common.yaml` there are the source of truth for the container input schema.

## Commands

```bash
make build              # Build binary to bin/k8s-container-service-provider
make test               # Run all tests (Ginkgo v2, race detector)
make test-cover         # Run tests with coverage (creates coverprofile.out)
make lint               # Run golangci-lint
make check              # fmt + vet + lint + test
make generate-api       # Regenerate all code from OpenAPI spec
make check-generate-api # Verify generated code is up to date (CI check)
make check-aep          # Validate OpenAPI spec against AEP standards (requires spectral)
```

### Running specific tests

```bash
# Single package
go run github.com/onsi/ginkgo/v2/ginkgo -r internal/handlers/container

# Single test by name/TC-ID
go run github.com/onsi/ginkgo/v2/ginkgo -r -v -focus "TC-U009" internal/handlers/container
```

## API Endpoints

Base path: `/api/v1alpha1/containers`

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1alpha1/containers/health` | Health check. Returns status, uptime, version. |
| `POST` | `/api/v1alpha1/containers` | Create container. Optional `id` query param (AEP-122 format). |
| `GET` | `/api/v1alpha1/containers` | List containers. Supports `max_page_size` (1-1000) and `page_token`. |
| `GET` | `/api/v1alpha1/containers/{container_id}` | Get container by ID. |
| `DELETE` | `/api/v1alpha1/containers/{container_id}` | Delete container. Returns 204 No Content. |

All error responses use RFC 7807 Problem Details with types: `INVALIDARGUMENT`, `NOTFOUND`, `ALREADYEXISTS`, `INTERNAL`.

## Architecture

### OpenAPI-first code generation

The API is defined in `api/v1alpha1/openapi.yaml`. All request/response types, server interfaces, embedded spec, and HTTP client are generated from it using `oapi-codegen`. After modifying the OpenAPI spec, run `make generate-api`. CI enforces that the generated code is up to date.

Generated files (do not edit manually):
- `api/v1alpha1/types.gen.go` — data models
- `api/v1alpha1/spec.gen.go` — embedded OpenAPI spec for request validation
- `internal/api/server/server.gen.go` — Chi router + strict server interfaces
- `pkg/client/client.gen.go` — HTTP client

### Request flow

`cmd/k8s-container-service-provider/main.go` → HTTP server (`internal/apiserver/`) with middleware chain: Recovery → Request Logging → Request Timeout → OpenAPI Validation → container handler (`internal/handlers/container/`) implements `StrictServerInterface`, validates business rules → store interface (`internal/store/repository.go`) → Kubernetes implementation (`internal/kubernetes/`)

### Internal packages

| Package | Purpose |
|---|---|
| `internal/apiserver/` | HTTP server setup, middleware chain (recovery, logging, timeout, OpenAPI validation), readiness probing |
| `internal/config/` | Environment variable parsing via `caarlos0/env`. Prefixes: `SP_*`, `SP_SERVER_*`, `SP_K8S_*`, `SP_NATS_*`, `SP_MONITOR_*`, `DCM_*` |
| `internal/handlers/container/` | Container API handler implementing `StrictServerInterface`. Business rule validation, error mapping. |
| `internal/store/` | `ContainerRepository` interface and custom error types (`NotFoundError`, `ConflictError`, `InvalidArgumentError`) |
| `internal/kubernetes/` | `K8sContainerStore` — Kubernetes implementation of `ContainerRepository`. Maps containers to Deployments/Services/Pods. |
| `internal/monitoring/` | `StatusMonitor` — watches Deployments/Pods via shared informers, reconciles status, publishes CloudEvents via NATS with per-instance debouncing |
| `internal/registration/` | `Registrar` — async self-registration with DCM Service Provider Manager using exponential backoff (1s initial, 60s max) |
| `internal/dcm/` | DCM label constants (`dcm.project/managed-by`, `dcm.project/dcm-instance-id`, `dcm.project/dcm-service-type`) and label selectors |
| `internal/units/` | CPU/memory unit conversions (API units MB/GB/TB to K8s units Mi/Gi/Ti and back) |
| `internal/httperror/` | RFC 7807 `application/problem+json` response writing |
| `internal/util/` | Generic helpers (e.g., `Ptr[T]`) |
| `internal/api/server/` | **Generated** — Chi router and `StrictServerInterface` |

### Kubernetes resource mapping

- **Deployment** (primary resource): one per container, always `replicas: 1`. Image, command, args, env, ports, and CPU/memory resource limits are mapped from the container spec.
- **Service** (conditional): created only when ports have non-`none` visibility. Port visibility drives Service type — `internal` → ClusterIP, `external` → configurable via `SP_K8S_EXTERNAL_SVC_TYPE` (LoadBalancer or NodePort).
- **Status derivation**: Pod phase maps to container status (Pending→PENDING, Running→RUNNING, Failed→FAILED, Unknown→UNKNOWN). When no Pod exists, status is derived from Deployment conditions.
- **Labeling**: all K8s resources get `dcm.project/managed-by`, `dcm.project/dcm-instance-id`, and `dcm.project/dcm-service-type` labels. User labels are merged; collisions with reserved DCM keys are rejected.

### Key patterns

- **Strict server interface**: oapi-codegen generates a `StrictServerInterface` with typed request/response objects. Handlers implement this interface — no manual HTTP parsing.
- **Repository pattern**: `internal/store/repository.go` defines `ContainerRepository`. The Kubernetes implementation in `internal/kubernetes/` maps containers to Deployments. Custom error types (`NotFoundError`, `ConflictError`, `InvalidArgumentError`) in `internal/store/errors.go` drive HTTP status code mapping in handlers.
- **RFC 7807 errors**: All error responses use Problem Details format with types like `INVALIDARGUMENT`, `NOTFOUND`, `ALREADYEXISTS`, `INTERNAL`.
- **Handler validation**: `internal/handlers/container/validation.go` validates business rules (CPU/memory min<=max, reserved label keys, container ID format per AEP-122).
- **Config**: Environment variables are parsed via `caarlos0/env` into structs in `internal/config/`. Prefixes: `SP_*` (provider identity), `SP_SERVER_*`, `SP_K8S_*`, `SP_NATS_*`, `SP_MONITOR_*`, `DCM_*` (registry).
- **Status monitoring**: `internal/monitoring/` watches K8s resources via shared informers, reconciles Deployment+Pod state into a single status, debounces rapid changes, and publishes CloudEvents v1.0 (`type: dcm.status.container`) to NATS subject `dcm.container`.
- **Registration**: `internal/registration/` registers with DCM on startup. Async with exponential backoff. Non-retryable errors (4xx) cause immediate failure; retryable errors (5xx, connection) retry indefinitely.

## Testing

- **Framework**: Ginkgo v2 (BDD) + Gomega assertions
- **Test naming**: Files use `_unit_test.go` / `_integration_test.go` suffixes. Test cases carry `TC-XXXX` identifiers.
- **Mocks**: Hand-written function-field mocks (e.g., `mockContainerRepository` with `CreateFunc`, `GetFunc`, etc.) — no mocking framework.
- **Spec first**: New requirements (REQ-*) and acceptance criteria (AC-*) MUST be added to the spec file(s) in `.ai/specs/` before any implementation or test planning begins.
- **Test plan first**: New test cases (TC-*) MUST be registered in the test plan (`.ai/test-plans/`) with mappings to REQ-* and AC-* from the spec before being implemented in code.

## .ai/ conventions

The `.ai/` directory holds project documentation artifacts:

```
.ai/
├── specs/              # Specifications with REQ-* and AC-* (git-tracked)
├── test-plans/         # Test plans with TC-* IDs (git-tracked)
├── decisions/          # Trust-boundary and design decision logs (local only)
├── plans/              # Implementation plans (local only)
├── checkpoints/        # Session state snapshots (local only)
├── exploration/        # Codebase analysis and research (local only)
└── reviews/            # Code review findings (local only)
```

Only `specs/` and `test-plans/` are committed to git. All other subdirectories are gitignored and remain local.

**Gate enforcement**: Spec (REQ + AC) must be complete before test plan (TC); test plan must be complete before implementation. See the spec and test plan files for the full requirements and coverage matrix.

## Linting

golangci-lint excludes generated code directories (`api/v1alpha1/`, `pkg/client/`). See `.golangci.yml` for enabled linters.

## Commit format

```
<type>(<scope>): <subject>
```

Use `git commit -s` to add sign-off. Types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`.
