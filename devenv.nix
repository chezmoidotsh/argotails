{ pkgs, lib, config, inputs, ... }:

{
  # ┌───────────────────────────────────────────────────────────────────────────┐
  # │ Environment variables definitions                                         │
  # └───────────────────────────────────────────────────────────────────────────┘

  # Fortify is disabled to avoid compilation issues with some Go dependencies
  env.hardeningDisable = [ "fortify" ];

  # NOTE: in order to speed up a bit the development experience, we will use the
  #       `.containerdev` folder as location for all cached items
  env.HELM_CACHE_HOME = "${config.env.DEVENV_ROOT}/.devcontainer/cache/helm/cache";
  env.HELM_CONFIG_HOME = "${config.env.DEVENV_ROOT}/.devcontainer/cache/helm/config";
  env.HELM_DATA_HOME = "${config.env.DEVENV_ROOT}/.devcontainer/cache/helm/data";

  env.KUBECONFIG = "${config.env.DEVENV_ROOT}/.devcontainer/cache/kubernetes/config.yaml";

  # ┌───────────────────────────────────────────────────────────────────────────┐
  # │ Packages & languages configuration                                        │
  # └───────────────────────────────────────────────────────────────────────────┘
  languages.go.enable = true;
  languages.nix.enable = true;

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

  env.DFT_SKIP_UNCHANGED = "true";
  env.KUBECTL_EXTERNAL_DIFF = "${pkgs.difftastic}/bin/difft";
  difftastic.enable = true;

  # ┌───────────────────────────────────────────────────────────────────────────┐
  # │ Devcontainer configuration                                                │
  # └───────────────────────────────────────────────────────────────────────────┘
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
  devcontainer.settings.mounts = [
    # NOTE: in order to avoid conflict with old .devenv files existing on the
    #       host, we will mount this folder into a dedicated `tmpfs` volume
    {
      type = "tmpfs";
      target = "\${containerWorkspaceFolder}/.devenv";
    }
  ];
  devcontainer.settings.updateContentCommand = "
    set -x;
    sudo chown --recursive --no-dereference --silent vscode: /nix \"\${containerWorkspaceFolder}/.devenv\";
    devenv test;
    ${pkgs.direnv}/bin/direnv allow \"\${containerWorkspaceFolder}/.direnv\";
  ";

  # ┌───────────────────────────────────────────────────────────────────────────┐
  # │ Scripts definitions                                                       │
  # └───────────────────────────────────────────────────────────────────────────┘

  scripts."dev:setup_kubernetes".exec = ''
    # Start the local Kubernetes cluster
    echo "🚀 Starting the local Kubernetes cluster..."
    ctlptl apply -f test/dev/argotails-dev.ctlptl.yaml
  '';

  tasks."dev:motd" = {
    description = "Display the welcome message";
    exec = ''
      cat <<EOF
      ────────────────────────────────────────────────────────────────────────────────
      👋 Welcome to the Argotails development environment !
      This space contains everything required to contribute to the Argotails project.

      📚 No documentation has been written yet ... but it is planned
      🚀 How to start my development experience?
        1. Run \`dev:setup_kubernetes\` to deploy the local Kubernetes cluster
        2. Run \`tilt up\` to start the Tilt development environment
        3. Start hacking on the code and see the changes live-reloaded in the Kubernetes cluster
      ────────────────────────────────────────────────────────────────────────────────
      EOF
    '';
  };
}
