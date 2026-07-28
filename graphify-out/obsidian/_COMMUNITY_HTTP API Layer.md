---
type: community
members: 40
---

# HTTP API Layer

**Members:** 40 nodes

## Members
- [[BalanceRepository_1]] - code
- [[BalanceRepository_2]] - code
- [[ClientRepository_1]] - code
- [[ClientRepository_2]] - code
- [[Context_1]] - code
- [[Context_2]] - code
- [[Decimal_2]] - code
- [[Deps]] - code - internal/httpserver/server.go
- [[HandlerFunc]] - code
- [[HandlerFunc_1]] - code
- [[HandlerFunc_2]] - code
- [[Logger_1]] - code
- [[Logger_3]] - code
- [[Logger_4]] - code
- [[New()_1]] - code - internal/httpserver/server.go
- [[RequireStoreAPIKey()]] - code - internal/httpserver/middleware.go
- [[ResponseWriter_1]] - code
- [[Server_1]] - code
- [[StoreRepository_1]] - code
- [[StoreRepository_2]] - code
- [[accrueRequestBody]] - code - internal/httpserver/transactions.go
- [[balanceResponseBody]] - code - internal/httpserver/clients.go
- [[clients.go]] - code - internal/httpserver/clients.go
- [[context.go]] - code - internal/httpserver/context.go
- [[handleAccrueTransaction()]] - code - internal/httpserver/transactions.go
- [[handleGetClientBalance()]] - code - internal/httpserver/clients.go
- [[handleLookupClientByPhone()]] - code - internal/httpserver/clients.go
- [[handleRedeemTransaction()]] - code - internal/httpserver/transactions.go
- [[hashAPIKey()]] - code - internal/httpserver/middleware.go
- [[middleware.go]] - code - internal/httpserver/middleware.go
- [[redeemRequestBody]] - code - internal/httpserver/transactions.go
- [[redeemResponseBody]] - code - internal/httpserver/transactions.go
- [[respond.go]] - code - internal/httpserver/respond.go
- [[server.go]] - code - internal/httpserver/server.go
- [[storeContextKey]] - code - internal/httpserver/context.go
- [[storeFromContext()]] - code - internal/httpserver/context.go
- [[transactionResponseBody]] - code - internal/httpserver/transactions.go
- [[transactions.go]] - code - internal/httpserver/transactions.go
- [[writeError()]] - code - internal/httpserver/respond.go
- [[writeJSON()]] - code - internal/httpserver/respond.go

## Live Query (requires Dataview plugin)

```dataview
TABLE source_file, type FROM #community/HTTP_API_Layer
SORT file.name ASC
```

## Connections to other communities
- 3 edges to [[_COMMUNITY_Loyalty Service Core]]
- 2 edges to [[_COMMUNITY_Runtime Configuration and Startup]]
- 1 edge to [[_COMMUNITY_Store Authentication and Persistence]]
- 1 edge to [[_COMMUNITY_HTTP Logging Middleware]]

## Top bridge nodes
- [[Deps]] - degree 10, connects to 2 communities
- [[New()_1]] - degree 11, connects to 1 community
- [[handleAccrueTransaction()]] - degree 8, connects to 1 community
- [[handleRedeemTransaction()]] - degree 8, connects to 1 community
- [[storeFromContext()]] - degree 7, connects to 1 community