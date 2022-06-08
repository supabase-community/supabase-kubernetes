# Supabase for Kubernetes with Helm 3

This directory contains the configurations and scripts required to run Supabase inside a Kubernetes cluster.

## Disclamer

We use [supabase/postgres](https://hub.docker.com/r/supabase/postgres) to create and manage the Postgres database. This permit you to use replication if needed but you'll have to use the Postgres image provided Supabase or build your own on top of it. You can also choose to use other databases provider like [StackGres](https://stackgres.io/) or [Postgres Operator](https://github.com/zalando/postgres-operator).

For the moment we are using a root container to permit the installation of the missing `pgjwt` and `wal2json` extension inside the `initdbScripts`. This is considered a security issue, but you can use your own Postgres image instead with the extension already installed to prevent this. We provide an example of `Dockerfile`for this purpose, you can use [ours](https://hub.docker.com/r/tdeoliv/supabase-bitnami-postgres) or build and host it on your own.

The database configuration we provide is an example using only one master. If you want to go to production, we highly recommend you to use a replicated database.

## Quickstart

> For this section we're using Minikube and Docker to create a Kubernetes cluster

```bash
git clone https://supabase-community.github.io/supabase-kubernetes
cd supabase-kubernetes/charts/supabase/
helm install --create-namespace -n default demo -f values.example.yaml .
```

You should consider to adjust the following values in `values.yaml`:

- `YOUR_SUPER_SECRET_JWT_TOKEN_WITH_AT_LEAST_32_CHARACTERS_LONG`: With a generated secret key (`openssl rand 64 | base64`).
- `JWT_ANON_KEY`: A JWT signed with the key above and the role `anon`.
- `JWT_SERVICE_KEY`: A JWT signed with the key above and the role `service_role`. You can use the [JWT Tool](https://supabase.com/docs/guides/hosting/overview#api-keys) to generate your keys.
- `YOUR_VERY_HARD_PASSWORD_FOR_DATABASE`: Postgres root password for the created database.
- `RELEASE_NAME`: Name used for helm release.
- `NAMESPACE`: Namespace used for the helm release.
- `API.EXAMPLE.COM` URL to Kong API.
- `STUDIO.EXAMPLE.COM` URL to Studio.

If you wan't to use mail, consider to adjust the following values in `values.yaml`:

- `YOUR_MAIL`
- `YOUR_MAIL_SERVER`
- `YOUR_MAIL_SERVER_PORT`
- `YOUR_MAIL_SERVER_USER`
- `YOUR_MAIL_SERVER_PASSWORD`
- `YOUR_MAIL_SERVER_SENDER_NAME`

The first deployment can take some time to complete (especially auth service). You can view the status of the pods using:

```bash
kubectl get pod -n NAMESPACE

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
minikube tunnel
```

If you just use the `value.example.yaml` file, you can access the API or the Studio App using the following endpoints:

- <http://api.localhost>
- <http://studio.localhost>

### Uninstall

```Bash
helm uninstall demo -n NAMESPACE
```

## How to use in production

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
