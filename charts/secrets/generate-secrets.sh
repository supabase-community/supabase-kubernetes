#!/bin/bash
# Script to generate secrets for Supabase on Kubernetes

# Exit on any error
set -e

# Import configuration utilities
source ./config_utils.sh

# Output directory for secrets
OUTPUT_DIR="secrets"
mkdir -p "$OUTPUT_DIR"

# Helper function to prompt for input with defaults
prompt_with_default() {
  local prompt="$1"
  local default="$2"
  local var_name="$3"
  local secure="$4"
  
  if [ "$secure" = "true" ]; then
    read -p "$prompt [$default]: " -s $var_name
    echo
  else
    read -p "$prompt [$default]: " $var_name
  fi
  
  # If input is empty, use the default
  eval "$var_name=\${$var_name:-$default}"
}

# Generate JWT secrets
generate_jwt_secrets() {
  echo "Generating JWT secrets..."
  
  # Generate a random JWT secret (at least 32 characters)
  JWT_SECRET=$(openssl rand -base64 32)
  echo -n "$JWT_SECRET" > "$OUTPUT_DIR/jwt_secret"
  
  # Create anon key (JWT token)
  # This is a simple example - in production you would use proper JWT signing
  # The example in values.example.yaml uses hardcoded values that are not secure
  ANON_PAYLOAD='{
    "role": "anon",
    "iss": "supabase",
    "iat": 1641769200,
    "exp": 1799535600
  }'
  ANON_HEADER='{
    "alg": "HS256",
    "typ": "JWT"
  }'
  
  # Base64 encode the header and payload
  ANON_HEADER_BASE64=$(echo -n "$ANON_HEADER" | base64 | tr -d '\n' | tr -d '=' | tr '+/' '-_')
  ANON_PAYLOAD_BASE64=$(echo -n "$ANON_PAYLOAD" | base64 | tr -d '\n' | tr -d '=' | tr '+/' '-_')
  
  # Sign the token
  ANON_SIGNATURE=$(echo -n "${ANON_HEADER_BASE64}.${ANON_PAYLOAD_BASE64}" | 
    openssl dgst -sha256 -hmac "$JWT_SECRET" -binary | 
    base64 | tr -d '\n' | tr -d '=' | tr '+/' '-_')
  
  # Create the token
  ANON_TOKEN="${ANON_HEADER_BASE64}.${ANON_PAYLOAD_BASE64}.${ANON_SIGNATURE}"
  echo -n "$ANON_TOKEN" > "$OUTPUT_DIR/jwt_anon_key"
  
  # Create service key (JWT token) - similar process as anon key but with different payload
  SERVICE_PAYLOAD='{
    "role": "service_role",
    "iss": "supabase",
    "iat": 1641769200,
    "exp": 1799535600
  }'
  
  # Base64 encode the payload (reuse header)
  SERVICE_PAYLOAD_BASE64=$(echo -n "$SERVICE_PAYLOAD" | base64 | tr -d '\n' | tr -d '=' | tr '+/' '-_')
  
  # Sign the token
  SERVICE_SIGNATURE=$(echo -n "${ANON_HEADER_BASE64}.${SERVICE_PAYLOAD_BASE64}" | 
    openssl dgst -sha256 -hmac "$JWT_SECRET" -binary | 
    base64 | tr -d '\n' | tr -d '=' | tr '+/' '-_')
  
  # Create the token
  SERVICE_TOKEN="${ANON_HEADER_BASE64}.${SERVICE_PAYLOAD_BASE64}.${SERVICE_SIGNATURE}"
  echo -n "$SERVICE_TOKEN" > "$OUTPUT_DIR/jwt_service_key"
  
  echo "JWT secrets generated."
}

# Generate database secrets
generate_db_secrets() {
  echo "Generating database secrets..."
  
  # Prompt for database username with config-based default
  if [ -z "$DB_USERNAME" ]; then
    prompt_with_config "Enter database username" "database.username" "postgres" "false" DB_USERNAME
  fi
  echo -n "$DB_USERNAME" > "$OUTPUT_DIR/db_username"
  
  # Get password from config or generate
  if [ -z "$DB_PASSWORD" ]; then
    prompt_with_config "Enter database password" "database.password" "" "true" DB_PASSWORD "true"
    if [ -z "$DB_PASSWORD" ]; then
      DB_PASSWORD=$(openssl rand -base64 12)
      set_config_value "database.password" "$DB_PASSWORD"
      echo "Generated random database password: $DB_PASSWORD"
    fi
  fi
  echo -n "$DB_PASSWORD" > "$OUTPUT_DIR/db_password"
  
  # Prompt for database name with config-based default
  if [ -z "$DB_NAME" ]; then
    prompt_with_config "Enter database name" "database.name" "postgres" "false" DB_NAME
  fi
  echo -n "$DB_NAME" > "$OUTPUT_DIR/db_database"
  
  echo "Database secrets generated."
}

# Generate analytics secrets
generate_analytics_secrets() {
  echo "Generating analytics secrets..."
  
  # Get API key from config or generate
  if [ -z "$ANALYTICS_API_KEY" ]; then
    prompt_with_config "Enter Logflare API key" "analytics.apiKey" "" "true" ANALYTICS_API_KEY "false"
    if [ -z "$ANALYTICS_API_KEY" ]; then
      echo "Error: Logflare API key is required"
      exit 1
    fi
  fi
  echo -n "$ANALYTICS_API_KEY" > "$OUTPUT_DIR/analytics_api_key"
  
  echo "Analytics secret saved."
}

# Check if AWS CLI is installed
check_aws_cli() {
  if ! command -v aws &> /dev/null; then
    echo "AWS CLI is not installed. Please install it to use AWS features."
    return 1
  fi
  return 0
}

# Check if IAM user already exists
check_iam_user_exists() {
  local username="$1"
  
  echo "Checking if IAM user $username already exists..."
  
  # Try to get user info, suppress errors
  if aws iam get-user --user-name "$username" --no-cli-pager >/dev/null 2>&1; then
    return 0  # User exists
  else
    return 1  # User doesn't exist
  fi
}

# Check if policy already exists
check_policy_exists() {
  local policy_name="$1"
  
  echo "Checking if policy $policy_name already exists..."
  
  # List policies and check if policy exists
  policy_arn=$(aws iam list-policies --query "Policies[?PolicyName=='$policy_name'].Arn" --output text)
  
  if [ -n "$policy_arn" ] && [ "$policy_arn" != "None" ]; then
    echo "Found existing policy: $policy_arn"
    return 0  # Policy exists
  else
    return 1  # Policy doesn't exist
  fi
}

# Check if S3 bucket already exists
check_s3_bucket_exists() {
  local bucket_name="$1"
  
  echo "Checking if S3 bucket $bucket_name already exists..."
  
  # Try to get bucket info, suppress errors
  if aws s3api head-bucket --bucket "$bucket_name" --no-cli-pager 2>/dev/null; then
    return 0  # Bucket exists
  else
    return 1  # Bucket doesn't exist
  fi
}

# Create IAM user for S3 access
create_aws_user() {
  local username="$1"
  
  # Check if user already exists
  if check_iam_user_exists "$username"; then
    echo "IAM user $username already exists."
    read -p "Do you want to use this existing user? (y/N): " use_existing
    
    if [[ "$use_existing" =~ ^[Yy]$ ]]; then
      echo "Using existing IAM user $username."
      # Create new access key for existing user
      read -p "Do you want to create a new access key for this user? (Y/n): " create_key
      
      if [[ "$create_key" =~ ^[Nn]$ ]]; then
        echo "Please enter the existing access key details:"
        prompt_with_config "Enter S3 key ID" "s3.keyId" "" "false" S3_KEY_ID
        prompt_with_config "Enter S3 access key" "s3.accessKey" "" "true" S3_ACCESS_KEY
      else
        aws_response=$(aws iam create-access-key --user-name "$username")
        S3_KEY_ID=$(echo "$aws_response" | grep -o '"AccessKeyId": "[^"]*"' | cut -d'"' -f4)
        S3_ACCESS_KEY=$(echo "$aws_response" | grep -o '"SecretAccessKey": "[^"]*"' | cut -d'"' -f4)
        set_config_value "s3.keyId" "$S3_KEY_ID"
        set_config_value "s3.accessKey" "$S3_ACCESS_KEY"
        echo "New access key created for existing user."
      fi
    else
      # Ask for a different username
      prompt_with_config "Enter a different IAM username" "s3.aws.username" "supabase-storage-$(date +%s)" "false" username
      set_config_value "s3.aws.username" "$username"
      create_aws_user "$username"  # Recursively call with new name
      return
    fi
  else
    echo "Creating IAM user $username..."
    
    # Create the IAM user
    aws iam create-user --user-name "$username"
    
    # Create access key
    aws_response=$(aws iam create-access-key --user-name "$username")
    
    # Extract access key ID and secret access key
    S3_KEY_ID=$(echo "$aws_response" | grep -o '"AccessKeyId": "[^"]*"' | cut -d'"' -f4)
    S3_ACCESS_KEY=$(echo "$aws_response" | grep -o '"SecretAccessKey": "[^"]*"' | cut -d'"' -f4)
    
    # Save to config
    set_config_value "s3.keyId" "$S3_KEY_ID"
    set_config_value "s3.accessKey" "$S3_ACCESS_KEY"
    
    echo "IAM user created with access key."
  fi
}

# Create S3 bucket
create_s3_bucket() {
  local bucket_name="$1"
  local region="$2"
  
  # Check if bucket already exists
  if check_s3_bucket_exists "$bucket_name"; then
    echo "S3 bucket $bucket_name already exists."
    read -p "Do you want to use this existing bucket? (Y/n): " use_existing
    
    if [[ "$use_existing" =~ ^[Nn]$ ]]; then
      # Ask for a different bucket name
      prompt_with_config "Enter a different S3 bucket name" "s3.aws.bucketName" "supabase-storage-$(date +%s)" "false" bucket_name
      set_config_value "s3.aws.bucketName" "$bucket_name"
      create_s3_bucket "$bucket_name" "$region"  # Recursively call with new name
      return
    else
      echo "Using existing S3 bucket $bucket_name."
      # Save to config
      set_config_value "s3.aws.bucketName" "$bucket_name"
      set_config_value "s3.aws.region" "$region"
    fi
  else
    echo "Creating S3 bucket $bucket_name in region $region..."
    
    # Create the bucket
    aws s3api create-bucket --bucket "$bucket_name" --region "$region" --create-bucket-configuration LocationConstraint="$region"
    
    # Save to config
    set_config_value "s3.aws.bucketName" "$bucket_name"
    set_config_value "s3.aws.region" "$region"
    
    echo "S3 bucket created."
  fi
}

# Attach policy to IAM user
attach_s3_policy() {
  local username="$1"
  local bucket_name="$2"
  local policy_name="${username}-s3-policy"
  
  echo "Attaching S3 policy to user $username for bucket $bucket_name..."
  
  # Check if policy already exists
  if check_policy_exists "$policy_name"; then
    echo "Policy $policy_name already exists."
    read -p "Do you want to use this existing policy? (y/n): " use_existing
    
    if [[ "$use_existing" =~ ^[Yy]$ ]]; then
      policy_arn=$(aws iam list-policies --query "Policies[?PolicyName=='$policy_name'].Arn" --output text)
      echo "Using existing policy: $policy_arn"
      
      # Check if already attached
      attached=$(aws iam list-attached-user-policies --user-name "$username" --query "AttachedPolicies[?PolicyName=='$policy_name'].PolicyName" --output text)
      
      if [ -n "$attached" ]; then
        echo "Policy is already attached to user $username."
      else
        # Attach policy to user
        aws iam attach-user-policy --user-name "$username" --policy-arn "$policy_arn"
        echo "Existing policy attached to user."
      fi
    else
      # Create a new policy with a different name
      local new_policy_name="${username}-s3-policy-$(date +%s)"
      echo "Creating new policy $new_policy_name instead..."
      create_and_attach_policy "$username" "$bucket_name" "$new_policy_name"
    fi
  else
    create_and_attach_policy "$username" "$bucket_name" "$policy_name"
  fi
}

# Create and attach policy helper function
create_and_attach_policy() {
  local username="$1"
  local bucket_name="$2"
  local policy_name="$3"
  
  # Create policy document
  policy_doc='{
    "Version": "2012-10-17",
    "Statement": [
      {
        "Effect": "Allow",
        "Action": [
          "s3:PutObject",
          "s3:GetObject",
          "s3:ListBucket",
          "s3:DeleteObject"
        ],
        "Resource": [
          "arn:aws:s3:::'$bucket_name'",
          "arn:aws:s3:::'$bucket_name'/*"
        ]
      }
    ]
  }'
  
  # Create policy
  policy_response=$(aws iam create-policy --policy-name "$policy_name" --policy-document "$policy_doc")
  policy_arn=$(echo "$policy_response" | grep -o '"Arn": "[^"]*"' | cut -d'"' -f4)
  
  # Attach policy to user
  aws iam attach-user-policy --user-name "$username" --policy-arn "$policy_arn"
  
  echo "New policy created and attached to user."
}

# Create AWS SES user with send email permissions
create_aws_ses_user() {
  local username="$1"
  local region="$2"
  
  # Check if user already exists
  if check_iam_user_exists "$username"; then
    echo "IAM user $username already exists."
    read -p "Do you want to use this existing user? (y/n): " use_existing
    
    if [[ "$use_existing" =~ ^[Yy]$ ]]; then
      echo "Using existing IAM user $username."
      # Create new access key for existing user
      read -p "Do you want to create a new access key for this user? (y/n): " create_key
      
      if [[ "$create_key" =~ ^[Yy]$ ]]; then
        aws_response=$(aws iam create-access-key --user-name "$username")
        SMTP_KEY_ID=$(echo "$aws_response" | grep -o '"AccessKeyId": "[^"]*"' | cut -d'"' -f4)
        SMTP_SECRET_KEY=$(echo "$aws_response" | grep -o '"SecretAccessKey": "[^"]*"' | cut -d'"' -f4)
        SMTP_PASSWORD="$SMTP_SECRET_KEY"
        set_config_value "smtp.username" "$SMTP_KEY_ID"
        set_config_value "smtp.password" "$SMTP_PASSWORD"
        echo "New access key created for existing user."
      else
        echo "Please enter the existing access key details:"
        prompt_with_config "Enter SMTP username (Access Key ID)" "smtp.username" "" "false" SMTP_USERNAME
        prompt_with_config "Enter SMTP password (Secret Key)" "smtp.password" "" "true" SMTP_PASSWORD
      fi
    else
      # Ask for a different username
      prompt_with_config "Enter a different IAM username" "smtp.aws.username" "supabase-smtp-$(date +%s)" "false" username
      set_config_value "smtp.aws.username" "$username"
      create_aws_ses_user "$username" "$region"  # Recursively call with new name
      return
    fi
  else
    echo "Creating IAM user $username for SES access..."
    
    # Create the IAM user
    aws iam create-user --user-name "$username"
    
    # Create access key
    aws_response=$(aws iam create-access-key --user-name "$username")
    
    # Extract access key ID and secret access key
    SMTP_KEY_ID=$(echo "$aws_response" | grep -o '"AccessKeyId": "[^"]*"' | cut -d'"' -f4)
    SMTP_SECRET_KEY=$(echo "$aws_response" | grep -o '"SecretAccessKey": "[^"]*"' | cut -d'"' -f4)
    
    # Create policy for SES permissions
    local policy_name="${username}-ses-policy"
    
    # Check if policy exists
    if ! check_policy_exists "$policy_name"; then
      # Create policy document for SES permissions
      policy_doc='{
        "Version": "2012-10-17",
        "Statement": [
          {
            "Effect": "Allow",
            "Action": [
              "ses:SendEmail",
              "ses:SendRawEmail"
            ],
            "Resource": "*"
          }
        ]
      }'
      
      # Create policy
      policy_response=$(aws iam create-policy --policy-name "$policy_name" --policy-document "$policy_doc")
      policy_arn=$(echo "$policy_response" | grep -o '"Arn": "[^"]*"' | cut -d'"' -f4)
    else
      policy_arn=$(aws iam list-policies --query "Policies[?PolicyName=='$policy_name'].Arn" --output text)
      echo "Using existing policy: $policy_arn"
    fi
    
    # Attach policy to user
    aws iam attach-user-policy --user-name "$username" --policy-arn "$policy_arn"
    
    # Convert AWS credentials to SMTP credentials
    # Note: SMTP password is a modified version of the secret key
    # See: https://docs.aws.amazon.com/ses/latest/dg/smtp-credentials.html
    
    # For simplicity, we'll use the secret key directly as the SMTP password
    # In a production scenario, you should convert it using the AWS algorithm
    SMTP_PASSWORD="$SMTP_SECRET_KEY"
    
    # Save to config
    set_config_value "smtp.username" "$SMTP_KEY_ID"
    set_config_value "smtp.password" "$SMTP_PASSWORD"
    
    echo "IAM user created with SES permissions."
  fi
}

# Verify SES identity (email or domain)
verify_ses_identity() {
  local identity="$1"
  local region="$2"
  
  echo "Verifying SES identity: $identity in region $region..."
  
  # Check if it's an email or domain
  if [[ $identity == *"@"* ]]; then
    # Verify email identity
    aws ses verify-email-identity --email-address "$identity" --region "$region"
    echo "Verification email sent to $identity. Please check your inbox and follow the instructions."
  else
    # Verify domain identity - updated for AWS CLI v2 compatibility
    aws sesv2 create-email-identity --email-identity "$identity" --region "$region" || \
    aws ses verify-domain-identity --domain "$identity" --region "$region"
    
    # Get DNS verification records - try both sesv2 and ses
    dkim_response=$(aws sesv2 get-email-identity --email-identity "$identity" --region "$region" 2>/dev/null || \
                   aws ses get-domain-dkim --domain "$identity" --region "$region")
    
    echo "Domain verification initiated. Please add the following DNS records to your domain:"
    echo
    
    if echo "$dkim_response" | grep -q "DkimTokens"; then
      # Old SES API response format
      dkim_tokens=$(echo "$dkim_response" | grep -o '"DkimTokens": \[[^]]*\]' | grep -o '"[^"]*"' | tr -d '"')
      for token in $dkim_tokens; do
        echo "CNAME: ${token}._domainkey.${identity} => ${token}.dkim.amazonses.com"
      done
      
      echo
      echo "Also add this TXT record:"
      echo "TXT: _amazonses.${identity} => $(aws ses get-identity-verification-attributes --identities "$identity" --region "$region" | grep -o '"VerificationToken": "[^"]*"' | cut -d'"' -f4)"
    else
      # New SESv2 API response format
      echo "$dkim_response" | grep -A 20 "DkimAttributes"
      echo
      echo "Follow the instructions above to set up the required DNS records."
    fi
  fi
  
  # Save identity to config
  set_config_value "smtp.aws.identity" "$identity"
  
  echo "SES identity verification initiated."
}

# Generate SMTP secrets
generate_smtp_secrets() {
  echo "Generating SMTP secrets..."
  
  # Get settings from config
  local create_ses_user=$(get_config_value "smtp.aws.createSesUser" "false")
  
  # If we have SMTP credentials already in config, use them
  SMTP_USERNAME=$(get_config_value "smtp.username" "")
  SMTP_PASSWORD=$(get_config_value "smtp.password" "")
  SMTP_HOST=$(get_config_value "smtp.host" "")
  SMTP_PORT=$(get_config_value "smtp.port" "")
  SMTP_SENDER_NAME=$(get_config_value "smtp.sender_name" "")
  
  # Check if sender name is null or empty
  if is_null_value "smtp.sender_name" || [ -z "$SMTP_SENDER_NAME" ]; then
    # Set a default sender name if not provided
    SMTP_SENDER_NAME="Supabase"
    set_config_value "smtp.sender_name" "$SMTP_SENDER_NAME"
  fi
  
  # Get previously selected mode if available
  local smtp_mode=$(get_config_value "smtp.mode" "")
  
  if [ "$SMTP_USERNAME" != "" ] && [ "$SMTP_PASSWORD" != "" ] && [ "$SMTP_HOST" != "" ] && [ "$SMTP_PORT" != "" ]; then
    echo "Using SMTP credentials from config file."
  else
    # Options for SMTP credentials
    echo "How would you like to configure SMTP credentials?"
    echo "1. Enter existing SMTP credentials"
    echo "2. Use AWS SES (Simple Email Service) (requires AWS CLI and sufficient permissions)"
    echo "3. Generate random credentials (for development only)"
    
    # Use previous choice as default if available
    if [ -n "$smtp_mode" ]; then
      read -p "Select an option (1-3) [previous: $smtp_mode]: " smtp_option
      smtp_option=${smtp_option:-$smtp_mode}
    else
      read -p "Select an option (1-3): " smtp_option
    fi
    
    # Save the chosen mode to config
    set_config_value "smtp.mode" "$smtp_option"
    
    case "$smtp_option" in
      1)
        # Enter existing credentials
        prompt_with_config "Enter SMTP username (email)" "smtp.username" "your-mail@example.com" "false" SMTP_USERNAME
        prompt_with_config "Enter SMTP password" "smtp.password" "" "true" SMTP_PASSWORD
        prompt_with_config "Enter SMTP host" "smtp.host" "smtp.example.com" "false" SMTP_HOST
        prompt_with_config "Enter SMTP port" "smtp.port" "587" "false" SMTP_PORT
        prompt_with_config "Enter SMTP sender name" "smtp.sender_name" "Supabase" "false" SMTP_SENDER_NAME
        ;;
      2)
        # Use AWS SES
        set_config_value "smtp.aws.createSesUser" "true"
        
        if check_aws_cli; then
          prompt_with_config "Enter AWS region for SES" "smtp.aws.region" "us-east-1" "false" AWS_REGION
          prompt_with_config "Enter IAM username for SES" "smtp.aws.username" "supabase-smtp" "false" AWS_USERNAME
          
          # Create IAM user with SES permissions
          create_aws_ses_user "$AWS_USERNAME" "$AWS_REGION"
          
          # Prompt for identity to verify
          prompt_with_config "Enter email or domain to verify with SES" "smtp.aws.identity" "" "false" SES_IDENTITY
          
          if [ -n "$SES_IDENTITY" ]; then
            verify_ses_identity "$SES_IDENTITY" "$AWS_REGION"
          fi
          
          # Set AWS SES SMTP settings
          SMTP_HOST="email-smtp.${AWS_REGION}.amazonaws.com"
          SMTP_PORT="587"
          SMTP_SENDER_NAME=${SES_IDENTITY%%@*}  # Use the part before @ as sender name
          
          set_config_value "smtp.host" "$SMTP_HOST"
          set_config_value "smtp.port" "$SMTP_PORT"
          set_config_value "smtp.sender_name" "$SMTP_SENDER_NAME"
          
          echo "AWS SES SMTP endpoint: $SMTP_HOST"
          echo "IMPORTANT: If you verified a new identity, you need to complete the verification process before sending emails."
        else
          # Fallback to manual entry
          echo "Falling back to manual credential entry..."
          prompt_with_config "Enter SMTP username (email)" "smtp.username" "your-mail@example.com" "false" SMTP_USERNAME
          prompt_with_config "Enter SMTP password" "smtp.password" "" "true" SMTP_PASSWORD
          prompt_with_config "Enter SMTP host" "smtp.host" "smtp.example.com" "false" SMTP_HOST
          prompt_with_config "Enter SMTP port" "smtp.port" "587" "false" SMTP_PORT
          prompt_with_config "Enter SMTP sender name" "smtp.sender_name" "Supabase" "false" SMTP_SENDER_NAME
        fi
        ;;
      3|*)
        # Generate random credentials (default)
        echo "Generating random SMTP credentials for development..."
        SMTP_USERNAME="dev-$(openssl rand -hex 4)@example.com"
        SMTP_PASSWORD=$(openssl rand -base64 12)
        SMTP_HOST="smtp.example.com"
        SMTP_PORT="587"
        SMTP_SENDER_NAME="Supabase Dev"
        
        set_config_value "smtp.username" "$SMTP_USERNAME"
        set_config_value "smtp.password" "$SMTP_PASSWORD"
        set_config_value "smtp.host" "$SMTP_HOST"
        set_config_value "smtp.port" "$SMTP_PORT"
        set_config_value "smtp.sender_name" "$SMTP_SENDER_NAME"
        
        echo "WARNING: These credentials are not real and emails won't be sent in production."
        ;;
    esac
  fi
  
  # Save the credentials
  echo -n "$SMTP_USERNAME" > "$OUTPUT_DIR/smtp_username"
  echo -n "$SMTP_PASSWORD" > "$OUTPUT_DIR/smtp_password"
  echo -n "$SMTP_HOST" > "$OUTPUT_DIR/smtp_host"
  echo -n "$SMTP_PORT" > "$OUTPUT_DIR/smtp_port"
  echo -n "$SMTP_SENDER_NAME" > "$OUTPUT_DIR/smtp_sender_name"
  
  echo "SMTP secrets generated."
}

# Generate dashboard secrets
generate_dashboard_secrets() {
  echo "Generating dashboard secrets..."
  
  # Prompt for username with config-based default
  if [ -z "$DASHBOARD_USERNAME" ]; then
    prompt_with_config "Enter dashboard username" "dashboard.username" "supabase" "false" DASHBOARD_USERNAME
  fi
  echo -n "$DASHBOARD_USERNAME" > "$OUTPUT_DIR/dashboard_username"
  
  # Get password from config or generate
  if [ -z "$DASHBOARD_PASSWORD" ]; then
    prompt_with_config "Enter dashboard password" "dashboard.password" "" "true" DASHBOARD_PASSWORD "true"
    if [ -z "$DASHBOARD_PASSWORD" ]; then
      DASHBOARD_PASSWORD=$(openssl rand -base64 12)
      set_config_value "dashboard.password" "$DASHBOARD_PASSWORD"
      echo "Generated random dashboard password"
    fi
  fi
  echo -n "$DASHBOARD_PASSWORD" > "$OUTPUT_DIR/dashboard_password"
  
  echo "Dashboard secrets generated."
}

# Generate S3 secrets
generate_s3_secrets() {
  echo "Generating S3 secrets..."
  
  # Get settings from config
  local generate_random=$(get_config_value "s3.generateRandom" "false")
  local create_new_user=$(get_config_value "s3.aws.createNewUser" "false")
  local create_new_bucket=$(get_config_value "s3.aws.createNewBucket" "false")
  
  # If we have credentials already in config, use them
  S3_KEY_ID=$(get_config_value "s3.keyId" "")
  S3_ACCESS_KEY=$(get_config_value "s3.accessKey" "")
  
  # Get previously selected mode if available
  local s3_mode=$(get_config_value "s3.mode" "")
  
  if [ "$S3_KEY_ID" != "" ] && [ "$S3_ACCESS_KEY" != "" ] && ! is_null_value "s3.keyId" && ! is_null_value "s3.accessKey" ; then
    echo "Using S3 credentials from config file."
  else
    # Options for S3 credentials
    echo "How would you like to configure S3 credentials?"
    echo "1. Enter existing AWS credentials"
    echo "2. Create new IAM user and S3 bucket (requires AWS CLI and sufficient permissions)"
    echo "3. Create new IAM user for existing S3 bucket (requires AWS CLI and sufficient permissions)"
    echo "4. Generate random credentials (not connected to AWS)"
    
    # Use previous choice as default if available
    if [ -n "$s3_mode" ]; then
      read -p "Select an option (1-4) [previous: $s3_mode]: " s3_option
      s3_option=${s3_option:-$s3_mode}
    else
      read -p "Select an option (1-4): " s3_option
    fi
    
    # Save the chosen mode to config
    set_config_value "s3.mode" "$s3_option"
    
    case "$s3_option" in
      1)
        # Enter existing credentials
        prompt_with_config "Enter S3 key ID" "s3.keyId" "" "false" S3_KEY_ID
        prompt_with_config "Enter S3 access key" "s3.accessKey" "" "true" S3_ACCESS_KEY
        ;;
      2)
        # Create new AWS user and bucket
        if ! check_aws_cli; then
          echo "AWS CLI is required for this option."
          return 1
        fi
        
        # Prompt for IAM username
        prompt_with_config "Enter IAM username for S3 access" "s3.aws.username" "supabase-storage-$(date +%s)" "false" username
        set_config_value "s3.aws.username" "$username"
        
        # Create IAM user
        create_aws_user "$username"
        
        # Prompt for S3 bucket name
        prompt_with_config "Enter S3 bucket name" "s3.aws.bucketName" "supabase-storage-$(date +%s)" "false" bucket_name
        set_config_value "s3.aws.bucketName" "$bucket_name"
        
        # Prompt for AWS region
        prompt_with_config "Enter AWS region" "s3.aws.region" "us-east-1" "false" region
        set_config_value "s3.aws.region" "$region"
        
        # Create bucket
        create_s3_bucket "$bucket_name" "$region"
        
        # Save bucket name and region to files
        echo -n "$bucket_name" > "$OUTPUT_DIR/s3_bucket_name"
        echo -n "$region" > "$OUTPUT_DIR/s3_region"
        
        # Create and attach policy
        attach_s3_policy "$username" "$bucket_name"
        
        # Record that we created a user and bucket
        set_config_value "s3.aws.createNewUser" "true"
        set_config_value "s3.aws.createNewBucket" "true"
        ;;
      3)
        # Create new AWS user for existing bucket
        if ! check_aws_cli; then
          echo "AWS CLI is required for this option."
          return 1
        fi
        
        # Prompt for IAM username
        prompt_with_config "Enter IAM username for S3 access" "s3.aws.username" "supabase-storage-$(date +%s)" "false" username
        set_config_value "s3.aws.username" "$username"
        
        # Create IAM user
        create_aws_user "$username"
        
        # Prompt for existing S3 bucket name
        prompt_with_config "Enter existing S3 bucket name" "s3.aws.bucketName" "" "false" bucket_name
        set_config_value "s3.aws.bucketName" "$bucket_name"
        
        # Prompt for AWS region
        prompt_with_config "Enter AWS region" "s3.aws.region" "us-east-1" "false" region
        set_config_value "s3.aws.region" "$region"
        
        # Save bucket name and region to files
        echo -n "$bucket_name" > "$OUTPUT_DIR/s3_bucket_name"
        echo -n "$region" > "$OUTPUT_DIR/s3_region"
        
        # Create and attach policy
        attach_s3_policy "$username" "$bucket_name"
        
        # Record that we created a user but not a bucket
        set_config_value "s3.aws.createNewUser" "true"
        set_config_value "s3.aws.createNewBucket" "false"
        ;;
      4)
        # Generate random credentials
        echo "Generating random S3 credentials (not connected to AWS)."
        S3_KEY_ID="FAKE$(openssl rand -hex 8)KEY"
        S3_ACCESS_KEY=$(openssl rand -base64 30 | tr -d '\n')
        set_config_value "s3.keyId" "$S3_KEY_ID"
        set_config_value "s3.accessKey" "$S3_ACCESS_KEY"
        set_config_value "s3.generateRandom" "true"
        ;;
      *)
        echo "Invalid option."
        return 1
        ;;
    esac
  fi
  
  # If we still don't have credentials, manually ask for them
  if [ -z "$S3_KEY_ID" ] || is_null_value "s3.keyId" ; then
    prompt_with_config "Enter S3 key ID" "s3.keyId" "" "false" S3_KEY_ID "true"
    if [ -z "$S3_KEY_ID" ]; then
      S3_KEY_ID="FAKE$(openssl rand -hex 8)KEY"
      set_config_value "s3.keyId" "$S3_KEY_ID"
      echo "Generated random S3 key ID: $S3_KEY_ID"
    fi
  fi
  
  if [ -z "$S3_ACCESS_KEY" ] || is_null_value "s3.accessKey" ; then
    prompt_with_config "Enter S3 access key" "s3.accessKey" "" "true" S3_ACCESS_KEY "true"
    if [ -z "$S3_ACCESS_KEY" ]; then
      S3_ACCESS_KEY=$(openssl rand -base64 30 | tr -d '\n')
      set_config_value "s3.accessKey" "$S3_ACCESS_KEY"
      echo "Generated random S3 access key"
    fi
  fi
  
  # Save to files
  echo -n "$S3_KEY_ID" > "$OUTPUT_DIR/s3_key_id"
  echo -n "$S3_ACCESS_KEY" > "$OUTPUT_DIR/s3_access_key"
  
  echo "S3 secrets generated."
}

# Generate all secrets
generate_all_secrets() {
  generate_jwt_secrets
  generate_db_secrets
  generate_analytics_secrets
  generate_smtp_secrets
  generate_dashboard_secrets
  generate_s3_secrets
  
  echo "All secrets generated in $OUTPUT_DIR/"
}

# Display help
show_help() {
  echo "Usage: $0 [option]"
  echo "Options:"
  echo "  --all             Generate all secrets"
  echo "  --jwt             Generate JWT secrets"
  echo "  --db              Generate database secrets"
  echo "  --analytics       Generate analytics secrets"
  echo "  --smtp            Generate SMTP secrets"
  echo "  --dashboard       Generate dashboard secrets"
  echo "  --s3              Generate S3 secrets"
  echo "  --help            Show this help message"
  echo
  echo "Configuration:"
  echo "  Settings are read from and saved to config.yaml"
  echo "  You can pre-configure values in config.yaml to avoid interactive prompts"
  echo
  echo "Environment variables (override config.yaml):"
  echo "  DB_USERNAME       Override database username"
  echo "  DB_PASSWORD       Override database password"
  echo "  DB_NAME           Override database name"
  echo "  SMTP_USERNAME     Override SMTP username"
  echo "  SMTP_PASSWORD     Override SMTP password"
  echo "  SMTP_HOST         Override SMTP host"
  echo "  SMTP_PORT         Override SMTP port"
  echo "  SMTP_SENDER_NAME  Override SMTP sender name"
  echo "  DASHBOARD_USERNAME Override dashboard username"
  echo "  DASHBOARD_PASSWORD Override dashboard password"
  echo "  ANALYTICS_API_KEY  Override analytics API key"
  echo "  S3_KEY_ID         Override S3 key ID"
  echo "  S3_ACCESS_KEY     Override S3 access key"
}

# Parse command line arguments
if [ $# -eq 0 ]; then
  # No arguments, generate all secrets
  generate_all_secrets
else
  case "$1" in
    --all)
      generate_all_secrets
      ;;
    --jwt)
      generate_jwt_secrets
      ;;
    --db)
      generate_db_secrets
      ;;
    --analytics)
      generate_analytics_secrets
      ;;
    --smtp)
      generate_smtp_secrets
      ;;
    --dashboard)
      generate_dashboard_secrets
      ;;
    --s3)
      generate_s3_secrets
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