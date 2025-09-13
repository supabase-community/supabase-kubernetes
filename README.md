# Supabase Kubernetes

This repository contains the charts to deploy a [Supabase](https://github.com/supabase/supabase) instance inside a Kubernetes cluster using Helm 3, with **production-ready PostgreSQL support** using CloudNativePG.

For any information regarding Supabase itself you can refer to the [official documentation](https://supabase.io/docs).

## What's Supabase ?

Supabase is an open source Firebase alternative. We're building the features of Firebase using enterprise-grade open source tools.

## Features

✅ **Production-Ready PostgreSQL**: Optional CloudNativePG integration for enterprise-grade PostgreSQL management  
✅ **High Availability**: Multi-replica deployments with anti-affinity rules and pod disruption budgets  
✅ **Automated Database Setup**: Comprehensive migration job that mimics official Supabase `migrate.sh`  
✅ **Horizontal Pod Autoscaling**: Automatic scaling based on CPU/memory utilization  
✅ **Multi-Zone Support**: Topology spread constraints for zone-aware deployments  
✅ **Backward Compatibility**: Existing embedded PostgreSQL option still available  

## Deployment Options

### Standard Deployment (Default)
Uses the embedded PostgreSQL container - suitable for development and testing:
```bash
helm install supabase charts/supabase
```

### Production Deployment with CloudNativePG
For production workloads with enterprise-grade PostgreSQL:

1. **Install CloudNativePG operator:**
   ```bash
   helm repo add cnpg https://cloudnative-pg.github.io/charts
   helm install cnpg-operator cnpg/cloudnative-pg -n cnpg-system --create-namespace
   ```

2. **Deploy PostgreSQL cluster:**
   ```bash
   helm install postgres-cluster cnpg/cluster \
     -f values/cloudnativepg/cnpg-cluster/cluster-values.yaml \
     -n supabase-dev --create-namespace
   ```

3. **Deploy Supabase with CloudNativePG:**
   ```bash
   helm install supabase charts/supabase \
     -f values/supabase/values-cloudnativepg.yaml \
     -n supabase-dev
   ```

### Quick Deployment Script
Use the automated deployment script:
```bash
chmod +x scripts/deploy-supabase.sh
./scripts/deploy-supabase.sh
```

## How to use ?

You can find the documentation inside the [chart directory](./charts/supabase/README.md)

# Roadmap

- [x] Multi-node Support ✅
- [x] High Availability ✅
- [x] Production-grade PostgreSQL ✅

## Support

This project is supported by the community and not officially supported by Supabase. Please do not create any issues on the official Supabase repositories if you face any problems using this project, but rather open an issue on this repository.

## Contributing

You can contribute to this project by forking this repository and opening a pull request.

When you're ready to publish your chart on the `main` branch, you'll have to execute `sh build.sh` to package the charts and generate the Helm manifest.

## License

[Apache 2.0 License.](./LICENSE)
