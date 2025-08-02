# Shipper Deployment Service

A lightweight Go service that acts as a secure deployment gateway for [HashiCorp Nomad](https://www.nomadproject.io/) clusters. This service provides a simple REST API for triggering and monitoring service deployments through Nomad.

## 🚀 Features

- **Secure Authentication**: API key-based authentication for deployment requests
- **Deployment Tracking**: SQLite database for tracking deployment status and history
- **Health Monitoring**: Built-in health check endpoint
- **Service Validation**: Optional whitelist of allowed services for enhanced security
- **Monitoring Integration**: Optional New Relic integration
- **Docker Ready**: Full containerization support with Docker Compose
- **Hot Reload Development**: Development environment with hot reload capabilities

## 📋 API Endpoints

### Health Check

```http
GET /health
```

Returns the service health status.

### Deploy Service

```http
POST /deploy
Content-Type: application/json

{
  "service_name": "my-service",
  "secret_key": "your-64-character-secret-key"
}
```

Triggers a deployment for the specified service.

### Check Deployment Status

```http
GET /status/{tag_id}
```

Returns the status of a specific deployment by its tag ID.

## ⚙️ Configuration

The service is configured through environment variables. Copy `.env.example` to `.env` and modify as needed:

```bash
cp .env.example .env
```

### Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `NOMAD_URL` | Nomad API URL | `https://your-nomad-cluster:4646` | ✅ |
| `NOMAD_TOKEN` | Nomad API token | - | ✅ |
| `RPC_SECRET` | Secret key for API authentication (64 chars) | - | ✅ |
| `PORT` | Server port | `16166` | ❌ |
| `VALID_SERVICES` | Comma-separated list of allowed services | (all allowed) | ❌ |
| `SKIP_TLS_VERIFY` | Skip TLS verification for Nomad API | `false` | ❌ |
| `LOG_LEVEL` | Logging level (debug, info, warn, error) | `info` | ❌ |
| `LOG_FORMAT` | Log format (json, text) | `json` | ❌ |
| `NEW_RELIC_ENABLED` | Enable New Relic monitoring | `false` | ❌ |
| `NEW_RELIC_LICENSE_KEY` | New Relic license key | - | ❌ |
| `NEW_RELIC_APP_NAME` | New Relic application name | `shipper-deployment` | ❌ |

## 🚀 Quick Start

### Prerequisites

- Go 1.24+ (for local development)
- Docker and Docker Compose (for containerized deployment)
- Access to a Nomad cluster

### Local Development

1. **Clone the repository**

   ```bash
   git clone <repository-url>
   cd bastion-deployment
   ```

2. **Set up environment variables**

   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

3. **Run locally**

   ```bash
   make run
   ```

### Docker Development

For development with hot reload (using Air inside Docker):

```bash
make dev
```

This starts the service with Docker Compose and automatically reloads on code changes using the `.air.toml` configuration.

### Production Deployment

1. **Build the Docker image**

   ```bash
   make docker-build
   ```

2. **Run with Docker Compose**

   ```bash
   make docker-run
   ```

## 🛠️ Development

### Available Make Commands

```bash
make build      # Build the application binary
make run        # Run the application locally
make dev        # Start development environment with hot reload
make test       # Run tests
make fmt        # Format code
make tidy       # Tidy Go modules
make clean      # Clean build artifacts
make docker-build    # Build Docker image
make docker-run      # Run with Docker Compose
make docker-stop     # Stop Docker Compose
```

### Project Structure

```text
.
├── cmd/
│   └── shipper/        # Application entry point
├── internal/
│   ├── config/         # Configuration management
│   ├── database/       # Database operations
│   ├── handlers/       # HTTP handlers
│   ├── logger/         # Logging setup
│   ├── models/         # Data models
│   ├── newrelic/       # New Relic integration
│   ├── nomad/          # Nomad client
│   └── server/         # HTTP server setup
├── .env.example        # Environment variables template
├── docker-compose.yml  # Production Docker Compose
├── docker-compose.dev.yml # Development Docker Compose
└── Makefile           # Build automation
```

## 🔒 Security Considerations

- **Change Default Secrets**: Always use a secure 64-character secret key in production
- **Service Whitelist**: Use `VALID_SERVICES` to restrict which services can be deployed
- **TLS Verification**: Keep `SKIP_TLS_VERIFY=false` in production environments
- **Network Security**: Ensure proper network policies for Nomad cluster access
- **Environment Variables**: Never commit `.env` files with real credentials

## 📊 Monitoring

The service includes optional New Relic integration for application performance monitoring. Enable by setting:

```bash
NEW_RELIC_ENABLED=true
NEW_RELIC_LICENSE_KEY=your-license-key
NEW_RELIC_APP_NAME=shipper-deployment
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🐛 Troubleshooting

### Common Issues

1. **Connection to Nomad fails**
   - Verify `NOMAD_URL` and `NOMAD_TOKEN` are correct
   - Check network connectivity to Nomad cluster
   - Verify TLS settings with `SKIP_TLS_VERIFY`

2. **Authentication errors**
   - Ensure `RPC_SECRET` is exactly 64 characters
   - Verify the secret key matches in deployment requests

3. **Service deployment fails**
   - Check if service name is in `VALID_SERVICES` (if configured)
   - Verify Nomad token has sufficient permissions
   - Check Nomad cluster status and resources
