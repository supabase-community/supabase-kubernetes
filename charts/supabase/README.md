# Supabase for Kubernetes with Helm

This directory contains the configurations and scripts required to run Supabase inside a Kubernetes cluster.

## Disclamer

We use [supabase/postgres](https://hub.docker.com/r/supabase/postgres) to create and manage the Postgres database. This permit you to use replication if needed but you'll have to use the Postgres image provided Supabase or build your own on top of it. You can also choose to use other databases provider like [StackGres](https://stackgres.io/) or [Postgres Operator](https://github.com/zalando/postgres-operator).

For the moment we are using a root container to permit the installation of the missing `pgjwt` and `wal2json` extension inside the `initdbScripts`. This is considered a security issue, but you can use your own Postgres image instead with the extension already installed to prevent this. We provide an example of `Dockerfile`for this purpose, you can use [ours](https://hub.docker.com/r/tdeoliv/supabase-bitnami-postgres) or build and host it on your own.

The database configuration we provide is an example using only one master. If you want to go to production, we highly recommend you to use a replicated database.

## Usage example

> For this section we're using Minikube and Docker to create a Kubernetes cluster


1. Create a cluster with Minikube:

    ```bash
    minikube start --driver=docker
    minikube addons enable ingress
    echo "$(minikube ip)     supabase.local" | sudo tee -a /etc/hosts > /dev/null
    ```

2. Add the Supabase Helm repository:

    ```bash
    helm repo add supabase https://supabase-community.github.io/supabase-kubernetes
    ```
  
3. Install Supabase:

    ```bash
    helm install demo supabase/supabase
    ```

4. The first deployment can take some time to complete (especially auth service). You can view the status of the pods using:

    ```bash
    kubectl get pod -l app.kubernetes.io/instance=demo
    
    NAME                                      READY   STATUS    RESTARTS      AGE
    demo-supabase-auth-xxxxxxxxxx-xxxxx       1/1     Running   0             47s
    demo-supabase-db-0-xxxxxxxxxx-xxxxx       1/1     Running   0             47s
    demo-supabase-functions-xxxxxxxxxx-xxxxx  1/1     Running   0             47s
    demo-supabase-imgproxy-xxxxxxxxxx-xxxxx   1/1     Running   0             47s
    demo-supabase-kong-xxxxxxxxxx-xxxxx       1/1     Running   0             47s
    demo-supabase-meta-xxxxxxxxxx-xxxxx       1/1     Running   0             47s
    demo-supabase-realtime-xxxxxxxxxx-xxxxx   1/1     Running   0             47s
    demo-supabase-rest-xxxxxxxxxx-xxxxx       1/1     Running   0             47s
    demo-supabase-storage-xxxxxxxxxx-xxxxx    1/1     Running   0             47s
    demo-supabase-studio-xxxxxxxxxx-xxxxx     1/1     Running   0             47s
    ```

    > **Note:** `analytics` (Logflare) and `vector` are disabled by default.
    > To enable the Logs section in Studio, set both `deployment.analytics.enabled=true` and `deployment.vector.enabled=true`.

5. Open Supabase Studio in your browser: http://supabase.local

   Use the **default credentials** below (for local development only):
   - **Username:** `supabase`
   - **Password:** `this_password_is_insecure_and_should_be_updated`

6. Uninstall Supabase example:

    ```bash
    helm uninstall demo
    minikube delete
    sudo sed -i '/[[:space:]]supabase\.local$/d' /etc/hosts
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
> You can use the [JWT Tool](https://supabase.com/docs/guides/self-hosting/docker#generate-and-configure-api-keys) to generate anon and service keys.

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
    password: <db-password>
    database: <supabase-database-name>
```

The secret can be created with kubectl via command-line:

> If you depend on database providers like [StackGres](https://stackgres.io/), [Postgres Operator](https://github.com/zalando/postgres-operator) or self-hosted Postgres instance, configure `secret.db.host`/`secret.db.port` (or map `host`/`port` from `secret.db.secretRef`) and adjust any additional Postgres attributes (e.g. `PGPORT`) as needed. Refer to [values.yaml](values.yaml) for more details.

### Dashboard secret

By default, a username and password is required to access the Supabase Studio dashboard. Simply change them at:

```yaml
secret:
  dashboard:
    username: supabase
    password: this_password_is_insecure_and_should_be_updated
```

### Analytics secret

Analytics/Vector (logs) are **disabled by default**. To enable the Logs feature in Studio, set both `deployment.analytics.enabled=true` and `deployment.vector.enabled=true`.

A new logflare secret API key is required for securing communication between all of the Supabase services. To set the secret, generate a new 32 characters long secret similar to the step [above](#jwt-secret).

```yaml
secret:
  analytics:
    publicAccessToken: "your-super-secret-and-long-logflare-key-public"
    privateAccessToken: "your-super-secret-and-long-logflare-key-private"
```

### BigQuery secret

When using BigQuery as analytics backend, provide a GCP service account JSON key via secret values:

```yaml
bigQuery:
  enabled: true

secret:
  bigquery:
    gcloudJson: '{"type":"service_account", ...}'
```

You can also reference an existing Kubernetes Secret:

```yaml
secret:
  bigquery:
    secretRef: my-bigquery-secret
    secretRefKey:
      gcloudJson: gcloud.json
```

### Autoscaling

Autoscaling is disabled by default. To enable it for a component, set `autoscaling.<component>.enabled=true` and configure CPU and/or memory requests for that component. Kubernetes HPA utilization metrics require metrics-server and container `resources.requests`.

```yaml
deployment:
  auth:
    resources:
      requests:
        cpu: 100m

autoscaling:
  auth:
    enabled: true
    minReplicas: 1
    maxReplicas: 5
    targetCPUUtilizationPercentage: 80
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

  environment:
    auth:
      - name: API_EXTERNAL_URL
        value: http://supabase.local
      - name: GOTRUE_SITE_URL
        value: http://supabase.local
      - name: GOTRUE_URI_ALLOW_LIST
        value: "*"
      - name: GOTRUE_EXTERNAL_AZURE_SECRET
        valueFrom:
          secretKeyRef:
            name: azure-secret
            key: secret
  ```
3. (Optional) Enable internal minio deployment
  ```yaml
  minio:
    enabled: true
  ```

### Environment variables

> [!NOTE]
> Starting with chart version `0.7.1`, `environment.<component>` is an array of Kubernetes env entries instead of a map. Each entry supports `value:` or `valueFrom:`.

Example:
```yaml
environment:
  auth:
    - name: GOTRUE_API_HOST
      value: "0.0.0.0"
    - name: GOTRUE_EXTERNAL_AZURE_SECRET
      valueFrom:
        secretKeyRef:
          name: azure-secret
          key: secret
```

User-provided entries are rendered after the chart defaults, so any chart-managed env var can be overridden by adding an entry with the same `name` to the component array.

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

#### `0.6.x` to `0.7.x`

This chart version bumps the default Postgres image from `15.8.1.085` to `17.6.1.136` and the `initDb` image from `15-alpine` to `17-alpine`.

> **Warning:** Before upgrading, make sure you have a recent backup of your database. Major version upgrades can fail and may require rollback.
>
> **Important:** We strongly recommend taking a snapshot or backup of the DB PersistentVolumeClaim (PVC) before running the upgrade. The upgrade script modifies the data directory in-place, and a PVC snapshot is the safest way to recover if anything goes wrong.

##### New installs

New installations automatically use Postgres 17:

```bash
helm repo update
helm install demo supabase/supabase
```

##### Upgrading an existing Postgres 15 deployment

For existing deployments running Postgres 15, use the `scripts/upgrade-pg17.sh` helper provided in the chart. This script is the Kubernetes equivalent of the Docker `utils/upgrade-pg17.sh` and performs an in-place `pg_upgrade`:

```bash
bash scripts/upgrade-pg17.sh -n <namespace> -r <release> [-c <chart-path>] [--yes]
```

Required flags:

- `-n, --namespace`: Kubernetes namespace where Supabase is deployed
- `-r, --release`: Helm release name

Optional flags:

- `-c, --chart`: Path to this Helm chart (auto-detected by default)
- `--yes, -y`: Skip confirmation prompts

What the script does:

1. Builds a PG 17 upgrade tarball from the Supabase `supabase/postgres:17.6.1.063` image
2. Scales down all Supabase services
3. Runs `pg_upgrade` (Postgres 15 → 17) inside a temporary PG 15 pod
4. Runs `complete.sh` inside a temporary PG 17 pod for post-upgrade patches
5. Swaps the data directories inside the DB PVC
6. Upgrades the Helm release to the PG 17 image (`supabase/postgres:17.6.1.136`)
7. Applies the PG 17 role and extension migrations
8. Verifies the final Postgres version and extension list

The original Postgres 15 data is preserved inside the DB PVC as `postgres-data.bak.pg15`. You can remove it after confirming the upgrade was successful.

##### Rollback (if needed)

If the upgrade fails before the Helm release is updated, scale the DB StatefulSet back to `1` to resume Postgres 15. If the Helm release was already updated to Postgres 17, use the `scripts/rollback-pg15.sh` helper to roll back to Postgres 15:

```bash
bash scripts/rollback-pg15.sh -n <namespace> -r <release> [-c <chart-path>] [--yes]
```

Required flags:

- `-n, --namespace`: Kubernetes namespace where Supabase is deployed
- `-r, --release`: Helm release name

Optional flags:

- `-c, --chart`: Path to this Helm chart (auto-detected by default)
- `--yes, -y`: Skip confirmation prompts

What the script does:

1. Scales down all Supabase services belonging to the release
2. Removes the Postgres 17 data directory (`postgres-data`) from the DB PVC
3. Restores the Postgres 15 backup directory (`postgres-data.bak.pg15`)
4. Fixes pgsodium volume ownership for the PG 15 image
5. Rolls the Helm release back to the PG 15 image tags
6. Scales up the DB StatefulSet
7. Verifies that Postgres reports version 15

> **Warning:** This rollback is destructive. The Postgres 17 data directory is removed from the PVC. Ensure you have a snapshot or backup of the DB PVC before rolling back, especially if you need to preserve the Postgres 17 state.

If you prefer to perform the rollback manually, the steps are:

```bash
helm upgrade <release> <chart-path> \
  --set image.db.tag=15.8.1.085 \
  --set image.initDb.tag=15-alpine \
  --reuse-values -n <namespace>

kubectl scale statefulset -n <namespace> <db-statefulset> --replicas=0

kubectl run supabase-rollback -n <namespace> --image=alpine:3.20 \
  --restart=Never \
  --overrides='{
    "apiVersion": "v1",
    "spec": {
      "containers": [{
        "name": "rollback",
        "image": "alpine:3.20",
        "command": ["sh", "-c"],
        "args": ["rm -rf /mnt/db-data/postgres-data && mv /mnt/db-data/postgres-data.bak.pg15 /mnt/db-data/postgres-data"],
        "volumeMounts": [{"name": "db-data", "mountPath": "/mnt/db-data"}]
      }],
      "volumes": [{"name": "db-data", "persistentVolumeClaim": {"claimName": "<db-pvc>"}}],
      "restartPolicy": "Never"
    }
  }'

kubectl run supabase-fix-owner -n <namespace> --image=supabase/postgres:15.8.1.085 \
  --restart=Never \
  --overrides='{
    "apiVersion": "v1",
    "spec": {
      "containers": [{
        "name": "fix-owner",
        "image": "supabase/postgres:15.8.1.085",
        "command": ["sh", "-c"],
        "args": ["chown -R postgres:postgres /vol/"],
        "volumeMounts": [{"name": "pgsodium", "mountPath": "/vol"}]
      }],
      "volumes": [{"name": "pgsodium", "persistentVolumeClaim": {"claimName": "<pgsodium-pvc>"}}],
      "restartPolicy": "Never"
    }
  }'

kubectl scale statefulset -n <namespace> <db-statefulset> --replicas=1
```

##### Staying on Postgres 15

If you are not ready to upgrade, you can keep using Postgres 15 by overriding the image tags:

```bash
helm upgrade <release> <chart-path> \
  --set image.db.tag=15.8.1.085 \
  --set image.initDb.tag=15-alpine \
  --reuse-values -n <namespace>
```

#### `0.0.x` to `0.1.x`

* `supabase/postgres` is updated from `14.1` to `15.1`, which warrants backing up all your data before proceeding to update to the next major version.
* Intialization scripts for `supabase/postgres` has been reworked and matched closely to the [Docker Compose](https://github.com/supabase/supabase/blob/master/docker/docker-compose.yml) version. Further tweaks to the scripts are needed to ensure backward-compatibility.
* Migration scripts are now exposed at `db.config`, which will be mounted at `/docker-entrypoint-initdb.d/migrations/`. Simply copy your migration files from your local project's `supabase/migration` and populate the `db.config`.
* Ingress are now limited to `kong` & `db` services. This is by design to limit entry to the stack through secure `kong` service.
* `kong.yaml` has been modified to follow [Docker kong.yaml](https://github.com/supabase/supabase/blob/master/docker/volumes/api/kong.yml) template.
* `supabase/storage` does not comes with pre-populated `/var/lib/storage`, therefore an `emptyDir` will be created if persistence is disabled. This might be incompatible with previous version if the persistent storage location is set to location other than specified above.
* `supabase/vector` requires read access to the `/var/log/pods` directory. When run in a Kubernetes cluster this can be provided with a [hostPath](https://kubernetes.io/docs/concepts/storage/volumes/#hostpath) volume.
