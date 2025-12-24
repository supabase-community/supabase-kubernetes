#!/bin/bash

# Supabase on Kubernetes Deployment Script
# This script deploys Supabase with CloudNativePG in the correct order

set -e

# Configuration
NAMESPACE=${NAMESPACE:-supabase-kubernetes}
CLUSTER_NAME=${CLUSTER_NAME:-supabase-postgres}
SUPABASE_RELEASE=${SUPABASE_RELEASE:-supabase-kubernetes}
CNPG_OPERATOR_NAMESPACE=${CNPG_OPERATOR_NAMESPACE:-cnpg-system}

echo "🚀 Deploying Supabase with CloudNativePG"
echo "Namespace: $NAMESPACE"
echo "Cluster: $CLUSTER_NAME"
echo "Release: $SUPABASE_RELEASE"
echo ""

# Step 1: Add CloudNativePG Helm repository
echo "📦 Adding CloudNativePG Helm repository..."
helm repo add cnpg https://cloudnative-pg.github.io/charts
helm repo update

# Step 2: Install CloudNativePG operator
echo "🔧 Installing CloudNativePG operator..."
if ! helm list -n $CNPG_OPERATOR_NAMESPACE | grep -q cnpg-operator; then
    helm upgrade --install cnpg-operator \
        cnpg/cloudnative-pg \
        -n $CNPG_OPERATOR_NAMESPACE \
        --create-namespace \
        -f cloudnativepg-values/cnpg-operator/operator-values.yaml
    echo "✅ CloudNativePG operator installed"
else
    echo "✅ CloudNativePG operator already installed"
fi

# Step 3: Wait for operator to be ready
echo "⏳ Waiting for CloudNativePG operator to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/cnpg-controller-manager -n $CNPG_OPERATOR_NAMESPACE

# Step 4: Deploy PostgreSQL cluster
echo "🐘 Deploying PostgreSQL cluster..."
if ! helm list -n $NAMESPACE | grep -q $CLUSTER_NAME; then
    helm upgrade --install $CLUSTER_NAME cnpg/cluster \
        -n $NAMESPACE \
        --create-namespace \
        -f cloudnativepg-values/cnpg-cluster/cluster-values.yaml
    echo "✅ PostgreSQL cluster deployment initiated"
else
    echo "✅ PostgreSQL cluster already deployed"
fi

# Step 5: Wait for PostgreSQL cluster to be ready
echo "⏳ Waiting for PostgreSQL cluster to be ready..."
kubectl wait --for=condition=Ready --timeout=600s cluster/$CLUSTER_NAME -n $NAMESPACE

# Step 6: Deploy Supabase services
echo "🔥 Deploying Supabase services..."
if ! helm list -n $NAMESPACE | grep -q $SUPABASE_RELEASE; then
    helm upgrade --install $SUPABASE_RELEASE charts/supabase \
        -n $NAMESPACE \
        -f cloudnativepg-values/supabase/cloudnativepg-values.yaml \
        --create-namespace
    echo "✅ Supabase services deployment initiated"
else
    echo "✅ Supabase services already deployed"
fi

# Step 7: Wait for migration job to complete
echo "⏳ Waiting for migration job to complete..."
kubectl wait --for=condition=complete --timeout=300s job/supabase-migrations -n $NAMESPACE || true

# Step 8: Show deployment status
echo ""
echo "🎉 Deployment completed!"
echo ""
echo "📊 Checking pod status..."
kubectl get pods -n $NAMESPACE

echo ""
echo "🔍 Useful commands:"
echo "  Check cluster status:    kubectl get cluster -n $NAMESPACE"
echo "  Check migration logs:    kubectl logs job/supabase-migrations -n $NAMESPACE"
echo "  Port-forward Studio:     kubectl port-forward svc/supabase-supabase-studio 3000:3000 -n $NAMESPACE"
echo "  Port-forward Database:   kubectl port-forward svc/$CLUSTER_NAME-rw 5432:5432 -n $NAMESPACE"
echo ""
echo "✨ Happy building with Supabase!"
