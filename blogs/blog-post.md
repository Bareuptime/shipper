# Simplifying Deployments with HashiCorp Nomad: Building a Secure Deployment Gateway

In today's containerized world, organizations are constantly seeking solutions that balance simplicity, security, and operational efficiency. While Kubernetes dominates the headlines, HashiCorp Nomad offers a compelling alternative that prioritizes ease of use without sacrificing power. In this post, I'll share how we built a lightweight deployment service that leverages Nomad's strengths and explore why Nomad and Consul make an excellent foundation for modern infrastructure.

## The Problem: Complex Deployment Pipelines

Most teams face similar challenges when deploying applications:

- **Security concerns**: Giving CI/CD systems direct access to cluster APIs
- **Complexity overhead**: Managing deployment configurations across environments
- **Operational burden**: Maintaining complex orchestration setups
- **Resource constraints**: Running heavy orchestration platforms on limited infrastructure

After evaluating various solutions, we found existing tools either too complex for our needs or requiring significant operational overhead. This led us to build Shipper, a lightweight deployment service designed specifically for Nomad clusters.

## Why Choose Nomad Over Kubernetes?

### Simplicity Without Compromise

Nomad's architecture is refreshingly straightforward. Where Kubernetes requires dozens of components and concepts, Nomad operates with a clean, minimal design:

```hcl
job "web-app" {
  datacenters = ["dc1"]
  type        = "service"
  
  group "web" {
    count = 3
    
    service {
      name = "web-app"
      port = "http"
      
      check {
        type = "http"
        path = "/health"
      }
    }
    
    task "app" {
      driver = "docker"
      config {
        image = "myapp:latest"
        ports = ["http"]
      }
    }
  }
}
```

This simplicity translates to:
- **Faster learning curve**: New team members become productive quickly
- **Reduced operational overhead**: Fewer moving parts mean fewer things to break
- **Lower resource requirements**: Nomad runs efficiently on modest hardware

### Multi-Workload Support

Unlike container-focused platforms, Nomad natively supports multiple workload types:

- **Docker containers**: Full container orchestration
- **Raw executables**: Direct binary execution
- **Java applications**: JVM-based workloads
- **Virtual machines**: VM orchestration via QEMU
- **System services**: Traditional daemon management

This flexibility eliminates the need for multiple orchestration platforms.

### Operational Efficiency

Nomad's operational characteristics make it attractive for teams wanting to focus on applications rather than infrastructure:

- **Single binary deployment**: No complex installation procedures
- **Automatic failover**: Built-in leader election and failure handling
- **Rolling deployments**: Zero-downtime updates out of the box
- **Resource optimization**: Intelligent bin-packing across nodes

## The Power of Consul Integration

Nomad's integration with Consul creates a powerful service mesh foundation:

### Service Discovery

Services automatically register with Consul, enabling dynamic discovery:

```hcl
service {
  name = "api-service"
  port = "http"
  
  tags = [
    "api",
    "version-v1.2.3",
    "environment-production"
  ]
  
  check {
    type     = "http"
    path     = "/health"
    interval = "10s"
  }
}
```

Applications can then discover dependencies using DNS or Consul's API:
```bash
# DNS-based discovery
curl http://api-service.service.consul:8080/api/users

# Load balancing happens automatically
```

### Security with Consul Connect

Consul Connect provides automatic mTLS between services:

```hcl
service {
  name = "web-app"
  port = "http"
  
  connect {
    sidecar_service {
      proxy {
        upstreams {
          destination_name = "api-service"
          local_bind_port  = 8080
        }
      }
    }
  }
}
```

This enables:
- **Zero-trust networking**: Encrypted communication by default
- **Traffic management**: Intelligent routing and load balancing
- **Observability**: Built-in metrics and tracing

### Configuration Management

Consul's key-value store centralizes configuration:

```hcl
template {
  data = <<EOF
API_KEY={{ key "app/api_key" }}
DATABASE_URL={{ key "app/database_url" }}
EOF
  destination = "secrets/config.env"
  env = true
}
```

## Building the Shipper Deployment Service

Our deployment service acts as a secure gateway between CI/CD systems and Nomad:

### Architecture Overview

```
[CI/CD Pipeline] → [Shipper API] → [Nomad Cluster] → [Applications]
                        ↓
                 [SQLite Database]
                (Deployment Tracking)
```

### Key Features

**Security First**: Instead of exposing Nomad APIs directly to CI/CD systems, Shipper provides a controlled interface with API key authentication.

**Deployment Tracking**: SQLite database maintains deployment history and status, enabling audit trails and rollback capabilities.

**Flexibility**: Supports both service updates and custom job file uploads, accommodating various deployment scenarios.

**Resource Efficiency**: Written in Go with minimal dependencies, running comfortably in 256MB of RAM.

### Simple API Design

The API follows REST principles with straightforward endpoints:

```bash
# Deploy existing service
curl -X POST "https://shipper.company.com/deploy" \
  -H "X-Secret-Key: your-key" \
  -d '{"service_name": "web-app", "tag_id": "v1.2.3"}'

# Upload and deploy custom job
curl -X POST "https://shipper.company.com/deploy/job" \
  -H "X-Secret-Key: your-key" \
  -F "tag_id=migration-v1.0.0" \
  -F "job_file=@database-migration.nomad"

# Check deployment status
curl -X GET "https://shipper.company.com/status/v1.2.3" \
  -H "X-Secret-Key: your-key"
```

## Production Deployment Patterns

### High Availability Setup

Nomad's built-in clustering enables robust deployments:

```hcl
job "shipper-deployment" {
  datacenters = ["dc1", "dc2"]
  
  group "shipper" {
    count = 3  # Multiple instances for HA
    
    # Spread across availability zones
    spread {
      attribute = "${node.datacenter}"
      weight    = 100
    }
    
    # Automatic failover configuration
    restart {
      attempts = 3
      interval = "5m"
      mode     = "fail"
    }
  }
}
```

### Service Mesh Integration

Consul Connect provides secure service-to-service communication:

```hcl
service {
  name = "shipper-deployment"
  
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
}
```

### Secrets Management

Integration with HashiCorp Vault ensures secure credential handling:

```hcl
vault {
  policies = ["shipper-deployment"]
}

template {
  data = <<EOF
NOMAD_TOKEN={{ with secret "kv/data/shipper" }}{{ .Data.data.nomad_token }}{{ end }}
RPC_SECRET={{ with secret "kv/data/shipper" }}{{ .Data.data.rpc_secret }}{{ end }}
EOF
  destination = "secrets/env"
  env = true
}
```

## Real-World Benefits

After deploying this solution in production, we've observed significant benefits:

### Operational Simplicity

- **Reduced complexity**: Single deployment interface for all environments
- **Consistent patterns**: Same deployment process for all services
- **Clear audit trails**: Complete deployment history and status tracking

### Security Improvements

- **Controlled access**: No direct Nomad API exposure to CI/CD systems
- **Centralized authentication**: Single point for managing deployment permissions
- **Encrypted communication**: All service-to-service traffic secured by default

### Developer Experience

- **Fast deployments**: Minimal overhead from deployment service
- **Clear feedback**: Real-time status updates and error reporting
- **Flexible workflows**: Support for both standard deployments and custom jobs

### Cost Efficiency

- **Lower resource usage**: Entire stack runs on modest hardware
- **Reduced operational overhead**: Less time spent on infrastructure management
- **Simplified licensing**: No complex per-node pricing models

## Comparison with Alternatives

### Versus Kubernetes

| Aspect | Nomad + Consul | Kubernetes |
|--------|----------------|------------|
| **Complexity** | Simple architecture | Complex, many components |
| **Resource Usage** | Lightweight | Resource-intensive |
| **Learning Curve** | Gentle | Steep |
| **Multi-workload** | Native support | Container-focused |
| **Operational Overhead** | Minimal | Significant |

### Versus Managed Services

While cloud-managed Kubernetes services reduce operational overhead, they often come with:
- **Vendor lock-in**: Platform-specific configurations
- **Cost scaling**: Expensive as cluster size grows
- **Limited control**: Restricted customization options

Nomad provides the benefits of managed services while maintaining control and cost efficiency.

## Getting Started

The complete project, including documentation and example configurations, is available as open source. Key resources include:

- **Production-ready job files**: Templates for deploying Shipper in Nomad
- **Integration examples**: CI/CD pipeline configurations
- **Deployment patterns**: Blue-green, canary, and rolling update strategies
- **Security configurations**: Vault integration and access control

Whether you're running a small team or managing enterprise workloads, the combination of Nomad's simplicity and Consul's service mesh capabilities provides a compelling alternative to more complex orchestration platforms.

## Conclusion

HashiCorp Nomad offers a refreshing approach to workload orchestration that prioritizes simplicity without sacrificing capability. Combined with Consul's service mesh features, it provides a robust foundation for modern applications.

Our Shipper deployment service demonstrates how this stack can be leveraged to build practical solutions that solve real operational challenges. By focusing on essential features and leveraging Nomad's strengths, we've created a deployment pipeline that's both secure and maintainable.

The infrastructure landscape doesn't have to be complex. Sometimes, the most effective solutions are the simplest ones.

*The complete source code and documentation for the Shipper Deployment Service is available on [GitHub](https://github.com/Bareuptime/bastion). The project includes comprehensive guides for deploying in production environments and integrating with various CI/CD systems.*
