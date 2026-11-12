# syntax=docker/dockerfile:1
#
# Used by scripts/lint.{ps1,sh} to run the backend lint locally.
#
# We use `docker build` instead of `docker run -v` because Docker Desktop on
# Windows can't bind-mount network drives (e.g. UNC-mapped `Y:`) reliably -
# the mount silently produces an empty directory inside the container.
# `docker build` streams the context to the daemon and works regardless.
#
# Three stages:
#   tooling          - cacheable layer with sqlc + golangci-lint preinstalled
#   lint             - runs sqlc generate + golangci-lint; build fails on lint error
#   sqlcgen-export   - scratch image holding ONLY the regenerated sqlcgen dir,
#                      used with `docker buildx build --output type=local,...`
#                      to copy the generated Go back to the host (also fixes
#                      IDE intellisense after editing queries/migrations).
#
# Pin to the same versions as infra/Dockerfile.backend and the shared CI
# (module-ci.yml in con-services). Update them together.

FROM golangci/golangci-lint:v2.11-alpine AS tooling
ARG SQLC_VERSION=1.30.0
RUN apk add --no-cache wget tar ca-certificates
RUN wget -qO- "https://github.com/sqlc-dev/sqlc/releases/download/v${SQLC_VERSION}/sqlc_${SQLC_VERSION}_linux_amd64.tar.gz" \
    | tar -xz -C /usr/local/bin sqlc \
    && chmod +x /usr/local/bin/sqlc
# The lint stage starts from this image and would inherit this user, so
# that stage switches back to root for sqlc and golangci-lint, then drops
# privileges again. A non-root USER is what the repository scan requires
# (Trivy DS-0002). 65532 is a numeric id because this stage is not a
# service and does not need a named account.
USER 65532:65532

FROM tooling AS lint
USER root
WORKDIR /src
COPY . .
RUN sqlc generate
RUN golangci-lint run ./...
USER 65532:65532

FROM scratch AS sqlcgen-export
COPY --from=lint /src/internal/repository/sqlcgen /
# Scratch has no /etc/passwd. The numeric user is metadata for the scan;
# buildx exports the files only, and the lint already ran in the stage above.
USER 65532:65532
