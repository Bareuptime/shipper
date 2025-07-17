#!/bin/bash

# Development script for bastion-deployment
# This script sets up the development environment with hot reload

echo "🚀 Setting up development environment..."

# Check if Air is installed
if ! command -v air &> /dev/null; then
    echo "📦 Installing Air for hot reload..."
    go install github.com/air-verse/air@latest
fi

# Create tmp directory if it doesn't exist
mkdir -p tmp

# Set development environment variables
export NOMAD_URL="http://10.10.85.1:4646"
export VALID_SECRET="dev-secret-key-change-this-in-production-64-characters-long"
export PORT="8080"

echo "🔥 Starting hot reload server..."
echo "📝 Edit main.go and save to see changes automatically reload"
echo "🌐 Server will be available at http://localhost:8080"
echo "❤️  Health check: http://localhost:8080/health"
echo "🛑 Press Ctrl+C to stop"

# Start Air for hot reload
air
