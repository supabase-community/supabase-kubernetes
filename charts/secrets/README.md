# Supabase Secrets Management

This directory contains tools for managing secrets for Supabase on Kubernetes.

## Overview

The secret management system provides:
1. A script to generate secrets (`generate-secrets.sh`)
2. A script to create Kubernetes secrets (`create-k8s-secrets.sh`)
3. Configuration templates (`config.yaml.example`, `values.secrets.example.yaml`)
4. A Makefile for common tasks

## Prerequisites

- `bash` shell
- `kubectl` configured to connect to your Kubernetes cluster
- `openssl` for generating random secrets
- `helm` for deploying Supabase
- `jq` for JSON processing (required by some scripts)

## Quick Start

```bash
# Copy the configuration example
cp config.yaml.example config.yaml

# Generate all secrets
make generate-secrets

# Create Kubernetes secrets
make create-k8s-secrets

# Copy the values example and edit it
cp values.secrets.example.yaml values.secrets.yaml

# Deploy Supabase with the generated secrets
make deploy-supabase
```

## Configuration

### config.yaml

Copy `config.yaml.example` to `config.yaml` and customize as needed:

```bash
cp config.yaml.example config.yaml
```

This file stores the generated secrets to avoid regenerating them each time.

### values.secrets.yaml

Copy `values.secrets.example.yaml` to `values.secrets.yaml` and customize:

```bash
cp values.secrets.example.yaml values.secrets.yaml
```

This file configures Supabase Helm chart to use the Kubernetes secrets.

## Secret Generation

Secrets are generated using the `generate-secrets.sh` script. This script creates files in the `secrets` directory.

```bash
# Generate all secrets
./generate-secrets.sh --all

# Generate specific secrets
./generate-secrets.sh --jwt
./generate-secrets.sh --db
./generate-secrets.sh --smtp
./generate-secrets.sh --s3
./generate-secrets.sh --dashboard
./generate-secrets.sh --analytics
```

You can also use the Makefile targets:

```bash
make generate-secrets
make generate-jwt
make generate-db
# ...etc
```

## Kubernetes Secret Creation

After generating secrets, create Kubernetes secrets using `create-k8s-secrets.sh`:

```bash
# Create all secrets
./create-k8s-secrets.sh --all

# Create specific secrets
./create-k8s-secrets.sh --jwt
./create-k8s-secrets.sh --db
# ...etc

# Create the combined secret that works with values.secrets.example.yaml
./create-k8s-secrets.sh --combined
```

Or use the Makefile targets:

```bash
make create-k8s-secrets
make create-k8s-jwt
make create-k8s-db
# ...etc
make create-k8s-combined
```

## Kubernetes Secret Types

The script creates the following secret types:

1. **Individual Secrets** (prefix defaults to "supabase"):
   - `supabase-jwt`: JWT secrets
   - `supabase-db`: Database credentials
   - `supabase-smtp`: SMTP settings
   - `supabase-s3`: S3 credentials
   - `supabase-dashboard`: Dashboard credentials
   - `supabase-analytics`: Analytics API key

2. **Combined Secret** (new format):
   - `supabase-secrets`: All secrets in a single Kubernetes secret
   - This follows a structured naming convention (e.g., `jwt.secret`, `smtp.host`)
   - Works directly with `values.secrets.example.yaml`

## Using Secrets with Supabase

The `values.secrets.example.yaml` file demonstrates how to configure Supabase to use the generated Kubernetes secrets. This file uses the combined secret format for all Supabase services.

The key benefits of using this approach:
1. **Secure**: Secret values are never stored in the Helm values file
2. **Integrated**: Works seamlessly with Kubernetes secret management
3. **Flexible**: Can be used with any secret provider (external vault, etc.)

To deploy Supabase using the generated secrets:

```bash
helm upgrade --install supabase ../supabase \
  --namespace my-namespace \
  --create-namespace \
  -f values.secrets.yaml
```

Or simply use:

```bash
make deploy-supabase NAMESPACE=my-namespace
```

## AWS Integrations

### AWS SES (SMTP) Integration

To configure AWS SES for email sending:

```bash
# Setup AWS SES
make setup-aws-ses
```

This will:
1. Generate SMTP credentials using AWS SES
2. Create an IAM user with SES permissions
3. Verify your email or domain identity
4. Create the necessary Kubernetes secrets

Follow the instructions after running the command to complete setup.

### AWS S3 Integration

To configure AWS S3 for storage:

```bash
# Setup AWS S3
make setup-aws-s3
```

This will:
1. Create an IAM user with S3 permissions
2. Create or use an existing S3 bucket
3. Generate credentials
4. Create the necessary Kubernetes secrets

Follow the instructions after running the command to complete setup.

## SMTP/Email Options

You have several options for configuring SMTP:

1. **Enter existing SMTP credentials**:
   - Provide your own SMTP server details (Gmail, SendGrid, etc.)
   - Fill in host, port, username, password, and sender name
   
2. **Use AWS SES (Simple Email Service)**:
   - Creates a new IAM user with SES permissions
   - Verifies your email or domain identity with SES
   - Configures SMTP settings automatically
   - Recommended for production deployments
   
3. **Generate random credentials**:
   - For development/testing only
   - Won't actually send emails
   
When using AWS SES, you'll need to:
- Verify your sender email or domain
- For domains, add DKIM and verification DNS records
- If in sandbox mode, also verify recipient emails

## S3 Storage Options

You have several options for configuring S3 storage:

1. **Enter existing AWS credentials**:
   - Use existing IAM user and bucket
   - Provide key ID and access key
   
2. **Create new IAM user and S3 bucket**:
   - Creates both a new user and bucket
   - Sets up proper permissions
   - Recommended for new deployments
   
3. **Create new IAM user for existing S3 bucket**:
   - Creates a new user with access to your bucket
   - Useful when bucket already exists
   
4. **Generate random credentials**:
   - For development/testing only
   - Not connected to AWS

## Customization

You can customize the behavior using environment variables:

```bash
# Kubernetes namespace
export NAMESPACE=supabase

# Secret name prefix
export SECRET_PREFIX=myapp

# Create a combined secret (yes/no)
export CREATE_COMBINED=true

# Then run the scripts
./create-k8s-secrets.sh --all
```

## Maintenance

- **Regenerating Secrets**: To regenerate specific secrets, delete them from `config.yaml` and run the generate script again.
- **Updating Kubernetes Secrets**: Run `make create-k8s-secrets` after changing secret values.
- **Redeploying**: Run `make deploy-supabase` after updating secrets. 