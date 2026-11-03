# syntax=docker/dockerfile:1
#
# Used by scripts/lint.{ps1,sh} to run the frontend lint + typecheck locally.
# See lint-backend.Dockerfile for why we use `docker build` instead of
# `docker run -v` here.
#
# Pin to the same Node version as .gitlab-ci.yml / Dockerfile.frontend.

FROM node:22-alpine AS lint
WORKDIR /src

COPY package*.json ./
RUN npm install --silent --no-audit --no-fund

COPY . .
RUN npm run lint
RUN npm run typecheck
