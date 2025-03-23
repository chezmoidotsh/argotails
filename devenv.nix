{ pkgs, lib, config, inputs, ... }:

{
  # -- Environment variables
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
    pkgs.kubectl
    pkgs.kubernetes-helm
    pkgs.tilt

    # - Development tools
    pkgs.air
    pkgs.just
    pkgs.mise
    pkgs.runme

    # - Quality assurance tools
    pkgs.commitlint
    pkgs.trunk-io

    # - Miscellaneous tools
    pkgs.bash
    pkgs.tailscale
  ];

  # -- Customizations
  languages.go.enable = true;
  languages.go.package = pkgs.go;
  languages.nix.enable = true;
  languages.python.enable = true;
  languages.python.directory = "${config.env.DEVENV_ROOT}/.direnv/python";

  devcontainer.enable = true;
  devcontainer.settings.customizations.vscode.extensions = [
    "bierner.github-markdown-preview"
    "bierner.markdown-preview-github-styles"
    "golang.go"
    "mkhl.direnv"
    "tilt-dev.tiltfile"
    "trunk.io"
  ];
  difftastic.enable = true;

  scripts.motd.exec = ''
      cat <<EOF
    ────────────────────────────────────────────────────────────────────────────────
    👋 Welcome to the ??? development environment !
    This space contains everything required to contribute to the ??? project.

    📚 No documentation has been written yet ... but it is planned
    🚀 How to build or update the infrastructure ?
    - You can't.... nothing is ready yet
    ────────────────────────────────────────────────────────────────────────────────
    EOF
  '';

  enterShell = ''
    # Show MOTD only once, when the environment is built
    find "${config.env.DEVENV_ROOT}/.direnv/motd" -type f -mtime +0 -exec rm {} \; 2> /dev/null
    test -f "${config.env.DEVENV_ROOT}/.direnv/motd" || motd | tee "${config.env.DEVENV_ROOT}/.direnv/motd"
  '';
}
