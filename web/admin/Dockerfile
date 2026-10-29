# Admin Web image: the built SPA served by nginx, which also proxies /api/
# to the Go backend (same-origin, docs/admin-web-plan.md 5.2). See
# docs/admin-web-deployment.md.
#
# Image tags are pinned to a patch version and digest, as in the repo-root
# Dockerfile. To bump: pick the tag, `docker pull <tag>`, then
# `docker inspect --format='{{index .RepoDigests 0}}' <tag>`, and update tag
# and digest together.
FROM node:24.11.1-alpine3.22@sha256:2867d550cf9d8bb50059a0fff528741f11a84d985c732e60e19e8e75c7239c43 AS build
WORKDIR /src

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
# Vite bakes these into the bundle at build time, so an image is built per
# environment (the OIDC issuer differs between stage and production). They
# are public values, not secrets: the SPA is a public OIDC client with PKCE.
ARG VITE_OIDC_AUTHORITY
ARG VITE_OIDC_CLIENT_ID
ARG VITE_OIDC_SCOPE=
RUN test -n "$VITE_OIDC_AUTHORITY" && test -n "$VITE_OIDC_CLIENT_ID" \
    || { echo "VITE_OIDC_AUTHORITY and VITE_OIDC_CLIENT_ID build args are required" >&2; exit 1; }
RUN npm run build

FROM nginxinc/nginx-unprivileged:1.28.0-alpine3.21@sha256:c97ff0bf7cbae369953c6da1232ec14ad9f971d66360c5698db0856a4cd657a0
# The official entrypoint renders templates/*.template with envsubst into
# conf.d at start; PROMOGO_API_UPSTREAM picks the backend.
ENV PROMOGO_API_UPSTREAM=http://app:8080
COPY nginx/default.conf.template /etc/nginx/templates/default.conf.template
COPY nginx/security-headers.conf /etc/nginx/snippets/security-headers.conf
COPY --from=build /src/dist /usr/share/nginx/html
EXPOSE 8080
