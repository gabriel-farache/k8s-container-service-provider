# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

DCM (Data Center Management) is a service provider that manages containers in Kubernetes clusters via a REST API following AEP (API Enhancement Proposals) standards. It maps the container API to Kubernetes Deployments/Pods/Services.

**Module:** `github.com/dcm-project/k8s-container-service-provider`

## Commands

```bash
make build              # Build binary to bin/k8s-container-service-provider
make test               # Run all tests (Ginkgo v2, race detector)
make test-cover         # Run tests with coverage (creates coverprofile.out)
make lint               # Run golangci-lint
make check              # fmt + vet + lint + test
make generate-api       # Regenerate all code from OpenAPI spec
make check-aep          # Validate OpenAPI spec against AEP standards (requires spectral)
```

### Running specific tests

```bash
# Single package
go run github.com/onsi/ginkgo/v2/ginkgo -r internal/handlers/container

# Single test by name/TC-ID
go run github.com/onsi/ginkgo/v2/ginkgo -r -v -focus "TC-U009" internal/handlers/container
```

## Architecture

### OpenAPI-first code generation

The API is defined in `api/v1alpha1/openapi.yaml`. All request/response types, server interfaces, embedded spec, and HTTP client are generated from it using `oapi-codegen`. After modifying the OpenAPI spec, run `make generate-api`. CI enforces that the generated code is up to date.

Generated files (do not edit manually):
- `api/v1alpha1/types.gen.go` — data models
- `api/v1alpha1/spec.gen.go` — embedded OpenAPI spec for request validation
- `internal/api/server/server.gen.go` — Chi router + strict server interfaces
- `pkg/client/client.gen.go` — HTTP client

### Request flow

`main.go` → HTTP server (`internal/apiserver/`) with middleware (recovery, OpenAPI request validation) → container handler (`internal/handlers/container/`) implements `StrictServerInterface`, validates business rules → store interface (`internal/store/repository.go`) → Kubernetes implementation (`internal/kubernetes/`)

### Key patterns

- **Strict server interface**: oapi-codegen generates a `StrictServerInterface` with typed request/response objects. Handlers implement this interface — no manual HTTP parsing.
- **Repository pattern**: `internal/store/repository.go` defines `ContainerRepository`. The Kubernetes implementation in `internal/kubernetes/` maps containers to Deployments. Custom error types (`NotFoundError`, `ConflictError`, `InvalidArgumentError`) in `internal/store/errors.go` drive HTTP status code mapping in handlers.
- **RFC 7807 errors**: All error responses use Problem Details format with types like `INVALIDARGUMENT`, `NOTFOUND`, `ALREADYEXISTS`, `INTERNAL`.
- **Handler validation**: `internal/handlers/container/validation.go` validates business rules (CPU/memory min≤max, reserved label keys, container ID format per AEP-122).
- **Config**: Environment variables with `SP_` prefix are parsed via `caarlos0/env` into structs in `internal/config/`.

### Testing

- **Framework**: Ginkgo v2 (BDD) + Gomega assertions
- **Test naming**: Files use `_unit_test.go` / `_integration_test.go` suffixes. Test cases carry `TC-XXXX` identifiers.
- **Mocks**: Hand-written function-field mocks (e.g., `mockContainerRepository` with `CreateFunc`, `GetFunc`, etc.) — no mocking framework.
- **Spec first**: New requirements (REQ-*) and acceptance criteria (AC-*) MUST be added to the spec file(s) in `.ai/specs/` before any implementation or test planning begins.
- **Test plan first**: New test cases (TC-*) MUST be registered in the test plan (`.ai/test-plans/`) with mappings to REQ-* and AC-* from the spec before being implemented in code.

### Linting

golangci-lint excludes generated code directories (`api/v1alpha1/`, `pkg/client/`). See `.golangci.yml` for enabled linters.

### Commit format

```
<type>(<scope>): <subject>
```

Use `git commit -s` to add sign-off. Types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`.
