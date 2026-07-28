---
type: community
members: 16
---

# Transaction Persistence

**Members:** 16 nodes

## Members
- [[dot-GetByExternalID()]] - code - internal/repository/postgres/transaction_repository.go
- [[dot-GetByExternalID()_1]] - code - internal/service/loyalty_test.go
- [[dot-ListByClient()]] - code - internal/repository/postgres/transaction_repository.go
- [[dot-ListByClient()_1]] - code - internal/service/loyalty_test.go
- [[Context_12]] - code
- [[Decimal_1]] - code
- [[NewTransactionRepository()]] - code - internal/repository/postgres/transaction_repository.go
- [[Pool_7]] - code
- [[Time_1]] - code
- [[Transaction]] - code - internal/domain/transaction.go
- [[TransactionRepository]] - code - internal/domain/transaction.go
- [[TransactionRepository_1]] - code - internal/repository/postgres/transaction_repository.go
- [[TransactionType]] - code - internal/domain/transaction.go
- [[fakeTxRepo]] - code - internal/service/loyalty_test.go
- [[transaction.go]] - code - internal/domain/transaction.go
- [[transaction_repository.go]] - code - internal/repository/postgres/transaction_repository.go

## Live Query (requires Dataview plugin)

```dataview
TABLE source_file, type FROM #community/Transaction_Persistence
SORT file.name ASC
```

## Connections to other communities
- 4 edges to [[_COMMUNITY_Balances and Loyalty Configuration]]
- 3 edges to [[_COMMUNITY_Loyalty Service Tests]]
- 2 edges to [[_COMMUNITY_Application Lifecycle and Persistence]]

## Top bridge nodes
- [[Transaction]] - degree 12, connects to 2 communities
- [[fakeTxRepo]] - degree 6, connects to 1 community
- [[NewTransactionRepository()]] - degree 4, connects to 1 community
- [[dot-GetByExternalID()_1]] - degree 4, connects to 1 community
- [[dot-ListByClient()_1]] - degree 3, connects to 1 community