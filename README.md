# ModelProvisioner

This application dynamically configures [LiteLLM](https://github.com/BerriAI/litellm) by adding and removing model deployments based on the availability of models on locally hosted backends with OpenAI-compatible `/models` endpoints. It ensures that models from endpoints not configured in the ConfigMap are protected from deletion.

## Features

- **Stateless**: Stores no local state; relies on backend and LiteLLM data each cycle.
- **Configurable**: Uses Kubernetes ConfigMaps and Secrets for configuration.
- **Rootless**: Runs in a minimal, secure `distroless` container as a non-root user.
- **Periodic Updates**: Checks backends every 60 seconds (configurable).
- **Local Focus**: Only manages locally hosted providers with `/models` endpoints.
- **Protection of Unconfigured Models**: Models from endpoints not in the ConfigMap are ignored and protected from deletion.
- **Capability Discovery**: Optionally enable discovery for each backend to automatically test and set model capabilities like tool use and vision.
- **Regex Overrides**: Define regex patterns to manually set or override model capabilities, providing precise control over the configuration.
- **Regex Filter**: Filter models by regex to only add matching models.

## Prerequisites

- A Kubernetes cluster.
- LiteLLM deployed in DB mode and STORE_MODEL_IN_DB set to true.
- Locally hosted backend services (e.g., Ollama) with OpenAI-compatible `/models` endpoints.
- Docker registry access to push the built image (or use the prebuilt image from ghcr).

## Configuration
The ModelProvisioner is configured via a Kubernetes ConfigMap and Secrets. The ConfigMap defines the LiteLLM URL and the backends to manage, including the features for capability discovery and overrides as well as regex filtering.

- Discovery: Add a discovery: true field to each backend where you want to enable capability discovery. When enabled, the provisioner will test each new model for tool use and vision capabilities by sending requests to the backend's /chat/completions endpoint.
- Overrides: Add an overrides section to each backend, containing a list of regex patterns and their associated capabilities. This allows you to manually set or override capabilities for models matching the regex patterns.
- Efficient Discovery: Capability discovery is only performed for models newly added to LiteLLM, avoiding unnecessary queries for existing models.

## Note
Enabling discovery will send test requests to the backend for each new model. Ensure that your backend can handle these requests without hitting rate limits or incurring excessive costs. Refer to `k8s/configmap.yaml` for an example configuration and update it with your backend details, including the discovery and overrides fields as needed.

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
