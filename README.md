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
- LiteLLM deployed in DB mode and STORE_MODEL_IN_DB set to true.
- Locally hosted backend services (e.g., Ollama) with OpenAI-compatible `/models` endpoints.
- Docker registry access to push the built image (or use the prebuilt image from ghcr).

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

2. **Modify Deployment yaml**

   Change image to your repo.
   If needed, enable debug by setting the environment variable to true.

3. **Deploy on Kubernetes**

   ```bash
   cd k8s
   kubectl create ns <your_namespace>
   kubectl -n <your_namespace> create -f .

4. **Watch ModelProvisioner doing it's magic***

   ```bash
   kubectl -n <your_namespace> logs deployment/modelprovisioner -f
