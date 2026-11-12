# syntax=docker/dockerfile:1
#
# Used by scripts/lint.{ps1,sh} to run the frontend lint + typecheck locally.
# See lint-backend.Dockerfile for why we use `docker build` instead of
# `docker run -v` here.
#
# Pin to the same Node version as infra/Dockerfile.frontend.

FROM node:22-alpine AS lint
WORKDIR /src

COPY package*.json ./
RUN npm install --silent --no-audit --no-fund

COPY . .
RUN npm run lint
RUN npm run typecheck
# Install and lint above run as root at build time. The finished image is
# not a service. `node` (uid 1000) is the non-root user shipped in the
# Node image, which is what the repository scan requires (Trivy DS-0002).
USER node
