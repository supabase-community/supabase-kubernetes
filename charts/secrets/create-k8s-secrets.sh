#!/bin/bash
# Script to create Kubernetes secrets for Supabase

# Exit on any error
set -e

# Default values
NAMESPACE=${NAMESPACE:-default}
SECRET_PREFIX=${SECRET_PREFIX:-supabase}
INPUT_DIR="secrets"
CREATE_COMBINED=${CREATE_COMBINED:-true}

# Create JWT secret
create_jwt_secret() {
  echo "Creating JWT secret in Kubernetes..."
  
  # Check if required files exist
  if [ ! -f "$INPUT_DIR/jwt_anon_key" ] || [ ! -f "$INPUT_DIR/jwt_service_key" ] || [ ! -f "$INPUT_DIR/jwt_secret" ]; then
    echo "Error: JWT secret files not found. Run generate-secrets.sh first."
    exit 1
  fi
  
  # Create the secret
  kubectl create secret generic "${SECRET_PREFIX}-jwt" \
    --from-file=anonKey="$INPUT_DIR/jwt_anon_key" \
    --from-file=serviceKey="$INPUT_DIR/jwt_service_key" \
    --from-file=secret="$INPUT_DIR/jwt_secret" \
    --namespace="$NAMESPACE" \
    --dry-run=client -o yaml | kubectl apply -f -
  
  echo "JWT secret created in namespace $NAMESPACE."
}

# Create DB secret
create_db_secret() {
  echo "Creating database secret in Kubernetes..."
  
  # Check if required files exist
  if [ ! -f "$INPUT_DIR/db_username" ] || [ ! -f "$INPUT_DIR/db_password" ] || [ ! -f "$INPUT_DIR/db_database" ]; then
    echo "Error: Database secret files not found. Run generate-secrets.sh first."
    exit 1
  fi
  
  # Create the secret
  kubectl create secret generic "${SECRET_PREFIX}-db" \
    --from-file=username="$INPUT_DIR/db_username" \
    --from-file=password="$INPUT_DIR/db_password" \
    --from-file=database="$INPUT_DIR/db_database" \
    --namespace="$NAMESPACE" \
    --dry-run=client -o yaml | kubectl apply -f -
  
  echo "Database secret created in namespace $NAMESPACE."
}

# Create analytics secret
create_analytics_secret() {
  echo "Creating analytics secret in Kubernetes..."
  
  # Check if required file exists
  if [ ! -f "$INPUT_DIR/analytics_api_key" ]; then
    echo "Error: Analytics secret file not found. Run generate-secrets.sh first."
    exit 1
  fi
  
  # Create the secret
  kubectl create secret generic "${SECRET_PREFIX}-analytics" \
    --from-file=apiKey="$INPUT_DIR/analytics_api_key" \
    --namespace="$NAMESPACE" \
    --dry-run=client -o yaml | kubectl apply -f -
  
  echo "Analytics secret created in namespace $NAMESPACE."
}

# Create SMTP secret
create_smtp_secret() {
  echo "Creating SMTP secret in Kubernetes..."
  
  # Check if required files exist
  if [ ! -f "$INPUT_DIR/smtp_username" ] || [ ! -f "$INPUT_DIR/smtp_password" ] || [ ! -f "$INPUT_DIR/smtp_host" ] || [ ! -f "$INPUT_DIR/smtp_port" ] || [ ! -f "$INPUT_DIR/smtp_sender_name" ]; then
    echo "Error: SMTP secret files not found. Run generate-secrets.sh first."
    exit 1
  fi
  
  # Create the secret
  kubectl create secret generic "${SECRET_PREFIX}-smtp" \
    --from-file=username="$INPUT_DIR/smtp_username" \
    --from-file=password="$INPUT_DIR/smtp_password" \
    --from-file=host="$INPUT_DIR/smtp_host" \
    --from-file=port="$INPUT_DIR/smtp_port" \
    --from-file=sender_name="$INPUT_DIR/smtp_sender_name" \
    --namespace="$NAMESPACE" \
    --dry-run=client -o yaml | kubectl apply -f -
  
  echo "SMTP secret created in namespace $NAMESPACE."
}

# Create dashboard secret
create_dashboard_secret() {
  echo "Creating dashboard secret in Kubernetes..."
  
  # Check if required files exist
  if [ ! -f "$INPUT_DIR/dashboard_username" ] || [ ! -f "$INPUT_DIR/dashboard_password" ]; then
    echo "Error: Dashboard secret files not found. Run generate-secrets.sh first."
    exit 1
  fi
  
  # Create the secret
  kubectl create secret generic "${SECRET_PREFIX}-dashboard" \
    --from-file=username="$INPUT_DIR/dashboard_username" \
    --from-file=password="$INPUT_DIR/dashboard_password" \
    --namespace="$NAMESPACE" \
    --dry-run=client -o yaml | kubectl apply -f -
  
  echo "Dashboard secret created in namespace $NAMESPACE."
}

# Create S3 secret
create_s3_secret() {
  echo "Creating S3 secret in Kubernetes..."
  
  # Check if required files exist
  if [ ! -f "$INPUT_DIR/s3_key_id" ] || [ ! -f "$INPUT_DIR/s3_access_key" ]; then
    echo "Error: S3 secret files not found. Run generate-secrets.sh first."
    exit 1
  fi
  
  # Create the secret
  kubectl create secret generic "${SECRET_PREFIX}-s3" \
    --from-file=keyId="$INPUT_DIR/s3_key_id" \
    --from-file=accessKey="$INPUT_DIR/s3_access_key" \
    --namespace="$NAMESPACE" \
    --dry-run=client -o yaml | kubectl apply -f -
  
  echo "S3 secret created in namespace $NAMESPACE."
}

# Create combined secret with all values
create_combined_secret() {
  echo "Creating combined secret in Kubernetes..."
  
  # Check if all required files exist
  local missing_files=false
  local required_files=(
    "jwt_anon_key" "jwt_service_key" "jwt_secret"
    "db_username" "db_password" "db_database"
    "analytics_api_key"
    "smtp_username" "smtp_password" "smtp_host" "smtp_port" "smtp_sender_name"
    "dashboard_username" "dashboard_password"
    "s3_key_id" "s3_access_key"
  )
  
  for file in "${required_files[@]}"; do
    if [ ! -f "$INPUT_DIR/$file" ]; then
      echo "Warning: File $INPUT_DIR/$file not found."
      missing_files=true
    fi
  done
  
  if [ "$missing_files" = "true" ]; then
    echo "Some secret files are missing. Run generate-secrets.sh first to create all secrets."
    read -p "Continue anyway? (y/n): " continue_anyway
    if [[ ! "$continue_anyway" =~ ^[Yy]$ ]]; then
      exit 1
    fi
  fi
  
  # Create the combined secret with properly structured keys
  # We use the structure jwt.anonKey instead of just anonKey to match values.secrets.example.yaml
  kubectl create secret generic "supabase-secrets" \
    --from-file=jwt.anonKey="$INPUT_DIR/jwt_anon_key" \
    --from-file=jwt.serviceKey="$INPUT_DIR/jwt_service_key" \
    --from-file=jwt.secret="$INPUT_DIR/jwt_secret" \
    --from-file=db.username="$INPUT_DIR/db_username" \
    --from-file=db.password="$INPUT_DIR/db_password" \
    --from-file=db.database="$INPUT_DIR/db_database" \
    --from-file=analytics.apiKey="$INPUT_DIR/analytics_api_key" \
    --from-file=smtp.username="$INPUT_DIR/smtp_username" \
    --from-file=smtp.password="$INPUT_DIR/smtp_password" \
    --from-file=smtp.host="$INPUT_DIR/smtp_host" \
    --from-file=smtp.port="$INPUT_DIR/smtp_port" \
    --from-file=smtp.sender_name="$INPUT_DIR/smtp_sender_name" \
    --from-file=dashboard.username="$INPUT_DIR/dashboard_username" \
    --from-file=dashboard.password="$INPUT_DIR/dashboard_password" \
    --from-file=s3.keyId="$INPUT_DIR/s3_key_id" \
    --from-file=s3.accessKey="$INPUT_DIR/s3_access_key" \
    --namespace="$NAMESPACE" \
    --dry-run=client -o yaml | kubectl apply -f -
  
  # Check if S3 bucket and region info exists and add to the secret
  if [ -f "$INPUT_DIR/s3_bucket_name" ] && [ -f "$INPUT_DIR/s3_region" ]; then
    # Get bucket name and region
    S3_BUCKET=$(cat "$INPUT_DIR/s3_bucket_name")
    S3_REGION=$(cat "$INPUT_DIR/s3_region")
    
    # Base64 encode the values first (required for Kubernetes secrets)
    S3_BUCKET_B64=$(echo -n "$S3_BUCKET" | base64)
    S3_REGION_B64=$(echo -n "$S3_REGION" | base64)
    
    # Add these to the secret as config values
    kubectl get secret supabase-secrets -n "$NAMESPACE" -o json | \
    jq --arg bucket_b64 "$S3_BUCKET_B64" --arg region_b64 "$S3_REGION_B64" \
      '.data["s3.bucketName"] = $bucket_b64 | .data["s3.region"] = $region_b64' | \
    kubectl apply -f -
    
    echo "Added S3 bucket information to combined secret."
  fi
  
  echo "Combined secret 'supabase-secrets' created in namespace $NAMESPACE."
  echo "This secret can be used with the values.secrets.example.yaml configuration."
}

# Create all secrets
create_all_secrets() {
  create_jwt_secret
  create_db_secret
  create_analytics_secret
  create_smtp_secret
  create_dashboard_secret
  create_s3_secret
  
  # Create the combined secret if requested
  if [ "$CREATE_COMBINED" = "true" ]; then
    create_combined_secret
  fi
  
  echo "All Kubernetes secrets created in namespace $NAMESPACE."
}

# Display help
show_help() {
  echo "Usage: $0 [option]"
  echo "Options:"
  echo "  --all             Create all Kubernetes secrets"
  echo "  --jwt             Create JWT secret"
  echo "  --db              Create database secret"
  echo "  --analytics       Create analytics secret"
  echo "  --smtp            Create SMTP secret"
  echo "  --dashboard       Create dashboard secret"
  echo "  --s3              Create S3 secret"
  echo "  --combined        Create a combined secret for use with values.secrets.example.yaml"
  echo "  --help            Show this help message"
  echo
  echo "Environment variables:"
  echo "  NAMESPACE         Kubernetes namespace (default: default)"
  echo "  SECRET_PREFIX     Prefix for secret names (default: supabase)"
  echo "  INPUT_DIR         Directory containing secret files (default: secrets)"
  echo "  CREATE_COMBINED   Whether to create a combined secret (default: true)"
}

# Parse command line arguments
if [ $# -eq 0 ]; then
  # No arguments, create all secrets
  create_all_secrets
else
  case "$1" in
    --all)
      create_all_secrets
      ;;
    --jwt)
      create_jwt_secret
      ;;
    --db)
      create_db_secret
      ;;
    --analytics)
      create_analytics_secret
      ;;
    --smtp)
      create_smtp_secret
      ;;
    --dashboard)
      create_dashboard_secret
      ;;
    --s3)
      create_s3_secret
      ;;
    --combined)
      create_combined_secret
      ;;
    --help)
      show_help
      ;;
    *)
      echo "Unknown option: $1"
      show_help
      exit 1
      ;;
  esac
fi

exit 0 