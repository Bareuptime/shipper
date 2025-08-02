job "shipper-deployment-production" {
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

    # Persistent volume for database
    volume "shipper_data" {
      type            = "csi"
      source          = "shipper_data"
      read_only       = false
      attachment_mode = "file-system"
      access_mode     = "single-node-writer"
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

      # Database persistence
      volume_mount {
        volume      = "shipper_data"
        destination = "/root/data"
      }

      # Logs configuration
      logs {
        max_files     = 5
        max_file_size = 10
      }
    }
  }
}
