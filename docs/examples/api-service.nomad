job "api-service" {
  datacenters = ["dc1"]
  type        = "service"

  group "api" {
    count = 3

    network {
      port "http" {
        to = 3000
      }
    }

    service {
      name = "api-service"
      port = "http"
      
      check {
        type     = "http"
        path     = "/api/health"
        interval = "15s"
        timeout  = "3s"
      }

      tags = [
        "api",
        "backend",
        "microservice"
      ]
    }

    task "api-server" {
      driver = "docker"

      config {
        image = "myregistry/api-service:${NOMAD_META_tag_id}"
        ports = ["http"]
      }

      env {
        NODE_ENV     = "production"
        PORT         = "3000"
        DATABASE_URL = "postgres://user:pass@db.service.consul:5432/api"
        REDIS_URL    = "redis://cache.service.consul:6379"
        JWT_SECRET   = "${NOMAD_VAR_jwt_secret}"
      }

      resources {
        cpu    = 256
        memory = 512
      }
    }
  }
}
