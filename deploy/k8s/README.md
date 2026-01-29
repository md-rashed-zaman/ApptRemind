# Kubernetes Skeleton (Optional)

This is a minimal starting point for Kubernetes. It is intentionally small and assumes you bring your own managed infrastructure for:
- Postgres (multiple databases)
- Kafka + Zookeeper (or managed Kafka)
- Redis (gateway rate limiting)
- OTEL collector/Jaeger (optional)

The manifests here show how to run **gateway-service** and **auth-service** in Kubernetes with environment-based configuration.
Extend the same pattern for the remaining services.

## Usage
```bash
kubectl apply -f deploy/k8s/base/namespace.yaml
kubectl apply -f deploy/k8s/base/gateway-deployment.yaml
kubectl apply -f deploy/k8s/base/gateway-service.yaml
kubectl apply -f deploy/k8s/base/auth-deployment.yaml
kubectl apply -f deploy/k8s/base/auth-service.yaml
```

## Image tags
Use `scripts/build-images.sh` and push to your registry. Then update the `image:` fields accordingly.

## Required external dependencies
Point these envs at your managed services:
- `DATABASE_URL`
- `KAFKA_BROKERS`
- `REDIS_ADDR`
- `OTEL_EXPORTER_OTLP_ENDPOINT`

## Notes
These manifests are not production hardening. Add:
- proper secrets management (K8s Secrets, External Secrets, Vault)
- readiness/liveness probes, autoscaling
- network policies + ingress
- TLS + auth for any public endpoints
