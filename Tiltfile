# Tiltfile for Kubernetes Controller
# This Tiltfile automates the build, deployment, and live-reload of a Kubernetes controller.
# It builds the controller's image, updates kustomization.yaml with the built image,
# and applies the manifests using Kustomize.

print("""
────────────────────────────────────────────────────────────────────────────────
                   🎉 Welcome to the Argotails Tiltfile! 🎉

 This Tiltfile automates the build, deployment, and live-reload of a Kubernetes
                 controller inside a local Kubernetes cluster.

────────────────────────────────────────────────────────────────────────────────
""")

# To avoid any issues with the Tiltfile, we only allow the use of the k3d-argotails context managed by ctlptl
allow_k8s_contexts('')

# Setup the Tilt project with some credentials
config.define_string("ts.tailnet", usage="Tailscale Tailnet to use")
config.define_string("ts.authkey", usage="Tailscale OAuth Key to use")
cfg = config.parse()

if not cfg.get('ts.tailnet') or not cfg.get('ts.authkey'):
    fail("Please provide a Tailscale Tailnet and Auth Key using `tilt up -- --ts.tailnet=<TAILNET> --ts.authkey=<AUTHKEY>`")

# Build the controller image when the Tiltfile is loaded or when source code changes
CONTROLLER_IMAGE = 'ghcr.io/chezmoidotsh/argotails:dev'
docker_build(CONTROLLER_IMAGE, '.')

# Generate the default Kustomize manifests and edit the deployment to use the built image and Tailscale secret
argotails_secrets = blob("""
apiVersion: v1
kind: Secret
metadata:
  name: argotails-secrets
  namespace: argocd-dev
type: Opaque
stringData:
  authkey: {}
  tailnet: {}
""".format(cfg.get('ts.authkey'), cfg.get('ts.tailnet')))


k8s_yaml([
    argotails_secrets,
    kustomize('test/dev')
])
k8s_resource(new_name="kustomize-manifests", objects=[
    "argocd-dev:Namespace:default",
    "argotails-rolebinding:RoleBinding:argocd-dev",
    "argotails-secrets:Secret:argocd-dev",
    "argotails:Role:argocd-dev", 
    "argotails:ServiceAccount:argocd-dev", 
])
