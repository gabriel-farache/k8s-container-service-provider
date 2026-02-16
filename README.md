# Kubernetes Container Service Provider

A DCM (Data Center Management) service provider for managing containers in Kubernetes clusters.

## Overview

This service provider implements the DCM container management interface for Kubernetes environments, enabling standardized container lifecycle management across hybrid cloud infrastructures.

## Features

- Container lifecycle management (create, read, delete)
- Resource allocation and monitoring
- AEP-compliant REST API
- Kubernetes-native implementation
- DCM ecosystem integration

## Development

### Prerequisites

- Go 1.25.5 or later
- Access to a Kubernetes cluster
- Make

### Building

```bash
make build
```

### Running

```bash
make run
```

### Code Generation

This project uses OpenAPI specifications with oapi-codegen for code generation:

```bash
# Generate all API code from OpenAPI spec
make generate-api

# Verify generated code is up to date
make check-generate-api
```

### Testing

```bash
make test
```

### API Compliance

The OpenAPI specifications are validated against AEP (API Enhancement Proposals):

```bash
make check-aep
```

## API Documentation

The REST API is defined in `api/v1alpha1/openapi.yaml` and follows AEP standards for consistency across DCM services.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests and validation: `make test check-generate-api check-aep`
5. Commit your changes
6. Push to the branch
7. Create a Pull Request

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.