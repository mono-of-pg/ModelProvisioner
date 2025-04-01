# ModelProvisioner

This application dynamically configures [LiteLLM](https://github.com/BerriAI/litellm) by adding and removing model deployments based on the availability of models on locally hosted backends with OpenAI-compatible `/models` endpoints. It ensures that models from endpoints not configured in the ConfigMap are protected from deletion.

## Features

- **Stateless**: Stores no local state; relies on backend and LiteLLM data each cycle.
- **Configurable**: Uses Kubernetes ConfigMaps and Secrets for configuration.
- **Rootless**: Runs in a minimal, secure `distroless` container as a non-root user.
- **Periodic Updates**: Checks backends every 60 seconds (configurable).
- **Local Focus**: Only manages locally hosted providers with `/models` endpoints.
- **Protection of Unconfigured Models**: Models from endpoints not in the ConfigMap are ignored and protected from deletion.

## Prerequisites

- A Kubernetes cluster.
- LiteLLM deployed n DB mode with an accessible API (e.g., `http://litellm.api.svc:4000`).
- Locally hosted backend services (e.g., Ollama) with OpenAI-compatible `/models` endpoints.
- Docker registry access to push the built image.

## Files

- `main.go`: Application source code.
- `go.mod`: Go module definition.
- `Dockerfile`: Docker image build instructions.
- `k8s/configmap.yaml`: Example ConfigMap.
- `k8s/secret.yaml`: Example Secret (edit with real keys).
- `k8s/deployment.yaml`: Kubernetes Deployment manifest.

## Deployment Steps

1. **Build and Push the Docker Image**

   ```bash
   docker build -t myregistry/modelprovisioner:latest .
   docker push myregistry/modelprovisioner:latest
