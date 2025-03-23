<!-- markdownlint-disable MD033 -->
<h1 align="center">
  Argotails
</h1>

<h4 align="center">Synchronize your remote ArgoCD clusters with Tailscale devices in real time</h4>

<div align="center">

[![Go Version](https://img.shields.io/github/go-mod/go-version/chezmoi-sh/argotails)](https://golang.org)
[![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/chezmoi-sh/argotails/release.publish-argotails.yaml?logo=github&label=Last%20release)](https://github.com/chezmoi-sh/argotails/actions/workflows/release.publish-argotails.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/chezmoi-sh/argotails)](https://goreportcard.com/report/github.com/chezmoi-sh/argotails)
[![License](https://img.shields.io/badge/license-MIT-blue?logo=git&logoColor=white)](LICENSE)

</div>

---

## 📚 Table of Contents

- [About](#ℹ%EF%B8%8F-about)
  - [Features](#-features)
- [How to use Argotails](#-how-to-use-argotails)
  - [Prerequisites](#-prerequisites)
  - [Setup Guide](#%EF%B8%8F-setup-guide)
    - [Creating Tailscale OAuth Credentials](#1-creating-tailscale-oauth-credentials)
    - [Setting Up the Webhook (optional, but recommended)](#2-setting-up-the-webhook-optional-but-recommended)
  - [Deploying with Kustomize](#-deploying-with-kustomize)
    - [Creating the Tailscale Secret](#1-creating-the-tailscale-secret)
    - [Basic Deployment Example](#2-basic-deployment-example)
    - [Deployment with Webhook Support](#3-deployment-with-webhook-support)
- [Configuration Reference](#-configuration-reference)
- [Troubleshooting & FAQ](#-troubleshooting--faq)
- [Contribution Guidelines](#-contribution-guidelines)
- [License](#-license)

---

## ℹ️ About

**Argotails** is a Kubernetes controller that synchronizes Tailscale devices with ArgoCD clusters. It ensures your cluster secrets remain up-to-date as your Tailscale environment changes. This tool is ideal for teams that want to automate secret updates without manual intervention.

> \[!IMPORTANT]
> **Argotails** is designed to work alongside the [Tailscale Kubernetes API proxy](https://tailscale.com/kb/1437/kubernetes-operator-api-server-proxy). Without this integration, ArgoCD will not be able to access the remote Kubernetes API, making all secrets managed by Argotails useless.

### ✨ Features

- **Real-time Synchronization:** Automatically update ArgoCD secrets when Tailscale devices are added or removed.
- **Customizable Filters:** Use regular expressions to determine which Tailscale devices should be synchronized.
- **Flexible Deployment:** Deploy Argotails on any Kubernetes cluster with ArgoCD installed.
- **Webhook Integration:** Optional webhook support for instant updates using Tailscale Funnel or your preferred method.

---

## 🚀 How to use Argotails

### 🔑 Prerequisites

Before deploying Argotails, ensure you have:

- A running Kubernetes cluster with ArgoCD installed
- A Tailscale account with admin privileges
- A Tailscale network configured to use the [Tailscale Kubernetes API proxy](https://tailscale.com/kb/1437/kubernetes-operator-api-server-proxy)
- [kubectl](https://kubernetes.io/docs/tasks/tools/) configured to access your cluster
- [kustomize](https://kustomize.io/) (or use `kubectl` built-in kustomize support)

For further guidance on setting up Kubernetes and ArgoCD, refer to the official [Kubernetes documentation](https://kubernetes.io/docs/home/) and [ArgoCD docs](https://argo-cd.readthedocs.io/en/stable/).

### ⚙️ Setup Guide

#### 1. Creating Tailscale OAuth Credentials

1. **Log in to Tailscale:**  
   Go to the [Tailscale Admin Console](https://login.tailscale.com/admin/).

2. **Generate OAuth Credentials:**

   - Navigate to **Settings** > **Keys**.
   - Click **Generate OAuth client credentials**.
   - Provide a descriptive name (e.g., "Argotails Integration").
   - Enable **Read** access on the **Devices / Core** scope.
   - Save your client ID and client secret securely.

   _Tip:_ Refer to Tailscale’s documentation for a detailed explanation of OAuth tokens and scopes.

#### 2. Setting Up the Webhook (optional, but recommended)

If you want real-time updates via webhooks:

1. **Configure the Webhook:**
   - In the Tailscale Admin Console, navigate to **Settings** > **Webhooks**.
   - Click **Add endpoint**.
   - Enter a name such as "Argotails Device Updates".
   - Set the endpoint URL to your Argotails webhook endpoint (e.g., `https://argotails.your-domain.com/webhook`).
   - Select only the `nodeCreated` and `nodeDeleted` events.
   - Copy and securely store the generated webhook secret for later use.

### 📦 Deploying with Kustomize

> \[!NOTE]
> No Helm chart is available for Argotails because they require too much maintenance for a simple project like this. Kustomize is the recommended way to deploy Argotails.

Argotails is deployable via Kustomize. Follow these steps based on your chosen deployment mode.

#### 1. Creating the Tailscale Secret

> \[!TIP]
> All example commands assume you have installed ArgoCD in the `argocd` namespace. Adjust the namespace if you have a different setup.

Create a Kubernetes secret with your Tailscale OAuth credentials (and webhook secret if using the webhook):

```bash
# Replace 'tskey-auth-xxxxxxxxxxxx' with your actual Tailscale auth key.
kubectl create secret generic tailscale-secrets \
  --namespace argocd \
  --from-literal=authkey='tskey-auth-xxxxxxxxxxxx' \
  --dry-run=client -o yaml | kubectl apply -f -
```

If using the webhook feature, patch the secret to add the webhook secret:

```bash
kubectl patch secret tailscale-secrets \
  --namespace argocd \
  --type merge \
  --patch '{"data":{"webhook_secret":"tskey-webhook--xxxxxxxxxxxx"}}'
```

#### 2. Basic Deployment Example

Create a `kustomization.yaml` file to deploy Argotails using the default manifests:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: argocd

resources:
  - https://github.com/chezmoi-sh/argotails//deploy/manifests/default?ref=main

patches:
  - patch: |
      - op: replace
        path: /spec/template/spec/containers/0/env/0/value
        value: my-tailnet.ts.net
    target:
      name: argotails
      kind: Deployment
```

> \[!TIP]
>
> - Replace `my-tailnet.ts.net` with your actual Tailscale tailnet.
> - Adjust patch values for other environments as needed.

#### 3. Deployment with Webhook Support

For deployments that require webhook support via [Tailscale Funnel](https://tailscale.com/kb/1439/kubernetes-operator-cluster-ingress#exposing-a-service-to-the-public-internet-using-ingress-and-tailscale-funnel), use the following `kustomization.yaml`:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: argocd

resources:
  - https://github.com/chezmoi-sh/argotails//deploy/manifests/with-webhook-funnel?ref=main

patches:
  - patch: |
      - op: replace
        path: /spec/template/spec/containers/0/env/0/value
        value: my-tailnet.ts.net
    target:
      name: argotails
      kind: Deployment
```

It will create a new Tailscale device with the name `argotails` available **PUBLICLY** on the internet; `https://argotails.your-tailnet.ts.net` will be the URL to access the webhook.

> \[!CAUTION]
> **Security Warning:** Exposing the webhook publicly can lead to security vulnerabilities. However, it is the only way to receive real-time updates from Tailscale.
> Everything has been done to secure the webhook and the only thing it does is to update the Kubernetes secret with the new Tailscale device... but be careful and don't expose it if some delay is acceptable for you.

---

## 📝 Configuration Reference

Argotails is highly configurable via flags and environment variables. Here’s an overview:

```
Usage: argotails run --ts.tailnet=TAILSCALE_TAILNET --ts.authkey=TAILSCALE_AUTH_KEY [flags]

Run the ArgoCD Tailscale integration controller.

Flags:
  -h, --help                      Show context-sensitive help.

      --reconcile.interval=30s    Time between two Tailscale devices and ArgoCD cluster secrets reconciliation ($RECONCILE_INTERVAL).

Tailscale flags
  --ts.base-url=https://api.tailscale.com                   Tailscale API base URL ($TAILSCALE_BASE_URL).
  --ts.tailnet=TAILSCALE_TAILNET                            Tailscale network name ($TAILSCALE_TAILNET).
  --ts.authkey=TAILSCALE_AUTH_KEY                           Tailscale OAuth key ($TAILSCALE_AUTH_KEY).
  --ts.authkey-file=TAILSCALE_AUTH_KEY_FILE                 Path to the file containing the Tailscale OAuth key ($TAILSCALE_AUTH_KEY_FILE).
  --ts.device-filter=PATTERN,...                            List of regular expressions to filter the Tailscale devices based on their tags.
  --ts.webhook.enable                                       Enable the Tailscale webhook handler ($TAILSCALE_WEBHOOK_ENABLE).
  --ts.webhook.port=3000                                    Tailscale webhook port ($TAILSCALE_WEBHOOK_PORT).
  --ts.webhook.secret=TAILSCALE_WEBHOOK_SECRET              Tailscale webhook secret ($TAILSCALE_WEBHOOK_SECRET).
  --ts.webhook.secret-file=TAILSCALE_WEBHOOK_SECRET_FILE    Path to the file containing the Tailscale webhook secret ($TAILSCALE_WEBHOOK_SECRET_FILE).

ArgoCD flags
  --argocd.namespace=STRING    Namespace where ArgoCD is installed (if the controller is runned outside a cluster) ($ARGOCD_NAMESPACE).

Log flags
  --log.devel          Enable development logging ($LOG_DEVEL).
  --log.v=2            Log verbosity level ($LOG_VERBOSITY).
  --log.format=json    Log encoding format, either 'json' or 'console' ($LOG_FORMAT).
```

> \[!TIP]
> You can also load credentials from files using the `--ts.authkey-file` and `--ts.webhook.secret-file` flags.

---

## 🔧 Troubleshooting & FAQ

> \[!TIP]
> You can use the `--log.v` flag to increase verbosity for debugging.

### Common Issues and Solutions

- **Pods Not Starting:**  
  Verify that your Kubernetes secret is correctly created and that your environment variables match your OAuth credentials.

- **Synchronization Failures:**  
  Check the logs:

  ```bash
  kubectl logs -n argocd -l app.kubernetes.io/name=argotails
  ```

  Confirm that the webhook (if enabled) is correctly receiving events.

- **Webhook Validation Errors:**  
  Ensure that the webhook secret in Tailscale and in your Kubernetes secret are identical.

---

## 🤝 Contribution Guidelines

Contributions are welcome! Please review our [CONTRIBUTING.md](CONTRIBUTING.md) for detailed guidelines on how to contribute, including coding standards, pull request instructions, and testing requirements.

---

## 📜 License

Argotails is distributed under the MIT license. See the [LICENSE](LICENSE) file for further details.

<!-- markdownlint-enable MD033 -->
