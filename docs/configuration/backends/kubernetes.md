# Configuring the Kubernetes backend

The Kubernetes backend uses the API server to store domain and zone information using a Custom Resource Definition. To use the Kubernetes backend, DNS4ACME must be compiled with the `kubernetes` build tag (enabled for our binary packages).

!!! note
    The Kubernetes backend is not related to [installing on Kubernetes](../../installation/kubernetes.md). You can use the Kubernetes backend without running DNS4ACME on Kubernetes and vice versa.

---

## Deploying the CRD

DNS4ACME doesn't automatically deploy its [CustomResourceDefinition](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/), you will have to do this manually. Please deploy the following CRD:

<details><summary>crd.yml</summary>
```yaml
{% include 'configuration/backends/crd.yml' %}
```
</details>

## Creating credentials for DNS4ACME

The next step is to create a role, role binding and service account for DNS4ACME. If you are deploying in-cluster, your procedure may be different depending on your cluster configuration.

First, the role:

<details><summary>role.yml</summary>
```yaml
{% include 'configuration/backends/role.yml' %}
```
</details>

Then the service account:

<details><summary>service_account.yml</summary>
```yaml
{% include 'configuration/backends/service_account.yml' %}
```
</details>

Finally, the role binding:

<details><summary>role_binding.yml</summary>
```yaml
{% include 'configuration/backends/role_binding.yml' %}
```
</details>

If you are running DNS4ACME outside the cluster, you can now create a token using the following command:

```
kubectl create token dns4acme
```

## Setting up domains

The Kubernetes backend requires you to create an entry for each domain you want DNS4ACME to server. For example:

<details><summary>domain.yml</summary>
```yaml
{% include 'configuration/backends/domain.yml' %}
```
</details>

---

## Configuration options

The Kubernetes backend has several configuration options.

| CLI option                       | Environment variable                    | Default                  | Description                                                                                                                                                                            |
|----------------------------------|-----------------------------------------|--------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `--backend`                      | `DNS4ACME_BACKEND`                      | -                        | Set this option to `kubernetes` to use the Kubernetes backend.                                                                                                                         |
| `--kubernetes-bearer-token`      | `DNS4ACME_KUBERNETES_BEARER_TOKEN`      | -                        | Token used to authenticate to the Kubernetes API.                                                                                                                                      |
| `--kubernetes-bearer-token-file` | `DNS4ACME_KUBERNETES_BEARER_TOKEN_FILE` | -                        | File containing the bearer token used to authenticate to the Kubernetes API. Set to `/var/run/secrets/kubernetes.io/serviceaccount/token` for in-cluster authentication.               |
| `--kubernetes-host`              | `DNS4ACME_KUBERNETES_HOST`              | `kubernetes.default.svc` | Host name for the Kubernetes cluster API server.                                                                                                                                       |
| `--kubernetes-server-name`       | `DNS4ACME_KUBERNETES_SERVER_NAME`       | -                        | SNI name to pass to the Kubernetes API server.                                                                                                                                         |
| `--kubernetes-path`              | `DNS4ACME_KUBERNETES_PATH`              | `/api`                   | Path for the API endpoint                                                                                                                                                              |
| `--kubernetes-cacert`            | `DNS4ACME_KUBERNETES_CACERT`            | -                        | PEM-encoded Certificate Authority to verify the connection to the Kubernetes API.                                                                                                      |
| `--kubernetes-cacert-file`       | `DNS4ACME_KUBERNETES_CACERT_FILE`       | -                        | File containing the PEM-encoded CA certificate to verify the connection to the Kubernetes API. Set to `/var/run/secrets/kubernetes.io/serviceaccount/ca.crt` for in-cluster operation. |
| `--kubernetes-cert`              | `DNS4ACME_KUBERNETES_CERT`              | -                        | PEM-encoded client certificate to use for authenticating to the Kubernetes API.                                                                                                        |
| `--kubernetes-cert-file`         | `DNS4ACME_KUBERNETES_CERT_FILE`         | -                        | File containing the PEM-encoded client certificate to use for authenticating to the Kubernetes API.                                                                                    |
| `--kubernetes-key`               | `DNS4ACME_KUBERNETES_KEY`               | -                        | PEM-encoded client private key to use for authenticating to the Kubernetes API.                                                                                                        | 
| `--kubernetes-key-file`          | `DNS4ACME_KUBERNETES_KEY_FILE`          | -                        | File containing the PEM-encoded client private key to use for authenticating to the Kubernetes API.                                                                                    |
| `--kubernetes-username`          | `DNS4ACME_KUBERNETES_USERNAME`          | -                        | Username for authenticating to the Kubernetes API.                                                                                                                                     |
| `--kubernetes-password`          | `DNS4ACME_KUBERNETES_PASSWORD`          | -                        | Password for authenticating to the Kubernetes API.                                                                                                                                     |
| `--kubernetes-qps`               | `DNS4ACME_KUBERNETES_QPS`               | `5`                      | Maximum QPS to use for Kubernetes API requests.                                                                                                                                        |
| `--kubernetes-burst`             | `DNS4ACME_KUBERNETES_BURST`             | `10`                     | Maximum burst to use for Kubernetes API requests.                                                                                                                                      |
| `--kubernetes-timeout`           | `DNS4ACME_KUBERNETES_TIMEOUT`           | `5s`                     | Maximum time to wait for a response from the Kubernetes API. Supports adding time qualifiers.                                                                                          |
