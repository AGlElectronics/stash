#!/usr/bin/env sh
# Run the same lint pass CI runs, against your local working tree, before
# pushing. Lint runs in throwaway Docker images built from
# scripts/lint-*.Dockerfile so the host needs Docker only.
#
# We use `docker build` instead of `docker run -v` because Docker Desktop on
# Windows can't bind-mount network drives reliably (silent empty mount).
# The Linux/macOS path could use bind mounts, but we keep one code path so
# behavior is identical across platforms.
#
# Backend lint exports the regenerated sqlcgen back to the host via
# `docker buildx --output type=local` so the IDE stays in sync after editing
# queries/migrations.
#
# Usage:
#   ./scripts/lint.sh                # both
#   ./scripts/lint.sh backend
#   ./scripts/lint.sh frontend

set -e

target="${1:-all}"
script_dir="$(cd "$(dirname "$0")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"

backend() {
    echo "==> backend: sqlc generate + golangci-lint (docker build)"
    docker buildx build \
        --target sqlcgen-export \
        --output "type=local,dest=${repo_root}/backend/internal/repository/sqlcgen" \
        -f "${script_dir}/lint-backend.Dockerfile" \
        "${repo_root}/backend"
}

frontend() {
    echo "==> frontend: eslint + tsc (docker build)"
    docker build \
        -f "${script_dir}/lint-frontend.Dockerfile" \
        "${repo_root}/frontend"
}

case "${target}" in
    backend) backend ;;
    frontend) frontend ;;
    all) backend; frontend ;;
    *)
        echo "usage: $0 [backend|frontend|all]" >&2
        exit 1
        ;;
esac

echo "==> lint OK"
