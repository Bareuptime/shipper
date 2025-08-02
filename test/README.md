# Test Suite

This directory contains comprehensive tests for the Shipper Deployment Service.

## Test Structure

### Unit Tests

- **`config_test.go`**: Tests configuration loading with various environment variables
- **`models_test.go`**: Tests JSON marshaling/unmarshaling of data models
- **`database_test.go`**: Tests database operations (CRUD) for deployments

### Integration Tests

- **`handlers_test.go`**: Tests HTTP handlers with mocked dependencies
- **`integration_test.go`**: Full HTTP server integration tests with authentication

## Running Tests

```bash
# Run all tests
make test

# Run tests with verbose output
go test ./test -v

# Run specific test
go test ./test -run TestConfigLoad -v
```

## Test Coverage

The tests cover:

- ✅ Configuration loading and validation
- ✅ Database operations (insert, update, get)
- ✅ HTTP handlers (health, deploy, status)
- ✅ Authentication middleware
- ✅ Error handling scenarios
- ✅ JSON request/response validation
- ✅ Multipart file upload handling

## Test Database

Tests use SQLite in-memory databases that are automatically created and cleaned up for each test. No external dependencies are required for testing.

## Mock Dependencies

- **Nomad API**: Tests expect network errors when calling mock Nomad servers (this is expected behavior)
- **New Relic**: Disabled in test configurations
- **File System**: Temporary files are created and cleaned up automatically
