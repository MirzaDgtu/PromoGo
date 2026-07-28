FROM golang:1.25-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -o /out/promogo ./cmd/promogo

FROM alpine:3.20
RUN adduser -D -g '' promogo
WORKDIR /app

COPY --from=build /out/promogo /usr/local/bin/promogo
COPY configs ./configs
COPY migrations ./migrations

USER promogo
EXPOSE 8080
ENTRYPOINT ["promogo"]
