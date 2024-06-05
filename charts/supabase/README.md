# Supabase for Kubernetes with Helm 3

![Version: 0.1.2](https://img.shields.io/badge/Version-0.1.2-informational?style=for-the-badge) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=for-the-badge)

The open source Firebase alternative.

![Supabase](https://avatars.githubusercontent.com/u/54469796?s=280&v=4) 

This directory contains the configurations and scripts required to run Supabase inside a Kubernetes cluster.

## Disclamer

We use [supabase/postgres](https://hub.docker.com/r/supabase/postgres) to create and manage the Postgres database. This permit you to use replication if needed but you'll have to use the Postgres image provided Supabase or build your own on top of it. You can also choose to use other databases provider like [StackGres](https://stackgres.io/) or [Postgres Operator](https://github.com/zalando/postgres-operator).

For the moment we are using a root container to permit the installation of the missing `pgjwt` and `wal2json` extension inside the `initdbScripts`. This is considered a security issue, but you can use your own Postgres image instead with the extension already installed to prevent this. We provide an example of `Dockerfile`for this purpose, you can use [ours](https://hub.docker.com/r/tdeoliv/supabase-bitnami-postgres) or build and host it on your own.

The database configuration we provide is an example using only one master. If you want to go to production, we highly recommend you to use a replicated database.

## Quickstart

> For this section we're using Minikube and Docker to create a Kubernetes cluster

```bash
# Clone Repository
git clone https://github.com/supabase-community/supabase-kubernetes

# Switch to charts directory
cd supabase-kubernetes/charts/supabase/

# Install the chart
helm install demo -f values.example.yaml .
```

The first deployment can take some time to complete (especially auth service). You can view the status of the pods using:

```bash
kubectl get pod -l app.kubernetes.io/instance=demo

NAME                                      READY   STATUS    RESTARTS      AGE
demo-supabase-analytics-xxxxxxxxxx-xxxxx  1/1     Running   0             47s
demo-supabase-auth-xxxxxxxxxx-xxxxx       1/1     Running   0             47s
demo-supabase-db-xxxxxxxxxx-xxxxx         1/1     Running   0             47s
demo-supabase-functions-xxxxxxxxxx-xxxxx  1/1     Running   0             47s
demo-supabase-imgproxy-xxxxxxxxxx-xxxxx   1/1     Running   0             47s
demo-supabase-kong-xxxxxxxxxx-xxxxx       1/1     Running   0             47s
demo-supabase-meta-xxxxxxxxxx-xxxxx       1/1     Running   0             47s
demo-supabase-realtime-xxxxxxxxxx-xxxxx   1/1     Running   0             47s
demo-supabase-rest-xxxxxxxxxx-xxxxx       1/1     Running   0             47s
demo-supabase-storage-xxxxxxxxxx-xxxxx    1/1     Running   0             47s
```

### Access with Minikube

Assuming that you have enabled Minikube ingress addon, note down the Minikube IP address:
```shell
minikube ip
```
Then, add the IP into your `/etc/hosts` file:
```bash
# This will redirect request for example.com to the minikube IP
<minikube-ip> example.com
```
Open http://example.com in your browser.

### Uninstall

```Bash
# Uninstall Helm chart
helm uninstall demo

# Backup and/or remove any Persistent Volume Claims that have keep annotation
kubectl delete pvc demo-supabase-storage-pvc
```

## Customize

You should consider to adjust the following values in `values.yaml`:

- `RELEASE_NAME`: Name used for helm release
- `STUDIO.EXAMPLE.COM` URL to Studio

If you want to use mail, consider to adjust the following values in `values.yaml`:

- `SMTP_ADMIN_MAIL`
- `SMTP_HOST`
- `SMTP_PORT`
- `SMTP_SENDER_NAME`

### JWT Secret

We encourage you to use your own JWT keys by generating a new Kubernetes secret and reference it in `values.yaml`:

```yaml
secret:
  jwt:
    anonKey: <new-anon-jwt>
    serviceKey: <new-service-role-jwt>
    secret: <jwt-secret>
```

> 32 characters long secret can be generated with `openssl rand 64 | base64`
> You can use the [JWT Tool](https://supabase.com/docs/guides/hosting/overview#api-keys) to generate anon and service keys.

### SMTP Secret

Connection credentials for the SMTP mail server will also be provided via Kubernetes secret referenced in `values.yaml`:

```yaml
secret:
  smtp:
    username: <your-smtp-username>
    password: <your-smtp-password>
```

### DB Secret

DB credentials will also be stored in a Kubernetes secret and referenced in `values.yaml`:

```yaml
secret:
  db:
    username: <db-username>
    password: <db-password>
    database: <supabase-database-name>
```

The secret can be created with kubectl via command-line:

> If you depend on database providers like [StackGres](https://stackgres.io/), [Postgres Operator](https://github.com/zalando/postgres-operator) or self-hosted Postgres instance, fill in the secret above and modify any relevant Postgres attributes such as port or hostname (e.g. `PGPORT`, `DB_HOST`) for any relevant deployments. Refer to [values.yaml](values.yaml) for more details.

### Dashboard secret

By default, a username and password is required to access the Supabase Studio dashboard. Simply change them at:

```yaml
secret:
  dashboard:
    username: supabase
    password: this_password_is_insecure_and_should_be_updated
```

### Analytics secret

A new logflare secret API key is required for securing communication between all of the Supabase services. To set the secret, generate a new 32 characters long secret similar to the step [above](#jwt-secret).

```yaml
secret:
  analytics:
    apiKey: your-super-secret-with-at-least-32-characters-long-logflare-key
```

### S3 secret

Supabase storage supports the use of S3 object-storage. To enable S3 for Supabase storage:

1. Set S3 key ID and access key:
  ```yaml
   secret:
    s3:
      keyId: your-s3-key-id
      accessKey: your-s3-access-key
  ```
2. Set storage S3 environment variables:
  ```yaml
  storage:
    environment:
      # Set S3 endpoint if using external object-storage
      # GLOBAL_S3_ENDPOINT: http://minio:9000
      STORAGE_BACKEND: s3
      GLOBAL_S3_PROTOCOL: http
      GLOBAL_S3_FORCE_PATH_STYLE: true
      AWS_DEFAULT_REGION: stub
  ```
3. (Optional) Enable internal minio deployment
  ```yaml
  minio:
    enabled: true
  ```

## How to use in Production

We didn't provide a complete configuration to go production because of the multiple possibility.

But here are the important points you have to think about:

- Use a replicated version of the Postgres database.
- Add SSL to the Postgres database.
- Add SSL configuration to the ingresses endpoints using either the `cert-manager` or a LoadBalancer provider.
- Change the domain used in the ingresses endpoints.
- Generate a new secure JWT Secret.

### Migration

Migration from local development is made easy by adding migration scripts at `db.config` field. This will apply all of the migration scripts during the database initialization. For example:

```yaml
db:
  config:
    20230101000000_profiles.sql: |
      create table profiles (
        id uuid references auth.users not null,
        updated_at timestamp with time zone,
        username text unique,
        avatar_url text,
        website text,

        primary key (id),
        unique(username),
        constraint username_length check (char_length(username) >= 3)
      );
```

To make copying scripts easier, use this handy bash script:

```bash
#!/bin/bash

for file in $1/*; do
  clipboard+="    $(basename $file): |\n"
  clipboard+=$(cat $file | awk '{print "      "$0}')
  clipboard+="\n"
done

echo -e "$clipboard"
```

and pipe it to your system clipboard handler:

```shell
# Using xclip as an example
./script.sh supabase/migrations | xclip -sel clipboard
```

## Troubleshooting

### Ingress Controller and Ingress Class

Depending on your Kubernetes version you might want to fill the `className` property instead of the `kubernetes.io/ingress.class` annotations. For example:

```yml
kong:
  ingress:
    enabled: 'true'
    className: "nginx"
    annotations:
      nginx.ingress.kubernetes.io/rewrite-target: /
```

### Testing suite

Before creating a merge request, you can test the charts locally by using [helm/chart-testing](https://github.com/helm/chart-testing). If you have Docker and a Kubernetes environment to test with, simply run:

```shell
# Run chart-testing (lint)
docker run -it \
  --workdir=/data \
  --volume $(pwd)/charts/supabase:/data \
  quay.io/helmpack/chart-testing:v3.7.1 \
  ct lint --validate-maintainers=false --chart-dirs . --charts .
# Run chart-testing (install)
docker run -it \
  --network host \
  --workdir=/data \
  --volume ~/.kube/config:/root/.kube/config:ro \
  --volume $(pwd)/charts/supabase:/data \
  quay.io/helmpack/chart-testing:v3.7.1 \
  ct install --chart-dirs . --charts .
```

### Version compatibility

#### `0.0.x` to `0.1.x`

* `supabase/postgres` is updated from `14.1` to `15.1`, which warrants backing up all your data before proceeding to update to the next major version.
* Intialization scripts for `supabase/postgres` has been reworked and matched closely to the [Docker Compose](https://github.com/supabase/supabase/blob/master/docker/docker-compose.yml) version. Further tweaks to the scripts are needed to ensure backward-compatibility.
* Migration scripts are now exposed at `db.config`, which will be mounted at `/docker-entrypoint-initdb.d/migrations/`. Simply copy your migration files from your local project's `supabase/migration` and populate the `db.config`.
* Ingress are now limited to `kong` & `db` services. This is by design to limit entry to the stack through secure `kong` service.
* `kong.yaml` has been modified to follow [Docker kong.yaml](https://github.com/supabase/supabase/blob/master/docker/volumes/api/kong.yml) template.
* `supabase/storage` does not comes with pre-populated `/var/lib/storage`, therefore an `emptyDir` will be created if persistence is disabled. This might be incompatible with previous version if the persistent storage location is set to location other than specified above.
* `supabase/vector` requires read access to the `/var/log/pods` directory. When run in a Kubernetes cluster this can be provided with a [hostPath](https://kubernetes.io/docs/concepts/storage/volumes/#hostpath) volume.

## Parameters
### Secret parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| secret.analytics | object | The configuration is detailed below. | Analytics Logflare API key |
| secret.analytics.secretRef | string | `""` | Specify an existing secret, which takes precedence over the above variable |
| secret.analytics.secretRefKey | object | `{"apiKey":"apiKey"}` | Override secret keys for existing secret refs |
| secret.dashboard | object | The configuration is detailed below. | Secret used to access the studio dashboard Leave it empty to disable dashboard authentication |
| secret.dashboard.secretRefKey | object | The configuration is detailed below. | Override secret keys for existing secret refs |
| secret.db | object | The configuration is detailed below. | Database credentials These fields must be provided even if using an external database |
| secret.db.secretRef | string | `""` | Specify an existing secret, which takes precedence over the above variables |
| secret.db.secretRefKey | object | The configuration is detailed below. | Override secret keys for existing secret refs |
| secret.jwt | object | The configuration is detailed below. | JWT will be used to reference secret in multiple services. Anon & Service key for Studio, Storage, Kong. JWT Secret for Analytics, Auth, Rest, Realtime, Storage. |
| secret.jwt.secretRef | string | `""` | Specify an existing secret, which takes precedence over the above variables above |
| secret.jwt.secretRefKey | object | The configuration is detailed below. | Override secret keys for existing secret refs |
| secret.s3 | object | The configuration is detailed below. | S3 credentials for storage object bucket |
| secret.s3.secretRefKey | object | The configuration is detailed below. | Override secret keys for existing secret refs |
| secret.smtp | object | The configuration is detailed below. | SMTP will be used to reference secrets including SMTP credentials |
| secret.smtp.secretRefKey | object | The configuration is detailed below. | Override secret keys for existing secret refs |

### Database parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| db | object | The configuration is detailed below. | Optional: Postgres Database A standalone Postgres database configured to work with Supabase services. You can spin up any other Postgres database container if required. If so, make sure to adjust DB_HOST accordingly to point to the right database service. |
| db.config | object | `{}` | Additional migration scripts can be defined here |
| db.enabled | bool | `true` | Enable database provisioning |
| db.serviceAccount.annotations | object | `{}` | Annotations to add to the service account |
| db.serviceAccount.create | bool | `true` | Specifies whether a service account should be created |
| db.serviceAccount.name | string | `""` | The name of the service account to use. If not set and create is true, a name is generated using the fullname template |

### Studio parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| studio.affinity | object | `{}` |  |
| studio.autoscaling.enabled | bool | `true` |  |
| studio.autoscaling.maxReplicas | int | `100` |  |
| studio.autoscaling.minReplicas | int | `1` |  |
| studio.autoscaling.targetCPUUtilizationPercentage | int | `80` |  |
| studio.enabled | bool | `true` | Enable studio provisioning |
| studio.environment.NEXT_ANALYTICS_BACKEND_PROVIDER | string | `"postgres"` | Set value to bigquery to use Big Query backend for analytics (postgres or bigquery) |
| studio.environment.NEXT_PUBLIC_ENABLE_LOGS | string | `"true"` |  |
| studio.environment.STUDIO_DEFAULT_ORGANIZATION | string | `"Default Organization"` |  |
| studio.environment.STUDIO_DEFAULT_PROJECT | string | `"Default Project"` |  |
| studio.environment.STUDIO_PORT | string | `"3000"` |  |
| studio.environment.SUPABASE_PUBLIC_URL | string | `"http://example.com"` |  |
| studio.fullnameOverride | string | `""` |  |
| studio.image.pullPolicy | string | `"IfNotPresent"` |  |
| studio.image.repository | string | `"supabase/studio"` |  |
| studio.image.tag | string | `"latest"` |  |
| studio.imagePullSecrets | list | `[]` |  |
| studio.livenessProbe | object | `{}` |  |
| studio.nameOverride | string | `""` |  |
| studio.nodeSelector | object | `{}` |  |
| studio.podAnnotations | object | `{}` |  |
| studio.podSecurityContext | object | `{}` |  |
| studio.readinessProbe | object | `{}` |  |
| studio.replicaCount | int | `1` |  |
| studio.resources | object | `{}` |  |
| studio.securityContext | object | `{}` |  |
| studio.service.port | int | `3000` |  |
| studio.service.type | string | `"ClusterIP"` |  |
| studio.serviceAccount.annotations | object | `{}` | Annotations to add to the service account |
| studio.serviceAccount.create | bool | `true` | Specifies whether a service account should be created |
| studio.serviceAccount.name | string | `""` | If not set and create is true, a name is generated using the fullname template |
| studio.tolerations | list | `[]` |  |

### Auth parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| auth.affinity | object | `{}` |  |
| auth.autoscaling.enabled | bool | `true` |  |
| auth.autoscaling.maxReplicas | int | `100` |  |
| auth.autoscaling.minReplicas | int | `1` |  |
| auth.autoscaling.targetCPUUtilizationPercentage | int | `80` |  |
| auth.enabled | bool | `true` | Enable auth provisioning |
| auth.environment.API_EXTERNAL_URL | string | `"http://example.com"` |  |
| auth.environment.DB_DRIVER | string | `"postgres"` |  |
| auth.environment.DB_PORT | int | `5432` |  |
| auth.environment.DB_SSL | string | `"disable"` |  |
| auth.environment.DB_USER | string | `"supabase_auth_admin"` |  |
| auth.environment.GOTRUE_API_HOST | string | `"0.0.0.0"` |  |
| auth.environment.GOTRUE_API_PORT | string | `"9999"` |  |
| auth.environment.GOTRUE_DISABLE_SIGNUP | string | `"false"` |  |
| auth.environment.GOTRUE_EXTERNAL_EMAIL_ENABLED | string | `"true"` |  |
| auth.environment.GOTRUE_EXTERNAL_PHONE_ENABLED | string | `"false"` |  |
| auth.environment.GOTRUE_JWT_ADMIN_ROLES | string | `"service_role"` |  |
| auth.environment.GOTRUE_JWT_AUD | string | `"authenticated"` |  |
| auth.environment.GOTRUE_JWT_DEFAULT_GROUP_NAME | string | `"authenticated"` |  |
| auth.environment.GOTRUE_JWT_EXP | string | `"3600"` |  |
| auth.environment.GOTRUE_MAILER_AUTOCONFIRM | string | `"true"` |  |
| auth.environment.GOTRUE_MAILER_URLPATHS_CONFIRMATION | string | `"/auth/v1/verify"` |  |
| auth.environment.GOTRUE_MAILER_URLPATHS_EMAIL_CHANGE | string | `"/auth/v1/verify"` |  |
| auth.environment.GOTRUE_MAILER_URLPATHS_INVITE | string | `"/auth/v1/verify"` |  |
| auth.environment.GOTRUE_MAILER_URLPATHS_RECOVERY | string | `"/auth/v1/verify"` |  |
| auth.environment.GOTRUE_SITE_URL | string | `"http://example.com"` |  |
| auth.environment.GOTRUE_SMS_AUTOCONFIRM | string | `"false"` |  |
| auth.environment.GOTRUE_SMTP_ADMIN_EMAIL | string | `"SMTP_ADMIN_MAIL"` |  |
| auth.environment.GOTRUE_SMTP_HOST | string | `"SMTP_HOST"` |  |
| auth.environment.GOTRUE_SMTP_PORT | string | `"SMTP_PORT"` |  |
| auth.environment.GOTRUE_SMTP_SENDER_NAME | string | `"SMTP_SENDER_NAME"` |  |
| auth.environment.GOTRUE_URI_ALLOW_LIST | string | `"*"` |  |
| auth.fullnameOverride | string | `""` |  |
| auth.image.pullPolicy | string | `"IfNotPresent"` |  |
| auth.image.repository | string | `"supabase/gotrue"` |  |
| auth.image.tag | string | `"latest"` |  |
| auth.imagePullSecrets | list | `[]` |  |
| auth.livenessProbe | object | `{}` |  |
| auth.nameOverride | string | `""` |  |
| auth.nodeSelector | object | `{}` |  |
| auth.podAnnotations | object | `{}` |  |
| auth.podSecurityContext | object | `{}` |  |
| auth.readinessProbe | object | `{}` |  |
| auth.replicaCount | int | `1` |  |
| auth.resources | object | `{}` |  |
| auth.securityContext | object | `{}` |  |
| auth.service.port | int | `9999` |  |
| auth.service.type | string | `"ClusterIP"` |  |
| auth.serviceAccount.annotations | object | `{}` | Annotations to add to the service account |
| auth.serviceAccount.create | bool | `true` | Specifies whether a service account should be created |
| auth.serviceAccount.name | string | `""` | The name of the service account to use. If not set and create is true, a name is generated using the fullname template |
| auth.tolerations | list | `[]` |  |

### Rest parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| rest.affinity | object | `{}` |  |
| rest.autoscaling.enabled | bool | `true` |  |
| rest.autoscaling.maxReplicas | int | `100` |  |
| rest.autoscaling.minReplicas | int | `1` |  |
| rest.autoscaling.targetCPUUtilizationPercentage | int | `80` |  |
| rest.enabled | bool | `true` | Enable postgrest provisioning |
| rest.environment.DB_DRIVER | string | `"postgres"` |  |
| rest.environment.DB_PORT | int | `5432` |  |
| rest.environment.DB_SSL | string | `"disable"` |  |
| rest.environment.DB_USER | string | `"authenticator"` |  |
| rest.environment.PGRST_APP_SETTINGS_JWT_EXP | int | `3600` |  |
| rest.environment.PGRST_DB_ANON_ROLE | string | `"anon"` |  |
| rest.environment.PGRST_DB_SCHEMAS | string | `"public,storage,graphql_public"` |  |
| rest.environment.PGRST_DB_USE_LEGACY_GUCS | bool | `false` |  |
| rest.fullnameOverride | string | `""` |  |
| rest.image.pullPolicy | string | `"IfNotPresent"` |  |
| rest.image.repository | string | `"postgrest/postgrest"` |  |
| rest.image.tag | string | `"latest"` |  |
| rest.imagePullSecrets | list | `[]` |  |
| rest.livenessProbe | object | `{}` |  |
| rest.nameOverride | string | `""` |  |
| rest.nodeSelector | object | `{}` |  |
| rest.podAnnotations | object | `{}` |  |
| rest.podSecurityContext | object | `{}` |  |
| rest.readinessProbe | object | `{}` |  |
| rest.resources | object | `{}` |  |
| rest.securityContext | object | `{}` |  |
| rest.service.port | int | `3000` |  |
| rest.service.type | string | `"ClusterIP"` |  |
| rest.serviceAccount.annotations | object | `{}` | Annotations to add to the service account |
| rest.serviceAccount.create | bool | `true` | Specifies whether a service account should be created |
| rest.serviceAccount.name | string | `""` | The name of the service account to use. If not set and create is true, a name is generated using the fullname template |
| rest.tolerations | list | `[]` |  |

### Realtime parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| realtime.affinity | object | `{}` |  |
| realtime.autoscaling.enabled | bool | `true` |  |
| realtime.autoscaling.maxReplicas | int | `100` |  |
| realtime.autoscaling.minReplicas | int | `1` |  |
| realtime.autoscaling.targetCPUUtilizationPercentage | int | `80` |  |
| realtime.enabled | bool | `true` | Enable realtime provisioning |
| realtime.environment.DB_AFTER_CONNECT_QUERY | string | `"SET search_path TO _realtime"` |  |
| realtime.environment.DB_ENC_KEY | string | `"supabaserealtime"` |  |
| realtime.environment.DB_PORT | int | `5432` |  |
| realtime.environment.DB_SSL | string | `"disable"` |  |
| realtime.environment.DB_USER | string | `"supabase_admin"` |  |
| realtime.environment.DNS_NODES | string | `"''"` |  |
| realtime.environment.ENABLE_TAILSCALE | string | `"false"` |  |
| realtime.environment.ERL_AFLAGS | string | `"-proto_dist inet_tcp"` |  |
| realtime.environment.FLY_ALLOC_ID | string | `"fly123"` |  |
| realtime.environment.FLY_APP_NAME | string | `"realtime"` |  |
| realtime.environment.PORT | string | `"4000"` |  |
| realtime.environment.SECRET_KEY_BASE | string | `"UpNVntn3cDxHJpq99YMc1T1AQgQpc8kfYTuRgBiYa15BLrx8etQoXz3gZv1/u2oq"` |  |
| realtime.fullnameOverride | string | `""` |  |
| realtime.image.pullPolicy | string | `"IfNotPresent"` |  |
| realtime.image.repository | string | `"supabase/realtime"` |  |
| realtime.image.tag | string | `"latest"` |  |
| realtime.imagePullSecrets | list | `[]` |  |
| realtime.livenessProbe | object | `{}` |  |
| realtime.nameOverride | string | `""` |  |
| realtime.nodeSelector | object | `{}` |  |
| realtime.podAnnotations | object | `{}` |  |
| realtime.podSecurityContext | object | `{}` |  |
| realtime.readinessProbe | object | `{}` |  |
| realtime.resources | object | `{}` |  |
| realtime.securityContext | object | `{}` |  |
| realtime.service.port | int | `4000` |  |
| realtime.service.type | string | `"ClusterIP"` |  |
| realtime.serviceAccount.annotations | object | `{}` | Annotations to add to the service account |
| realtime.serviceAccount.create | bool | `true` | Specifies whether a service account should be created |
| realtime.serviceAccount.name | string | `""` | The name of the service account to use. If not set and create is true, a name is generated using the fullname template |
| realtime.tolerations | list | `[]` |  |

### Meta parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| meta.affinity | object | `{}` |  |
| meta.autoscaling.enabled | bool | `true` |  |
| meta.autoscaling.maxReplicas | int | `100` |  |
| meta.autoscaling.minReplicas | int | `1` |  |
| meta.autoscaling.targetCPUUtilizationPercentage | int | `80` |  |
| meta.enabled | bool | `true` | Enable meta provisioning |
| meta.environment.DB_DRIVER | string | `"postgres"` |  |
| meta.environment.DB_PORT | int | `5432` |  |
| meta.environment.DB_SSL | string | `"disable"` |  |
| meta.environment.DB_USER | string | `"supabase_admin"` |  |
| meta.environment.PG_META_PORT | string | `"8080"` |  |
| meta.fullnameOverride | string | `""` |  |
| meta.image.pullPolicy | string | `"IfNotPresent"` |  |
| meta.image.repository | string | `"supabase/postgres-meta"` |  |
| meta.image.tag | string | `"latest"` |  |
| meta.imagePullSecrets | list | `[]` |  |
| meta.livenessProbe | object | `{}` |  |
| meta.nameOverride | string | `""` |  |
| meta.nodeSelector | object | `{}` |  |
| meta.podAnnotations | object | `{}` |  |
| meta.podSecurityContext | object | `{}` |  |
| meta.readinessProbe | object | `{}` |  |
| meta.replicaCount | int | `1` |  |
| meta.resources | object | `{}` |  |
| meta.securityContext | object | `{}` |  |
| meta.service.port | int | `8080` |  |
| meta.service.type | string | `"ClusterIP"` |  |
| meta.serviceAccount.annotations | object | `{}` | Annotations to add to the service account |
| meta.serviceAccount.create | bool | `true` | Specifies whether a service account should be created |
| meta.serviceAccount.name | string | `""` | The name of the service account to use. If not set and create is true, a name is generated using the fullname template |
| meta.tolerations | list | `[]` |  |

### Storage parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| storage.affinity | object | `{}` |  |
| storage.autoscaling.enabled | bool | `true` |  |
| storage.autoscaling.maxReplicas | int | `100` |  |
| storage.autoscaling.minReplicas | int | `1` |  |
| storage.autoscaling.targetCPUUtilizationPercentage | int | `80` |  |
| storage.enabled | bool | `true` | Enable storage provisioning |
| storage.environment.DB_DRIVER | string | `"postgres"` |  |
| storage.environment.DB_PORT | int | `5432` |  |
| storage.environment.DB_SSL | string | `"disable"` |  |
| storage.environment.DB_USER | string | `"supabase_storage_admin"` |  |
| storage.environment.FILE_SIZE_LIMIT | string | `"52428800"` |  |
| storage.environment.FILE_STORAGE_BACKEND_PATH | string | `"/var/lib/storage"` |  |
| storage.environment.GLOBAL_S3_BUCKET | string | `"stub"` |  |
| storage.environment.PGOPTIONS | string | `"-c search_path=storage,public"` |  |
| storage.environment.REGION | string | `"stub"` |  |
| storage.environment.STORAGE_BACKEND | string | `"file"` |  |
| storage.environment.TENANT_ID | string | `"stub"` |  |
| storage.fullnameOverride | string | `""` |  |
| storage.image.pullPolicy | string | `"IfNotPresent"` |  |
| storage.image.repository | string | `"supabase/storage-api"` |  |
| storage.image.tag | string | `"latest"` |  |
| storage.imagePullSecrets | list | `[]` |  |
| storage.livenessProbe | object | `{}` |  |
| storage.nameOverride | string | `""` |  |
| storage.nodeSelector | object | `{}` |  |
| storage.persistence.accessModes[0] | string | `"ReadWriteOnce"` |  |
| storage.persistence.annotations | object | `{}` |  |
| storage.persistence.class | string | `""` |  |
| storage.persistence.enabled | bool | `true` |  |
| storage.persistence.size | string | `"10Gi"` |  |
| storage.persistence.storageClassName | string | `""` |  |
| storage.podAnnotations | object | `{}` |  |
| storage.podSecurityContext | object | `{}` |  |
| storage.readinessProbe | object | `{}` |  |
| storage.replicaCount | int | `1` |  |
| storage.resources | object | `{}` |  |
| storage.securityContext | object | `{}` |  |
| storage.service.port | int | `5000` |  |
| storage.service.type | string | `"ClusterIP"` |  |
| storage.serviceAccount.annotations | object | `{}` | Annotations to add to the service account |
| storage.serviceAccount.create | bool | `true` | Specifies whether a service account should be created |
| storage.serviceAccount.name | string | `""` | The name of the service account to use. If not set and create is true, a name is generated using the fullname template |
| storage.tolerations | list | `[]` |  |

### Image Proxy parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| imgproxy.affinity | object | `{}` |  |
| imgproxy.autoscaling.enabled | bool | `true` |  |
| imgproxy.autoscaling.maxReplicas | int | `100` |  |
| imgproxy.autoscaling.minReplicas | int | `1` |  |
| imgproxy.autoscaling.targetCPUUtilizationPercentage | int | `80` |  |
| imgproxy.enabled | bool | `true` | Enable imgproxy provisioning |
| imgproxy.environment.IMGPROXY_BIND | string | `":5001"` |  |
| imgproxy.environment.IMGPROXY_ENABLE_WEBP_DETECTION | string | `"true"` |  |
| imgproxy.environment.IMGPROXY_LOCAL_FILESYSTEM_ROOT | string | `"/"` |  |
| imgproxy.environment.IMGPROXY_USE_ETAG | string | `"true"` |  |
| imgproxy.fullnameOverride | string | `""` |  |
| imgproxy.image.pullPolicy | string | `"IfNotPresent"` |  |
| imgproxy.image.repository | string | `"darthsim/imgproxy"` |  |
| imgproxy.image.tag | string | `"latest"` |  |
| imgproxy.imagePullSecrets | list | `[]` |  |
| imgproxy.livenessProbe | object | `{}` |  |
| imgproxy.nameOverride | string | `""` |  |
| imgproxy.nodeSelector | object | `{}` |  |
| imgproxy.persistence.accessModes[0] | string | `"ReadWriteOnce"` |  |
| imgproxy.persistence.annotations | object | `{}` |  |
| imgproxy.persistence.class | string | `""` |  |
| imgproxy.persistence.enabled | bool | `true` |  |
| imgproxy.persistence.size | string | `"10Gi"` |  |
| imgproxy.persistence.storageClassName | string | `""` |  |
| imgproxy.podAnnotations | object | `{}` |  |
| imgproxy.podSecurityContext | object | `{}` |  |
| imgproxy.readinessProbe | object | `{}` |  |
| imgproxy.replicaCount | int | `1` |  |
| imgproxy.resources | object | `{}` |  |
| imgproxy.securityContext | object | `{}` |  |
| imgproxy.service.port | int | `5001` |  |
| imgproxy.service.type | string | `"ClusterIP"` |  |
| imgproxy.serviceAccount.annotations | object | `{}` | Annotations to add to the service account |
| imgproxy.serviceAccount.create | bool | `true` | Specifies whether a service account should be created |
| imgproxy.serviceAccount.name | string | `""` | The name of the service account to use. If not set and create is true, a name is generated using the fullname template |
| imgproxy.tolerations | list | `[]` |  |

### Kong parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| kong.affinity | object | `{}` |  |
| kong.autoscaling.enabled | bool | `true` |  |
| kong.autoscaling.maxReplicas | int | `100` |  |
| kong.autoscaling.minReplicas | int | `1` |  |
| kong.autoscaling.targetCPUUtilizationPercentage | int | `80` |  |
| kong.enabled | bool | `true` | Enable kong provisioning |
| kong.environment.KONG_DATABASE | string | `"off"` |  |
| kong.environment.KONG_DECLARATIVE_CONFIG | string | `"/usr/local/kong/kong.yml"` |  |
| kong.environment.KONG_DNS_ORDER | string | `"LAST,A,CNAME"` |  |
| kong.environment.KONG_LOG_LEVEL | string | `"warn"` |  |
| kong.environment.KONG_NGINX_PROXY_PROXY_BUFFERS | string | `"64 160k"` |  |
| kong.environment.KONG_NGINX_PROXY_PROXY_BUFFER_SIZE | string | `"160k"` |  |
| kong.environment.KONG_PLUGINS | string | `"request-transformer,cors,key-auth,acl,basic-auth"` |  |
| kong.fullnameOverride | string | `""` |  |
| kong.image.pullPolicy | string | `"IfNotPresent"` |  |
| kong.image.repository | string | `"kong"` |  |
| kong.image.tag | string | `"latest"` |  |
| kong.imagePullSecrets | list | `[]` |  |
| kong.ingress.annotations."nginx.ingress.kubernetes.io/rewrite-target" | string | `"/"` |  |
| kong.ingress.className | string | `"nginx"` |  |
| kong.ingress.enabled | bool | `true` |  |
| kong.ingress.hosts[0].host | string | `"example.com"` |  |
| kong.ingress.hosts[0].paths[0].path | string | `"/"` |  |
| kong.ingress.hosts[0].paths[0].pathType | string | `"Prefix"` |  |
| kong.ingress.tls | list | `[]` |  |
| kong.livenessProbe | object | `{}` |  |
| kong.nameOverride | string | `""` |  |
| kong.nodeSelector | object | `{}` |  |
| kong.podAnnotations | object | `{}` |  |
| kong.podSecurityContext | object | `{}` |  |
| kong.readinessProbe | object | `{}` |  |
| kong.replicaCount | int | `1` |  |
| kong.resources | object | `{}` |  |
| kong.securityContext | object | `{}` |  |
| kong.service.port | int | `8000` |  |
| kong.service.type | string | `"ClusterIP"` |  |
| kong.serviceAccount.annotations | object | `{}` | Annotations to add to the service account |
| kong.serviceAccount.create | bool | `true` | Specifies whether a service account should be created |
| kong.serviceAccount.name | string | `""` | The name of the service account to use. If not set and create is true, a name is generated using the fullname template |
| kong.tolerations | list | `[]` |  |

### Analytics parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| analytics.affinity | object | `{}` |  |
| analytics.autoscaling.enabled | bool | `true` |  |
| analytics.autoscaling.maxReplicas | int | `100` |  |
| analytics.autoscaling.minReplicas | int | `1` |  |
| analytics.autoscaling.targetCPUUtilizationPercentage | int | `80` |  |
| analytics.bigQuery | object | `{}` (See [values.yaml]) | Enable Big Query backend for analytics |
| analytics.enabled | bool | `true` | Enable analytics provisioning |
| analytics.environment.DB_DRIVER | string | `"postgresql"` |  |
| analytics.environment.DB_PORT | int | `5432` |  |
| analytics.environment.DB_SCHEMA | string | `"_analytics"` |  |
| analytics.environment.DB_USERNAME | string | `"supabase_admin"` |  |
| analytics.environment.FEATURE_FLAG_OVERRIDE | string | `"multibackend=true"` |  |
| analytics.environment.LOGFLARE_NODE_HOST | string | `"127.0.0.1"` |  |
| analytics.environment.LOGFLARE_SINGLE_TENANT | string | `"true"` |  |
| analytics.environment.LOGFLARE_SUPABASE_MODE | string | `"true"` |  |
| analytics.fullnameOverride | string | `""` |  |
| analytics.image.pullPolicy | string | `"IfNotPresent"` |  |
| analytics.image.repository | string | `"supabase/logflare"` |  |
| analytics.image.tag | string | `"latest"` |  |
| analytics.imagePullSecrets | list | `[]` |  |
| analytics.livenessProbe | object | `{}` |  |
| analytics.nameOverride | string | `""` |  |
| analytics.nodeSelector | object | `{}` |  |
| analytics.podAnnotations | object | `{}` |  |
| analytics.podSecurityContext | object | `{}` |  |
| analytics.readinessProbe | object | `{}` |  |
| analytics.replicaCount | int | `1` |  |
| analytics.resources | object | `{}` |  |
| analytics.securityContext | object | `{}` |  |
| analytics.service.port | int | `4000` |  |
| analytics.service.type | string | `"ClusterIP"` |  |
| analytics.serviceAccount.annotations | object | `{}` | Annotations to add to the service account |
| analytics.serviceAccount.create | bool | `true` | Specifies whether a service account should be created |
| analytics.serviceAccount.name | string | `""` | The name of the service account to use. If not set and create is true, a name is generated using the fullname template |
| analytics.tolerations | list | `[]` |  |

### Vector parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| vector.affinity | object | `{}` |  |
| vector.autoscaling.enabled | bool | `true` |  |
| vector.autoscaling.maxReplicas | int | `100` |  |
| vector.autoscaling.minReplicas | int | `1` |  |
| vector.autoscaling.targetCPUUtilizationPercentage | int | `80` |  |
| vector.enabled | bool | `true` | Enable vector provisioning |
| vector.fullnameOverride | string | `""` |  |
| vector.image.pullPolicy | string | `"IfNotPresent"` |  |
| vector.image.repository | string | `"timberio/vector"` |  |
| vector.image.tag | string | `"latest"` |  |
| vector.imagePullSecrets | list | `[]` |  |
| vector.livenessProbe | object | `{}` |  |
| vector.nameOverride | string | `""` |  |
| vector.nodeSelector | object | `{}` |  |
| vector.podAnnotations | object | `{}` |  |
| vector.podSecurityContext | object | `{}` |  |
| vector.readinessProbe | object | `{}` |  |
| vector.replicaCount | int | `1` |  |
| vector.resources | object | `{}` |  |
| vector.securityContext | object | `{}` |  |
| vector.service.port | int | `9001` |  |
| vector.service.type | string | `"ClusterIP"` |  |
| vector.serviceAccount.annotations | object | `{}` | Annotations to add to the service account |
| vector.serviceAccount.create | bool | `true` | Specifies whether a service account should be created |
| vector.serviceAccount.name | string | `""` | The name of the service account to use. If not set and create is true, a name is generated using the fullname template |
| vector.tolerations | list | `[]` |  |

### Functions parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| functions.affinity | object | `{}` |  |
| functions.autoscaling.enabled | bool | `true` |  |
| functions.autoscaling.maxReplicas | int | `100` |  |
| functions.autoscaling.minReplicas | int | `1` |  |
| functions.autoscaling.targetCPUUtilizationPercentage | int | `80` |  |
| functions.enabled | bool | `true` | Enable functions provisioning |
| functions.environment.DB_DRIVER | string | `"postgresql"` |  |
| functions.environment.DB_PORT | int | `5432` |  |
| functions.environment.DB_SSL | string | `"disable"` |  |
| functions.environment.DB_USERNAME | string | `"supabase_functions_admin"` |  |
| functions.fullnameOverride | string | `""` |  |
| functions.image.pullPolicy | string | `"IfNotPresent"` |  |
| functions.image.repository | string | `"supabase/edge-runtime"` |  |
| functions.image.tag | string | `"latest"` |  |
| functions.imagePullSecrets | list | `[]` |  |
| functions.livenessProbe | object | `{}` |  |
| functions.nameOverride | string | `""` |  |
| functions.nodeSelector | object | `{}` |  |
| functions.podAnnotations | object | `{}` |  |
| functions.podSecurityContext | object | `{}` |  |
| functions.readinessProbe | object | `{}` |  |
| functions.replicaCount | int | `1` |  |
| functions.resources | object | `{}` |  |
| functions.securityContext | object | `{}` |  |
| functions.service.port | int | `9000` |  |
| functions.service.type | string | `"ClusterIP"` |  |
| functions.serviceAccount.annotations | object | `{}` | Annotations to add to the service account |
| functions.serviceAccount.create | bool | `true` | Specifies whether a service account should be created |
| functions.serviceAccount.name | string | `""` | The name of the service account to use. If not set and create is true, a name is generated using the fullname template |
| functions.tolerations | list | `[]` |  |

### Minio parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| minio.affinity | object | `{}` |  |
| minio.autoscaling.enabled | bool | `true` |  |
| minio.autoscaling.maxReplicas | int | `100` |  |
| minio.autoscaling.minReplicas | int | `1` |  |
| minio.autoscaling.targetCPUUtilizationPercentage | int | `80` |  |
| minio.enabled | bool | `false` |  |
| minio.environment | object | `{}` |  |
| minio.fullnameOverride | string | `""` |  |
| minio.image.pullPolicy | string | `"IfNotPresent"` |  |
| minio.image.repository | string | `"minio/minio"` |  |
| minio.image.tag | string | `"latest"` |  |
| minio.imagePullSecrets | list | `[]` |  |
| minio.livenessProbe | object | `{}` |  |
| minio.nameOverride | string | `""` |  |
| minio.nodeSelector | object | `{}` |  |
| minio.persistence.accessModes[0] | string | `"ReadWriteOnce"` |  |
| minio.persistence.annotations | object | `{}` |  |
| minio.persistence.class | string | `""` |  |
| minio.persistence.enabled | bool | `false` |  |
| minio.persistence.size | string | `"10Gi"` |  |
| minio.persistence.storageClassName | string | `""` |  |
| minio.podAnnotations | object | `{}` |  |
| minio.podSecurityContext | object | `{}` |  |
| minio.readinessProbe | object | `{}` |  |
| minio.replicaCount | int | `1` |  |
| minio.resources | object | `{}` |  |
| minio.securityContext | object | `{}` |  |
| minio.service.port | int | `9000` |  |
| minio.service.type | string | `"ClusterIP"` |  |
| minio.serviceAccount.annotations | object | `{}` | Annotations to add to the service account |
| minio.serviceAccount.create | bool | `true` | Specifies whether a service account should be created |
| minio.serviceAccount.name | string | `""` | The name of the service account to use. If not set and create is true, a name is generated using the fullname template |
| minio.tolerations | list | `[]` |  |

----------------------------------------------

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.13.1](https://github.com/norwoodj/helm-docs/releases/v1.13.1). 

To update run `helm-docs -t README.md.gotmpl -o README.md -b for-the-badge`.
