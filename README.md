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
- `PORT`: Server port (default: 8080)

## Running

### Local Development
```bash
go run .
```

### Docker
```bash
docker build -t bastion-deployment .
docker run -p 8080:8080 bastion-deployment
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
