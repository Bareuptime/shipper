# Deploying Shipper Deployment Service in Nomad

This guide explains how to deploy the Shipper Deployment Service itself in a Nomad cluster, as well as how to use it to deploy other services.

## Table of Contents

- [Deploying Shipper to Nomad](#deploying-shipper-to-nomad)
- [Service Architecture](#service-architecture)
- [Prerequisites](#prerequisites)
- [Basic Deployment](#basic-deployment)
- [Production Deployment](#production-deployment)
- [Configuration Options](#configuration-options)
- [Scaling and High Availability](#scaling-and-high-availability)
- [Monitoring and Observability](#monitoring-and-observability)
- [Troubleshooting](#troubleshooting)

## Deploying Shipper to Nomad

### Prerequisites

Before deploying the Shipper service in Nomad, ensure you have:

1. **Nomad Cluster**: A running Nomad cluster with appropriate access
2. **Docker Registry**: Access to a container registry containing the Shipper image
3. **Nomad Token**: A Nomad token with sufficient permissions for:
   - Creating and managing jobs
   - Reading cluster information
   - Managing allocations and evaluations
4. **Network Configuration**: Proper network policies allowing:
   - Shipper service to communicate with Nomad API
   - External services to reach Shipper's API endpoints
   - Database persistence (for SQLite)

### Service Architecture

The Shipper Deployment Service acts as a secure gateway between external deployment requests and your Nomad cluster:

```
[External Service] → [Shipper API] → [Nomad Cluster] → [Target Applications]
                           ↓
                    [SQLite Database]
                    (Deployment Tracking)
```

## Basic Deployment

### Step 1: Create the Nomad Job File

Create a basic Nomad job file for the Shipper service:

```hcl
job "shipper-deployment" {
  datacenters = ["dc1"]
  type        = "service"

  group "shipper" {
    count = 1

    network {
      port "http" {
        static = 16166
        to     = 16166
      }
    }

    service {
      name = "shipper-deployment"
      port = "http"

      check {
        type     = "http"
        path     = "/health"
        interval = "30s"
        timeout  = "10s"
      }

      tags = [
        "shipper",
        "deployment",
        "api"
      ]
    }

    task "shipper" {
      driver = "docker"

      config {
        image = "your-registry/shipper-deployment:latest"
        ports = ["http"]
        
        # Mount for database persistence
        volumes = [
          "local/data:/root/data"
        ]
      }

      # Environment variables
      env {
        NOMAD_URL          = "https://nomad.service.consul:4646"
        NOMAD_TOKEN        = "${NOMAD_VAR_nomad_token}"
        RPC_SECRET         = "${NOMAD_VAR_rpc_secret}"
        PORT               = "16166"
        LOG_LEVEL          = "info"
        LOG_FORMAT         = "json"
        SKIP_TLS_VERIFY    = "false"
      }

      # Resource allocation
      resources {
        cpu    = 256
        memory = 512
      }

      # Database persistence
      volume_mount {
        volume      = "shipper_data"
        destination = "/root/data"
      }
    }

    # Persistent volume for database
    volume "shipper_data" {
      type   = "host"
      source = "shipper_data"
    }
  }
}
```

### Step 2: Deploy the Service

```bash
# Submit the job to Nomad
nomad job run shipper-deployment.nomad

# Check job status
nomad job status shipper-deployment

# Check service health
curl http://shipper.service.consul:16166/health
```

## Production Deployment

For production environments, consider the following enhancements:

### Enhanced Job Configuration

```hcl
job "shipper-deployment" {
  datacenters = ["dc1", "dc2"]
  type        = "service"
  priority    = 80

  # Constraint to ensure deployment on stable nodes
  constraint {
    attribute = "${meta.node_class}"
    value     = "production"
  }

  # Update strategy
  update {
    max_parallel      = 1
    min_healthy_time  = "30s"
    healthy_deadline  = "5m"
    progress_deadline = "10m"
    auto_revert       = true
    canary            = 1
  }

  group "shipper" {
    count = 3  # High availability with multiple instances

    # Spread across different nodes
    spread {
      attribute = "${node.unique.id}"
      weight    = 100
    }

    # Restart policy
    restart {
      attempts = 3
      interval = "5m"
      delay    = "30s"
      mode     = "fail"
    }

    # Reschedule policy
    reschedule {
      attempts       = 5
      interval       = "1h"
      delay          = "30s"
      delay_function = "exponential"
      max_delay      = "10m"
      unlimited      = false
    }

    network {
      port "http" {
        to = 16166
      }
    }

    # Service registration with Consul
    service {
      name = "shipper-deployment"
      port = "http"

      # Health checks
      check {
        type     = "http"
        path     = "/health"
        interval = "15s"
        timeout  = "5s"
        
        check_restart {
          limit           = 3
          grace           = "30s"
          ignore_warnings = false
        }
      }

      # Service mesh integration (if using Consul Connect)
      connect {
        sidecar_service {
          proxy {
            upstreams {
              destination_name = "nomad-api"
              local_bind_port  = 4646
            }
          }
        }
      }

      tags = [
        "shipper",
        "deployment",
        "api",
        "production",
        "traefik.enable=true",
        "traefik.http.routers.shipper.rule=Host(`shipper.yourdomain.com`)",
        "traefik.http.routers.shipper.entrypoints=web-secure",
        "traefik.http.routers.shipper.tls.certresolver=letsencrypt"
      ]
    }

    task "shipper" {
      driver = "docker"

      config {
        image = "your-registry/shipper-deployment:${NOMAD_VAR_app_version}"
        ports = ["http"]
        
        # Security options
        security_opt = [
          "no-new-privileges:true"
        ]
        
        # Run as non-root user
        user = "1000:1000"
        
        # Mount for database persistence
        volumes = [
          "secrets/nomad.pem:/certs/nomad.pem:ro",
          "secrets/nomad-key.pem:/certs/nomad-key.pem:ro"
        ]
      }

      # Vault integration for secrets
      vault {
        policies = ["shipper-deployment"]
      }

      # Environment variables with Vault secrets
      template {
        data = <<EOF
NOMAD_URL={{ with secret "kv/data/shipper" }}{{ .Data.data.nomad_url }}{{ end }}
NOMAD_TOKEN={{ with secret "kv/data/shipper" }}{{ .Data.data.nomad_token }}{{ end }}
RPC_SECRET={{ with secret "kv/data/shipper" }}{{ .Data.data.rpc_secret }}{{ end }}
NEW_RELIC_LICENSE_KEY={{ with secret "kv/data/shipper" }}{{ .Data.data.newrelic_key }}{{ end }}
PORT=16166
LOG_LEVEL=info
LOG_FORMAT=json
SKIP_TLS_VERIFY=false
NEW_RELIC_ENABLED=true
NEW_RELIC_APP_NAME=shipper-deployment-prod
EOF
        destination = "secrets/env"
        env         = true
      }

      # TLS certificates for Nomad API communication
      template {
        data = <<EOF
{{ with secret "pki/issue/nomad-client" "common_name=shipper-deployment" }}
{{ .Data.certificate }}{{ end }}
EOF
        destination = "secrets/nomad.pem"
      }

      template {
        data = <<EOF
{{ with secret "pki/issue/nomad-client" "common_name=shipper-deployment" }}
{{ .Data.private_key }}{{ end }}
EOF
        destination = "secrets/nomad-key.pem"
      }

      # Resource allocation
      resources {
        cpu    = 500
        memory = 1024
      }

      # Logs configuration
      logs {
        max_files     = 5
        max_file_size = 10
      }
    }
  }
}
```

## Configuration Options

### Environment Variables

| Variable | Description | Example | Required |
|----------|-------------|---------|----------|
| `NOMAD_URL` | Nomad cluster API endpoint | `https://nomad.service.consul:4646` | ✅ |
| `NOMAD_TOKEN` | Nomad API authentication token | `s.xyz123...` | ✅ |
| `RPC_SECRET` | API authentication secret (64 chars) | Generated securely | ✅ |
| `PORT` | Service listening port | `16166` | ❌ |
| `SKIP_TLS_VERIFY` | Skip TLS verification for Nomad API | `false` | ❌ |
| `LOG_LEVEL` | Logging verbosity | `info` | ❌ |
| `LOG_FORMAT` | Log output format | `json` | ❌ |
| `NEW_RELIC_ENABLED` | Enable New Relic monitoring | `false` | ❌ |

### Nomad Variables

For production deployments, use Nomad variables to manage sensitive configuration:

```bash
# Create variables for sensitive data
nomad var put -namespace=default nomad/jobs/shipper-deployment \
  nomad_token="s.xyz123..." \
  rpc_secret="your-64-character-secret-key" \
  newrelic_key="your-new-relic-license-key"
```

## Scaling and High Availability

### Horizontal Scaling

The Shipper service can be scaled horizontally by increasing the `count` parameter:

```hcl
group "shipper" {
  count = 5  # Scale to 5 instances
  
  # Load balancing considerations
  spread {
    attribute = "${node.datacenter}"
    weight    = 100
  }
}
```

### Database Considerations

Since the service uses SQLite for deployment tracking:

1. **Single Writer**: Only one instance should write to the database
2. **Shared Storage**: Use network-attached storage for database sharing
3. **Consider Alternatives**: For high-scale deployments, consider PostgreSQL

### Load Balancing

Configure your load balancer (Traefik, HAProxy, etc.) to distribute requests:

```hcl
# Traefik example tags
tags = [
  "traefik.enable=true",
  "traefik.http.routers.shipper.rule=Host(`shipper.internal`)",
  "traefik.http.services.shipper.loadbalancer.sticky.cookie=true"
]
```

## Monitoring and Observability

### Health Checks

The service provides a health endpoint that should be monitored:

```hcl
check {
  type     = "http"
  path     = "/health"
  interval = "30s"
  timeout  = "10s"
  
  # Restart unhealthy tasks
  check_restart {
    limit           = 3
    grace           = "30s"
    ignore_warnings = false
  }
}
```

### Metrics Collection

Integrate with your metrics collection system:

```hcl
# Prometheus metrics (if available)
tags = [
  "prometheus.io/scrape=true",
  "prometheus.io/port=16166",
  "prometheus.io/path=/metrics"
]
```

### Logging

Configure structured logging for better observability:

```hcl
env {
  LOG_LEVEL  = "info"
  LOG_FORMAT = "json"
}

# Log aggregation
logs {
  max_files     = 10
  max_file_size = 100
}
```

## Troubleshooting

### Common Issues

1. **Nomad Connection Failures**
   ```bash
   # Check network connectivity
   nomad job status shipper-deployment
   
   # Verify token permissions
   nomad acl token self
   ```

2. **Database Lock Issues**
   ```bash
   # Check multiple instances writing to same DB
   nomad alloc logs <alloc-id> shipper
   ```

3. **Service Discovery Problems**
   ```bash
   # Verify Consul service registration
   consul catalog services
   consul health service shipper-deployment
   ```

### Debug Mode

Enable debug logging for troubleshooting:

```hcl
env {
  LOG_LEVEL = "debug"
}
```

### Performance Tuning

For high-throughput deployments:

```hcl
resources {
  cpu    = 1000  # Increase CPU allocation
  memory = 2048  # Increase memory allocation
}

# Tune database settings if needed
env {
  SQLITE_CACHE_SIZE = "10000"
}
```

## Security Considerations

1. **Network Policies**: Restrict access to Shipper API endpoints
2. **Authentication**: Use strong, randomly generated RPC secrets
3. **TLS**: Always use TLS for Nomad API communication in production
4. **Secrets Management**: Use Vault or Nomad variables for sensitive data
5. **Container Security**: Run containers as non-root users when possible

## Next Steps

- [API Usage Examples](./api-usage.md)
- [Job File Templates](./examples/)
- [Integration Patterns](./integration-patterns.md)
- [Performance Tuning](./performance-tuning.md)
