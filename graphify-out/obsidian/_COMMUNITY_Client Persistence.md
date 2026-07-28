---
type: community
members: 12
---

# Client Persistence

**Members:** 12 nodes

## Members
- [[dot-Create()]] - code - internal/repository/postgres/client_repository.go
- [[dot-GetByID()]] - code - internal/repository/postgres/client_repository.go
- [[dot-GetByPhone()]] - code - internal/repository/postgres/client_repository.go
- [[Client]] - code - internal/domain/client.go
- [[ClientRepository]] - code - internal/domain/client.go
- [[ClientRepository_3]] - code - internal/repository/postgres/client_repository.go
- [[Context_7]] - code
- [[NewClientRepository()]] - code - internal/repository/postgres/client_repository.go
- [[Pool_2]] - code
- [[Time]] - code
- [[client.go]] - code - internal/domain/client.go
- [[client_repository.go]] - code - internal/repository/postgres/client_repository.go

## Live Query (requires Dataview plugin)

```dataview
TABLE source_file, type FROM #community/Client_Persistence
SORT file.name ASC
```

## Connections to other communities
- 3 edges to [[_COMMUNITY_Loyalty Service Tests]]
- 2 edges to [[_COMMUNITY_Application Lifecycle and Persistence]]
- 1 edge to [[_COMMUNITY_Balances and Loyalty Configuration]]
- 1 edge to [[_COMMUNITY_Loyalty Service Core]]

## Top bridge nodes
- [[Client]] - degree 11, connects to 4 communities
- [[NewClientRepository()]] - degree 4, connects to 1 community