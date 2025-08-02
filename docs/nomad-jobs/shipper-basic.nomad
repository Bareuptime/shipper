job "shipper-deployment-basic" {
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
    }
  }
}
