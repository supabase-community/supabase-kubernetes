# Makefile for Supabase secret management

# Default namespace for Kubernetes operations
NAMESPACE ?= default
# Default Helm release name
RELEASE_NAME ?= supabase
# Default Helm chart path (relative to this directory)
CHART_PATH ?= ../supabase

.PHONY: generate-secrets create-k8s-secrets deploy-supabase help setup-aws-s3 setup-aws-ses

# Generate all secrets
generate-secrets:
	@echo "Generating Supabase secrets..."
	./generate-secrets.sh --all

# Generate specific secrets
generate-jwt:
	./generate-secrets.sh --jwt

generate-db:
	./generate-secrets.sh --db

generate-smtp:
	./generate-secrets.sh --smtp

generate-s3:
	./generate-secrets.sh --s3

generate-dashboard:
	./generate-secrets.sh --dashboard

generate-analytics:
	./generate-secrets.sh --analytics

# Create Kubernetes secrets
create-k8s-secrets:
	@echo "Creating Kubernetes secrets in namespace $(NAMESPACE)..."
	NAMESPACE=$(NAMESPACE) ./create-k8s-secrets.sh --all

# Create specific Kubernetes secrets
create-k8s-jwt:
	NAMESPACE=$(NAMESPACE) ./create-k8s-secrets.sh --jwt

create-k8s-db:
	NAMESPACE=$(NAMESPACE) ./create-k8s-secrets.sh --db

create-k8s-smtp:
	NAMESPACE=$(NAMESPACE) ./create-k8s-secrets.sh --smtp

create-k8s-s3:
	NAMESPACE=$(NAMESPACE) ./create-k8s-secrets.sh --s3

create-k8s-dashboard:
	NAMESPACE=$(NAMESPACE) ./create-k8s-secrets.sh --dashboard

create-k8s-analytics:
	NAMESPACE=$(NAMESPACE) ./create-k8s-secrets.sh --analytics

create-k8s-combined:
	NAMESPACE=$(NAMESPACE) ./create-k8s-secrets.sh --combined

# Deploy Supabase with the generated secrets
deploy-supabase:
	@echo "Deploying Supabase to namespace $(NAMESPACE) using generated secrets..."
	@if [ ! -f values.secrets.yaml ]; then \
		echo "Error: values.secrets.yaml not found. Please create it based on values.secrets.example.yaml"; \
		exit 1; \
	fi
	helm upgrade --install $(RELEASE_NAME) $(CHART_PATH) \
		--namespace $(NAMESPACE) \
		--create-namespace \
		-f values.secrets.yaml

# Setup AWS S3 for storage
setup-aws-s3:
	@echo "Setting up AWS S3 for Supabase Storage..."
	./generate-secrets.sh --s3
	NAMESPACE=$(NAMESPACE) ./create-k8s-secrets.sh --s3
	NAMESPACE=$(NAMESPACE) ./create-k8s-secrets.sh --combined
	@echo "S3 setup complete. Now you need to:"
	@echo "1. Copy values.secrets.example.yaml to values.secrets.yaml"
	@echo "2. Edit values.secrets.yaml to set your S3 bucket name and region"
	@echo "3. Run: make deploy-supabase"

# Setup AWS SES for SMTP
setup-aws-ses:
	@echo "Setting up AWS SES for Supabase Auth emails..."
	./generate-secrets.sh --smtp
	NAMESPACE=$(NAMESPACE) ./create-k8s-secrets.sh --smtp
	NAMESPACE=$(NAMESPACE) ./create-k8s-secrets.sh --combined
	@echo "SES setup complete. Now you need to:"
	@echo "1. Copy values.secrets.example.yaml to values.secrets.yaml"
	@echo "2. Verify your email or domain identity with AWS SES"
	@echo "3. Run: make deploy-supabase"
	@echo "NOTE: If using a new SES account, you may be in the SES sandbox"
	@echo "      and can only send to verified email addresses."

# Generate all secrets, create Kubernetes secrets, and deploy Supabase
all: generate-secrets create-k8s-secrets

# Display help information
help:
	@echo "Supabase Secret Management Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  generate-secrets      Generate all secrets"
	@echo "  create-k8s-secrets    Create all Kubernetes secrets"
	@echo "  deploy-supabase       Deploy Supabase with the generated secrets"
	@echo "  setup-aws-s3          Setup AWS S3 for Supabase Storage"
	@echo "  setup-aws-ses         Setup AWS SES for Supabase Auth emails"
	@echo "  all                   Generate secrets, create K8s secrets, and deploy"
	@echo "  help                  Display this help message"
	@echo ""
	@echo "Individual secret generation targets:"
	@echo "  generate-jwt, generate-db, generate-smtp, generate-s3,"
	@echo "  generate-dashboard, generate-analytics"
	@echo ""
	@echo "Individual Kubernetes secret creation targets:"
	@echo "  create-k8s-jwt, create-k8s-db, create-k8s-smtp, create-k8s-s3,"
	@echo "  create-k8s-dashboard, create-k8s-analytics, create-k8s-combined"
	@echo ""
	@echo "Environment variables:"
	@echo "  NAMESPACE      Kubernetes namespace (default: default)"
	@echo "  RELEASE_NAME   Helm release name (default: supabase)"
	@echo "  CHART_PATH     Path to Helm chart (default: ../supabase)"

# Default target
.DEFAULT_GOAL := help 