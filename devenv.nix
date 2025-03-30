{ pkgs, lib, config, inputs, ... }:

{
  # -- Environment variables
  # Fortify is disabled to avoid compilation issues with some Go dependencies
  env.hardeningDisable = [ "fortify" ];

  env.DFT_SKIP_UNCHANGED = "true";

  env.HELM_CACHE_HOME = "${config.env.DEVENV_ROOT}/.direnv/helm/cache";
  env.HELM_CONFIG_HOME = "${config.env.DEVENV_ROOT}/.direnv/helm/config";
  env.HELM_DATA_HOME = "${config.env.DEVENV_ROOT}/.direnv/helm/data";

  env.KUBECONFIG = "${config.env.DEVENV_ROOT}/.direnv/kubernetes/config.yaml";
  env.KUBECTL_EXTERNAL_DIFF = "${pkgs.difftastic}/bin/difft";

  # -- Required packages
  packages = [
    # - Kubernetes and container tools
    pkgs.pack
    pkgs.helm-docs
    pkgs.k3d
    pkgs.k9s
    pkgs.kubectl
    pkgs.kubernetes-helm

    # - Development tools
    pkgs.air
    pkgs.ctlptl
    pkgs.just
    pkgs.mise
    pkgs.runme
    pkgs.tilt

    # - Quality assurance tools
    pkgs.commitlint
    pkgs.trunk-io

    # - Miscellaneous tools
    pkgs.act
    pkgs.bash
    pkgs.tailscale
  ];

  # -- Customizations
  languages.go.enable = true;
  languages.nix.enable = true;

  devcontainer.enable = true;
  devcontainer.settings.customizations.vscode.extensions = [
    "bierner.github-markdown-preview"
    "bierner.markdown-preview-github-styles"
    "golang.go"
    "jnoortheen.nix-ide"
    "mkhl.direnv"
    "ms-kubernetes-tools.vscode-kubernetes-tools"
    "tilt-dev.tiltfile"
    "trunk.io"
  ];
  devcontainer.settings.features = {
    "ghcr.io/devcontainers/features/docker-in-docker:2.12.1" = { };
  };
  difftastic.enable = true;

  scripts.motd.exec = ''
      cat <<EOF
    ────────────────────────────────────────────────────────────────────────────────
    👋 Welcome to the Argotails development environment !
    This space contains everything required to contribute to the Argotails project.

    📚 No documentation has been written yet ... but it is planned
    🚀 How to start my development experience?
      1. Run \`dev:setup_kubernetes\` to deploy the local Kubernetes cluster
         NOTE: This have been probably done for you already when you entered the shell
      2. Run \`tilt up\` to start the Tilt development environment
      3. Start hacking on the code and see the changes live-reloaded in the Kubernetes cluster
    ────────────────────────────────────────────────────────────────────────────────
    EOF
  '';
  scripts."dev:setup_kubernetes".exec = ''
    # Start the local Kubernetes cluster
    echo "🚀 Starting the local Kubernetes cluster..."
    ctlptl apply -f test/dev/argotails-dev.ctlptl.yaml
  '';

  tasks."devenv:enterShell:setup_kubernetes" = {
    exec = ''
      # Start the local Kubernetes cluster
      ctlptl apply -f test/dev/argotails-dev.ctlptl.yaml
    '';
    before = [ "devenv:enterShell" ];
    status = "kubectl cluster-info > /dev/null 2>&1";
  };

  enterShell = ''
    # Show MOTD every time we enter the shell
    motd
  '';
}
