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

# Create JWT secret
kubectl -n default create secret generic demo-supabase-jwt \
  --from-literal=anonKey='eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.ewogICAgInJvbGUiOiAiYW5vbiIsCiAgICAiaXNzIjogInN1cGFiYXNlIiwKICAgICJpYXQiOiAxNjc1NDAwNDAwLAogICAgImV4cCI6IDE4MzMxNjY4MDAKfQ.ztuiBzjaVoFHmoljUXWmnuDN6QU2WgJICeqwyzyZO88' \
  --from-literal=serviceKey='eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.ewogICAgInJvbGUiOiAic2VydmljZV9yb2xlIiwKICAgICJpc3MiOiAic3VwYWJhc2UiLAogICAgImlhdCI6IDE2NzU0MDA0MDAsCiAgICAiZXhwIjogMTgzMzE2NjgwMAp9.qNsmXzz4tG7eqJPh1Y58DbtIlJBauwpqx39UF-MwM8k' \
  --from-literal=secret='abcdefghijklmnopqrstuvwxyz123456'

# Create SMTP secret
kubectl -n default create secret generic demo-supabase-smtp \
  --from-literal=username='your-mail@example.com' \
  --from-literal=password='example123456'

# Create DB secret
kubectl -n default create secret generic demo-supabase-db \
  --from-literal=username='postgres' \
  --from-literal=password='example123456' 

# Install the chart
helm -n default install demo -f values.example.yaml .
```

The first deployment can take some time to complete (especially auth service). You can view the status of the pods using:

```bash
kubectl -n default get pod 

NAME                                      READY   STATUS    RESTARTS      AGE
demo-supabase-auth-78547c5c8d-chkbm       1/1     Running   2 (40s ago)   47s
demo-supabase-db-5bc75fbf56-4cxcv         1/1     Running   0             47s
demo-supabase-kong-8c666695f-5vzwt        1/1     Running   0             47s
demo-supabase-meta-6779677c7-s77qq        1/1     Running   0             47s
demo-supabase-realtime-6b55986d7d-csnr7   1/1     Running   0             47s
demo-supabase-rest-5d864469d-bk5rv        1/1     Running   0             47s
demo-supabase-storage-6c878dcbd4-zzzcv    1/1     Running   0             47s
```

### Tunnel with Minikube

When the installation will be complete you'll be able to create a tunnel using minikube:

```bash
# First, enable the ingress addon in Minikube
minikube addons enable ingress

# Then enable the tunnel (will need sudo credentials because you are opening Port 80/443 on your local machine)
minikube tunnel
```

If you just use the `value.example.yaml` file, you can access the API or the Studio App using the following endpoints:

- <http://api.localhost>
- <http://studio.localhost>

### Uninstall

```Bash
# Uninstall Helm chart
helm -n default uninstall demo 

# Delete secrets
kubectl -n default delete secret demo-supabase-db
kubectl -n default delete secret demo-supabase-jwt
kubectl -n default delete secret demo-supabase-smtp
```

## Customize

You should consider to adjust the following values in `values.yaml`:

- `JWT_SECRET_NAME`: Reference to Kubernetes secret with JWT secret data `secret`, `anonKey` & `serviceKey`
- `SMTP_SECRET_NAME`: Reference to Kubernetes secret with SMTP credentials `username` & `password`
- `DB_SECRET_NAME`: Reference to Kubernetes secret with Postgres credentials `username` & `password`
- `RELEASE_NAME`: Name used for helm release
- `NAMESPACE`: Namespace used for the helm release
- `API.EXAMPLE.COM` URL to Kong API
- `STUDIO.EXAMPLE.COM` URL to Studio

If you want to use mail, consider to adjust the following values in `values.yaml`:

- `SMTP_ADMIN_MAIL`
- `SMTP_HOST`
- `SMTP_PORT`
- `SMTP_SENDER_NAME`

### JWT Secret

We encourage you to use your own JWT keys by generating a new Kubernetes secret and reference it in `values.yaml`:

```yaml
  jwt:
    secretName: "JWT_SECRET_NAME"
```

The secret can be created with kubectl via command-line:

```bash
kubectl -n NAMESPACE create secret generic JWT_SECRET_NAME \
  --from-literal=secret='JWT_TOKEN_AT_LEAST_32_CHARACTERS_LONG' \
  --from-literal=anonKey='JWT_ANON_KEY' \
  --from-literal=serviceKey='JWT_SERVICE_KEY'
```

> 32 characters long secret can be generated with `openssl rand 64 | base64`
> You can use the [JWT Tool](https://supabase.com/docs/guides/hosting/overview#api-keys) to generate anon and service keys.

### SMTP Secret

Connection credentials for the SMTP mail server will also be provided via Kubernetes secret referenced in `values.yaml`:

```yaml
  smtp:
    secretName: "SMTP_SECRET_NAME"
```

The secret can be created with kubectl via command-line:

```bash
kubectl -n NAMESPACE create secret generic SMTP_SECRET_NAME \
  --from-literal=username='SMTP_USER' \
  --from-literal=password='SMTP_PASSWORD'
```

### DB Secret

DB credentials will also be stored in a Kubernetes secret and referenced in `values.yaml`:

```yaml
  db:
    secretName: "DB_SECRET_NAME"
```

The secret can be created with kubectl via command-line:

```bash
kubectl -n NAMESPACE create secret generic DB_SECRET_NAME \
  --from-literal=username='DB_USER' \
  --from-literal=password='PW_USER'
```

> If you depend on database providers like [StackGres](https://stackgres.io/) or [Postgres Operator](https://github.com/zalando/postgres-operator) you only need to reference the already existing secret in `values.yaml`.

## How to use in Production

We didn't provide a complete configuration to go production because of the multiple possibility.

But here are the important points you have to think about:

- Use a replicated version of the Postgres database.
- Add SSL to the Postgres database.
- Add SSL configuration to the ingresses endpoints using either the `cert-manager` or a LoadBalancer provider.
- Change the domain used in the ingresses endpoints.
- Generate a new secure JWT Secret.

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
