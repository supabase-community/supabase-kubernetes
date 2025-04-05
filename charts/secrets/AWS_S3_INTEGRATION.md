# AWS S3 Integration for Supabase Storage

This document provides detailed information on configuring Supabase to use AWS S3 for storage.

## Overview

Supabase Storage can be configured to use AWS S3 as the backend storage provider. This allows you to:

1. Use scalable, durable S3 storage for your application files
2. Comply with data residency requirements by choosing specific AWS regions
3. Leverage existing AWS infrastructure and policies

## Setup Options

Our secret management tools provide three options for S3 integration:

### Option 1: Use Existing AWS Credentials

Use this option if you already have an IAM user with appropriate S3 permissions. You'll need:

- AWS Access Key ID
- AWS Secret Access Key
- Existing S3 bucket name
- AWS region

### Option 2: Create New IAM User and S3 Bucket

This option automatically:

1. Creates a new IAM user specific to Supabase storage
2. Creates a new S3 bucket with a unique name
3. Attaches appropriate IAM policies for bucket access
4. Generates and securely stores the credentials

Requirements:
- AWS CLI installed and configured with admin credentials
- Permissions to create IAM users, policies, and S3 buckets

### Option 3: Create New IAM User for Existing S3 Bucket

This option automatically:

1. Creates a new IAM user specific to Supabase storage
2. Attaches appropriate IAM policies for accessing your existing bucket
3. Generates and securely stores the credentials

Requirements:
- AWS CLI installed and configured with admin credentials
- Permissions to create IAM users and policies
- Name of an existing S3 bucket

## IAM Permissions

The IAM policy created by our tools grants the following permissions to the user:

```json
{
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
        "arn:aws:s3:::your-bucket-name",
        "arn:aws:s3:::your-bucket-name/*"
      ]
    }
  ]
}
```

## S3 Bucket Considerations

When using S3 with Supabase Storage:

1. **CORS Configuration**: Your S3 bucket may need CORS configuration to allow requests from your application domain.

2. **Public Access**: By default, S3 objects are not publicly accessible. Supabase Storage manages access control.

3. **Bucket Policies**: If you're using an existing bucket with bucket policies, ensure they don't conflict with Supabase Storage operations.

4. **Bucket Lifecycle**: Consider setting up lifecycle rules to manage object versions and transitions.

## Configuring Supabase Helm Chart

After creating the S3 setup, you need to update your Helm values:

```yaml
# Reference the S3 credentials secret
secret:
  s3:
    secretRef: "supabase-s3"

# Configure storage to use S3
storage:
  environment:
    STORAGE_BACKEND: s3
    GLOBAL_S3_BUCKET: "your-bucket-name"
    GLOBAL_S3_PROTOCOL: https
    AWS_DEFAULT_REGION: "your-region"
    # If not using AWS (e.g., for MinIO or other S3-compatible services)
    # GLOBAL_S3_ENDPOINT: "http://your-s3-endpoint"
    # GLOBAL_S3_FORCE_PATH_STYLE: "true"
```

## Command Line Setup

To set up AWS S3 integration using the Makefile:

```bash
# Interactive setup that guides you through the options
make setup-aws-s3

# Create the Kubernetes secret
make create-s3-secret
```

## Troubleshooting

If you encounter issues with S3 integration:

1. Verify AWS credentials are correct and have the necessary permissions
2. Ensure the S3 bucket exists and is in the correct region
3. Check that the S3 bucket is accessible from your Kubernetes cluster
4. Verify network policies allow outbound connections to S3 endpoints
5. Check the Supabase Storage logs for specific error messages

## Manual Setup

If you prefer to set up S3 integration manually:

1. Create an IAM user with S3 access policy
2. Create or select an S3 bucket
3. Configure the bucket for Supabase use (CORS, etc.)
4. Update your Helm values file with the S3 configuration
5. Create the S3 credentials secret manually

## Security Considerations

- IAM users should follow the principle of least privilege
- Consider using temporary credentials or IAM roles for production
- Enable bucket encryption for sensitive data
- Implement bucket policies to prevent unauthorized access
- Consider using VPC endpoints for S3 to keep traffic within AWS network 