---
type: "query"
date: "2026-09-09T07:41:54.364822+00:00"
question: "Почему такая ошибка при make redeploy? Error response from daemon: failed to set up container networking: Bind for 0.0.0.0:8080 failed: port is already allocated"
contributor: "graphify"
outcome: "useful"
source_nodes: ["Docker Compose Local Stack", "PromoGo Application Service", "HTTPConfig"]
---

# Q: Почему такая ошибка при make redeploy? Error response from daemon: failed to set up container networking: Bind for 0.0.0.0:8080 failed: port is already allocated

## Answer

Expanded via graph vocabulary: [docker, compose, app, service, local, stack, config, http]. Makefile redeploy builds app and runs docker compose up -d --no-deps app. PromoGo compose publishes host 8080 to container 8080. Read-only Docker inspection showed coingobot-app-1 from C:\Projects\Golang\github.com\MirzaDgtu\CoinGoBot\deployments\docker-compose.yml already owns 0.0.0.0:8080, while promogo-app-1 is left in Created state. Stop the conflicting CoinGoBot app if it is not needed, or change one project host mapping (for example PromoGo 8081:8080), then rerun make redeploy.

## Outcome

- Signal: useful

## Source Nodes

- Docker Compose Local Stack
- PromoGo Application Service
- HTTPConfig