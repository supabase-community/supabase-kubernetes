# AWS SES Integration for Supabase Email

This document provides detailed information on configuring Supabase authentication to use AWS Simple Email Service (SES) for sending emails.

## Overview

Supabase Auth (GoTrue) can be configured to use AWS SES for sending transactional emails such as:
- Verification emails
- Password reset emails
- Magic link authentication
- Invitation emails

Using AWS SES provides several benefits:
1. Scalable, reliable email delivery
2. High deliverability rates and reputation management
3. Detailed sending statistics and bounce tracking
4. Compliance with email regulations

## Setup Options

Our secret management tools provide the following options for email configuration:

### Option 1: Use Existing SMTP Credentials

Use this option if you already have SMTP credentials for any email service provider. You'll need:

- SMTP username
- SMTP password
- SMTP host
- SMTP port
- Sender name

### Option 2: Use AWS SES

This option automatically:

1. Creates a new IAM user specific to Supabase email sending
2. Attaches appropriate IAM policies for SES access
3. Initiates verification of your email or domain identity
4. Configures the SMTP credentials for Supabase

Requirements:
- AWS CLI installed and configured with admin credentials
- Permissions to create IAM users and policies
- A valid email address or domain to verify with SES

## IAM Permissions

The IAM policy created by our tools grants the following permissions to the user:

```json
{
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
}
```

## SES Identity Verification

AWS SES requires verification of email addresses or domains before you can send emails from them:

### Email Verification
- A verification email is sent to the address
- You must click the verification link to complete the process
- Until verified, you cannot send emails from this address

### Domain Verification
- DNS records (DKIM and TXT) must be added to your domain
- Once DNS propagates, the domain will be verified
- You can then send emails from any address on that domain

## SES Production Access

By default, new SES accounts are in the "sandbox" mode with restrictions:
- You can only send to verified email addresses/domains
- Daily sending limits are low

To request production access:
1. Go to the AWS SES console
2. Click "Request Production Access"
3. Fill out the request form with your use case details

## Configuring Supabase Helm Chart

After setting up AWS SES, you need to update your Helm values:

```yaml
# Reference the SMTP credentials secret
secret:
  smtp:
    secretRef: "supabase-smtp"

# Configure Auth to use SES
auth:
  environment:
    GOTRUE_SMTP_ADMIN_EMAIL: "your-verified-email@example.com"
    GOTRUE_SMTP_HOST: "email-smtp.us-east-1.amazonaws.com"
    GOTRUE_SMTP_PORT: "587"
    GOTRUE_SMTP_SENDER_NAME: "Your App Name"
    GOTRUE_EXTERNAL_EMAIL_ENABLED: "true"
```

## Command Line Setup

To set up AWS SES integration using the Makefile:

```bash
# Interactive setup that guides you through the options
make setup-aws-ses

# Create the Kubernetes secret
make create-smtp-secret
```

## Troubleshooting

If you encounter issues with SES integration:

1. Verify SES Identity Status: Check that your email/domain is verified in the SES console
2. Check Sending Limits: Ensure you're not exceeding SES sandbox limits
3. Monitor Bounces/Complaints: SES might suspend sending if bounce rates are high
4. Check IAM Permissions: Ensure the user has proper SES permissions
5. Verify SMTP Credentials: Make sure you're using the correct SMTP credentials
6. Check Auth Logs: Examine GoTrue logs for specific errors

## Security Considerations

- IAM users should follow the principle of least privilege
- Monitor SES sending statistics to detect unauthorized use
- Consider using separate identities for different email types
- Rotate SMTP credentials periodically
- Keep your sender reputation high by following email best practices 