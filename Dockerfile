# Image tags are pinned to a specific patch version and digest so a build is
# reproducible and an upstream tag move can't silently change what ships.
#
# To bump these images: pick the new tag, `docker pull <tag>`, then
# `docker inspect --format='{{index .RepoDigests 0}}' <tag>` for the digest,
# and update both the tag and the digest below together.
FROM golang:1.25.14-alpine@sha256:1ae0735f00daffa3aaf1363a5184c0d2dc55c78e3db4ec70241cdac97bf84b59 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -o /out/promogo ./cmd/promogo

FROM alpine:3.22.2@sha256:4b7ce07002c69e8f3d704a9c5d6fd3053be500b7f1c69fc0d80990c2ad8dd412
RUN adduser -D -g '' promogo
WORKDIR /app

COPY --from=build /out/promogo /usr/local/bin/promogo
COPY configs ./configs
COPY migrations ./migrations

USER promogo
EXPOSE 8080
ENTRYPOINT ["promogo"]
