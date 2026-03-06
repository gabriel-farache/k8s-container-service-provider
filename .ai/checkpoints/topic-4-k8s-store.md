# Checkpoint: Topic #4 -- K8s Container Store (All Phases Complete)

## Context
- **Task:** Topic #4 -- Kubernetes Integration & Container Store (CRUD)
- **Branch:** feat/introduce_K8S_service
- **Status:** All 3 phases complete (RED, GREEN, REFACTOR)
- **Final test count:** 43/43 pass

## Phase 1: RED (BDD Tests)

- Wrote 42 integration tests across 8 test files
- Test IDs: TC-I009 through TC-I081 (non-contiguous, per test plan)
- Created stub source files (store interface, errors, config, convert)
- All 42 tests verified failing (correct RED state)
- Committed: `f5c6354`

### Test Files
- `internal/kubernetes/store_test.go` -- Ginkgo suite bootstrap
- `internal/kubernetes/helpers_test.go` -- Shared helpers + TC-U024 compile-time assertion
- `internal/kubernetes/store_create_test.go` -- 13 tests
- `internal/kubernetes/store_service_test.go` -- 12 tests
- `internal/kubernetes/store_conflict_test.go` -- 3 tests
- `internal/kubernetes/store_get_test.go` -- 7 tests
- `internal/kubernetes/store_list_test.go` -- 4 tests (+ 1 added in REFACTOR = 5)
- `internal/kubernetes/store_delete_test.go` -- 3 tests

## Phase 2: GREEN (Implementation)

- Implemented all CRUD methods to make 42/42 tests pass
- Created 10 source files in `internal/kubernetes/`
- Committed: `6c0ba69`

### Source Files
- `internal/store/repository.go` -- ContainerRepository interface
- `internal/store/errors.go` -- NotFoundError, ConflictError, InvalidArgumentError
- `internal/kubernetes/config.go` -- K8sConfig struct
- `internal/kubernetes/convert.go` -- ConvertCPU, ConvertMemory, MapPodPhaseToStatus
- `internal/kubernetes/container.go` -- containerFromDeployment, enrichWith* functions
- `internal/kubernetes/deploy.go` -- buildDeployment
- `internal/kubernetes/labels.go` -- Label management, selectors
- `internal/kubernetes/service.go` -- shouldCreateService, buildService
- `internal/kubernetes/store.go` -- K8sContainerStore struct + constructor
- `internal/kubernetes/store_create.go` -- Create method
- `internal/kubernetes/store_delete.go` -- Delete method
- `internal/kubernetes/store_get.go` -- Get method
- `internal/kubernetes/store_list.go` -- List method

## Phase 3: REFACTOR (Code Review)

### Fixes Applied
| ID | Description |
|----|-------------|
| F-01 | Fixed panic on negative page token offset (added validation + TC-I079) |
| F-02 | Removed dead code from `deploy.go` (`int32Ptr`, `resourceQuantity`) |
| F-03 | Removed dead code from `helpers_test.go` (`int32Ptr`, `stringPtr`, `var _ = fmt.Sprintf`, `"fmt"` import) |
| F-04 | Added memory validation in `store_create.go` before `buildDeployment` |
| F-05 | Fixed List enrichment to match Get (Service data + Deployment condition fallback) |
| F-07 | Added Makefile targets (`lint`, `test-cover`, `check`) and `.golangci.yml` |
| F-08 | Strengthened TC-I077 assertion (direct `BeNil()` instead of conditional guard) |

### Deferred Items
| ID | Description | Reason |
|----|-------------|--------|
| F-06 | N+1 Pod/Service queries in List | Performance optimization, no correctness impact |
| F-09 | Missing unit test coverage for edge cases | Follow-up work |
| F-10 | K8sConfig env integration | Belongs in wiring/integration topic |

## Key Decisions
- No file reorganization needed -- structure is already idiomatic Go
- Memory validation added at store layer for defense-in-depth (API layer also validates via OpenAPI regex)
- List now matches Get behavior exactly for enrichment consistency

## Verification
```
go test -count=1 ./...    # 43/43 pass
go vet ./...              # Clean
go build ./...            # Clean
```
