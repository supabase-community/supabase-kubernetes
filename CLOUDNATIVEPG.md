# CloudNativePG Integration Guide

This guide covers the production-grade PostgreSQL deployment using CloudNativePG for Supabase on Kubernetes.

## Overview

CloudNativePG provides enterprise-grade PostgreSQL management with:

- **High Availability**: Multi-replica clusters with automatic failover
- **Automated Backups**: Point-in-time recovery capabilities
- **Connection Pooling**: Built-in PgBouncer integration
- **Monitoring**: Prometheus metrics and observability
- **Rolling Updates**: Zero-downtime PostgreSQL updates

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                          │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │   Supabase      │  │   CloudNativePG │  │    MinIO        │ │
│  │   Services      │  │   PostgreSQL    │  │   Storage       │ │
│  │                 │  │   Cluster       │  │                 │ │
│  │ • Auth          │  │                 │  │ • S3 Backend    │ │
│  │ • Storage       │  │ • Primary       │  │ • File Storage  │ │
│  │ • Realtime      │  │ • Replicas      │  │                 │ │
│  │ • Functions     │  │ • Pooler        │  │                 │ │
│  │ • Kong          │  │                 │  │                 │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## Prerequisites

- Kubernetes cluster (1.21+)
- Helm 3.x
- CloudNativePG operator

## Installation Steps

### 1. Install CloudNativePG Operator

```bash
# Add CloudNativePG Helm repository
helm repo add cnpg https://cloudnative-pg.github.io/charts
helm repo update

# Install CloudNativePG operator
helm install cnpg-operator cnpg/cloudnative-pg -n cnpg-system --create-namespace
```

### 2. Deploy PostgreSQL Cluster

```bash
# Deploy PostgreSQL cluster using our configuration
helm install postgres-cluster cnpg/cluster \
  -f values/cloudnativepg/cnpg-cluster/cluster-values.yaml \
  -n supabase-dev --create-namespace
```

### 3. Deploy Supabase Services

```bash
# Install Supabase with CloudNativePG configuration
helm install supabase charts/supabase \
  -f values/supabase/values-cloudnativepg.yaml \
  -n supabase-dev
```

### 4. Automated Deployment

Use the provided script for one-command deployment:

```bash
chmod +x scripts/deploy-supabase.sh
./scripts/deploy-supabase.sh
```

## Configuration Files

### CloudNativePG Cluster Configuration

**File**: `values/cloudnativepg/cnpg-cluster/cluster-values.yaml`

Key configurations:
- **Image**: `supabase/postgres:17.5.1.024-orioledb`
- **Storage**: 100Gi with gp3 storage class
- **Resources**: 4Gi memory, 2 CPU cores
- **Extensions**: All Supabase-required extensions pre-loaded
- **Backup**: S3-compatible backup configuration

### Supabase Integration Configuration

**File**: `values/supabase/values-cloudnativepg.yaml`

Key features:
- **Database**: Disabled internal PostgreSQL, uses CloudNativePG
- **High Availability**: 2+ replicas for critical services
- **Secrets**: Automatic integration with CloudNativePG secrets
- **MinIO**: S3-compatible storage backend
- **Migration**: Comprehensive database setup job

## High Availability Features

### Replica Configuration

```yaml
# Enable HA in values/supabase/values-cloudnativepg.yaml
global:
  highAvailability:
    enabled: true
    minReplicas: 2
    maxReplicas: 10
```

### Anti-Affinity Rules

Services are automatically spread across nodes:

```yaml
auth:
  replicaCount: 2
  affinity:
    podAntiAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchLabels:
              app.kubernetes.io/component: auth
          topologyKey: kubernetes.io/hostname
```

### Pod Disruption Budgets

Automatic PDB creation ensures service availability during maintenance:

```yaml
global:
  highAvailability:
    podDisruptionBudget:
      enabled: true
      maxUnavailable: 1
```

## Migration Job

The migration job handles complete Supabase database setup:

### Features

- **Schema Setup**: All Supabase schemas and extensions
- **Role Management**: Proper user roles and permissions
- **JWT Configuration**: Automatic JWT secret configuration
- **Extension Installation**: All required PostgreSQL extensions
- **Error Handling**: Robust error handling for known issues

### Monitoring Migration

```bash
# Check migration job status
kubectl get jobs -n supabase-dev
kubectl logs job/supabase-migrations -n supabase-dev
```

## Monitoring and Observability

### PostgreSQL Metrics

CloudNativePG provides built-in Prometheus metrics:

```bash
# Check cluster status
kubectl get cluster -n supabase-dev

# View cluster details
kubectl describe cluster supabase-postgres-cluster -n supabase-dev
```

### Service Health

```bash
# Check all pods
kubectl get pods -n supabase-dev

# Check service endpoints
kubectl get svc -n supabase-dev
```

## Backup and Recovery

### Automated Backups

Configured in `cluster-values.yaml`:

```yaml
backups:
  enabled: true
  destinationPath: "s3://supabase-backups/postgres"
  scheduledBackups:
    - name: daily-backup
      schedule: "0 2 * * *"  # Daily at 2 AM
  retentionPolicy: "30d"
```

### Point-in-Time Recovery

CloudNativePG supports PITR for disaster recovery scenarios.

## Scaling

### Horizontal Scaling

```bash
# Scale auth service
kubectl scale deployment supabase-supabase-auth --replicas=3 -n supabase-dev

# Scale PostgreSQL replicas
kubectl patch cluster supabase-postgres-cluster \
  --type='merge' -p='{"spec":{"instances":3}}' -n supabase-dev
```

### Vertical Scaling

Update resource limits in configuration files and upgrade:

```bash
helm upgrade supabase charts/supabase \
  -f values/supabase/values-cloudnativepg.yaml \
  -n supabase-dev
```

## Troubleshooting

### Common Issues

#### Migration Job Fails

```bash
# Check migration logs
kubectl logs job/supabase-migrations -n supabase-dev

# Common fixes:
# 1. Ensure PostgreSQL cluster is ready
kubectl get cluster -n supabase-dev
# 2. Verify database connectivity
kubectl exec -it supabase-postgres-cluster-1 -n supabase-dev -- pg_isready
```

#### Database Connection Issues

```bash
# Check PostgreSQL cluster status
kubectl get cluster supabase-postgres-cluster -n supabase-dev

# Test connectivity
kubectl exec -it supabase-postgres-cluster-1 -n supabase-dev -- \
  psql -U postgres -c "SELECT version();"
```

#### Service Startup Issues

```bash
# Check service logs
kubectl logs deployment/supabase-supabase-auth -n supabase-dev

# Verify secrets
kubectl get secret supabase-postgres-cluster-superuser -n supabase-dev -o yaml
```

### Performance Tuning

#### PostgreSQL Configuration

Adjust in `cluster-values.yaml`:

```yaml
postgresql:
  parameters:
    max_connections: "200"
    shared_buffers: "256MB"
    effective_cache_size: "1GB"
    work_mem: "4MB"
```

#### Connection Pooling

PgBouncer is configured automatically:

```yaml
poolers:
  - enabled: true
    name: pooler-rw
    instances: 4
    pgbouncer:
      poolMode: transaction
      parameters:
        max_client_conn: "2000"
        default_pool_size: "50"
```

## Security Considerations

### Network Policies

Implement network policies to restrict traffic between services.

### Secret Management

- Use external secret management systems
- Rotate JWT secrets regularly
- Enable encryption at rest

### Database Security

- Enable SSL/TLS for database connections
- Use strong passwords
- Implement proper role-based access control

## Production Checklist

- [ ] **SSL/TLS**: Configure SSL for all endpoints
- [ ] **Monitoring**: Set up Prometheus and Grafana
- [ ] **Backups**: Configure S3 backup storage
- [ ] **Secrets**: Use external secret management
- [ ] **Network**: Implement network policies
- [ ] **Resources**: Set appropriate resource limits
- [ ] **Storage**: Use high-performance storage classes
- [ ] **Scaling**: Configure HPA and VPA
- [ ] **Disaster Recovery**: Test backup and restore procedures

## Support

For CloudNativePG-specific issues:
- [CloudNativePG Documentation](https://cloudnative-pg.io/documentation/)
- [CloudNativePG GitHub](https://github.com/cloudnative-pg/cloudnative-pg)

For Supabase integration issues:
- Open an issue in this repository
- Check the troubleshooting section above
