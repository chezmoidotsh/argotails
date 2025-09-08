# ┌───────────────────────────────────────────────────────────────────────────┐
# │ <builder>: build the ArgoTails binary (Go)                                │
# └───────────────────────────────────────────────────────────────────────────┘
FROM docker.io/library/golang:1.25-alpine AS builder

# renovate: datasource=github-tags depName=chezmoidotsh/argotails versioning=semver
ARG ARGOTAILS_VERSION="v0.1.12"

RUN set -eux; \
    apk add --no-cache git;

COPY . /src

WORKDIR /src
RUN set -eux; \
    go build \
        -ldflags " \
            -X github.com/prometheus/common/version.Version=${ARGOTAILS_VERSION} \
            -X github.com/prometheus/common/version.Revision=$(git rev-parse --short HEAD) \
            -X github.com/prometheus/common/version.Branch=$(git rev-parse --abbrev-ref HEAD) \
            -X github.com/prometheus/common/version.BuildUser=$(whoami)@$(hostname) \
            -X github.com/prometheus/common/version.BuildDate=$(date -u +'%Y-%m-%dT%H:%M:%SZ') \
        " \
        -o /src/argotails ./cmd/argotails/


# ┌───────────────────────────────────────────────────────────────────────────┐
# │ <runtime>: create the ArgoTails runtime image using all previous stages   │
# └───────────────────────────────────────────────────────────────────────────┘
FROM gcr.io/distroless/static:nonroot@sha256:e8a4044e0b4ae4257efa45fc026c0bc30ad320d43bd4c1a7d5271bd241e386d0

# renovate: datasource=github-tags depName=chezmoidotsh/argotails versioning=semver
ARG ARGOTAILS_VERSION="v0.1.12"

COPY --from=builder /src/argotails /src/LICENSE /opt/argotails/

ENV PATH=/opt/argotails:${PATH}

USER nonroot
WORKDIR /opt/argotails
ENTRYPOINT ["argotails"]

# metadata as defined by the Open Container Initiative (OCI) and using the 
# chezmoi.sh conventions to keep traceability with the source code.
LABEL \
    org.opencontainers.image.authors="xunleii <xunleii@users.noreply.github.com>" \
    org.opencontainers.image.created="01/01/1970T00:00:00.000" \
    org.opencontainers.image.description="Kubernetes controller for ArgoCD cluster through Tailscale" \
    org.opencontainers.image.documentation="https://github.com/chezmoidotsh/argotails" \
    org.opencontainers.image.licenses="MIT" \
    org.opencontainers.image.revision="" \
    org.opencontainers.image.source="" \
    org.opencontainers.image.title="argotails" \
    org.opencontainers.image.url="https://github.com/chezmoidotsh/argotails" \
    org.opencontainers.image.version=${ARGOTAILS_VERSION}
