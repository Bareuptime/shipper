# Bastion Deployment Service

A lightweight Go service that acts as a jump server for deploying services to Nomad.

## Features

- **Health Check**: `/health` endpoint for service health monitoring
- **Deploy**: `/deploy` endpoint to trigger service deployments
- **Status**: `/status/{tag_id}` endpoint to check deployment status
- **SQLite Database**: Lightweight database for tracking deployments
- **Docker Support**: Containerized deployment ready

## Endpoints

### Health Check
```
GET /health
```

### Deploy Service
```
POST /deploy
Content-Type: application/json

{
  "service_name": "my-service",
  "secret_key": "your-64-character-secret-key"
}
```

### Check Status
```
GET /status/{tag_id}
```

## Configuration

Set environment variables:
- `NOMAD_URL`: Nomad API URL (default: http://10.10.85.1:4646)
- `VALID_SECRET`: 64-character secret key for authentication
- `PORT`: Server port (default: 16166)

## Environment Variables

The service supports the following environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `NOMAD_URL` | Nomad API URL | `http://10.10.85.1:4646` |
| `VALID_SECRET` | Secret key for API authentication | `your-64-character-secret-key-here-please-change-this-in-production` |
| `PORT` | Server port | `16166` |
| `VALID_SERVICES` | Comma-separated list of valid service names | (empty - allows all services) |
| `LOG_LEVEL` | Logging level (debug, info, warn, error) | `info` |
| `LOG_FORMAT` | Log format (json, text) | `json` |

### Example Configuration

```bash
export NOMAD_URL="http://nomad.example.com:4646"
export VALID_SECRET="your-production-secret-key-here-64-characters-long"
export PORT="8080"
export VALID_SERVICES="web-app,api-service,worker-service"
export LOG_LEVEL="debug"
export LOG_FORMAT="json"
```

## Running

### Local Development

```bash
go run .
```

### Hot Reload Development

Install Air for hot reload:

```bash
make install-air
```

Start with hot reload:

```bash
make dev
# or
./dev.sh
```

### Docker Development

```bash
make dev-docker
```

### Docker Production

```bash
docker build -t bastion-deployment .
docker run -p 16166:16166 bastion-deployment
```

### Docker Compose

```bash
docker-compose up -d
```

## Build

```bash
make build
```

## Security

- Change the default secret key in production
- Use environment variables for configuration
- Service validates secret key for all deployment requests
