# Supabase for Kubernetes with Helm 3

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
