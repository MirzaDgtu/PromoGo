---
type: community
members: 21
---

# Loyalty Service Tests

**Members:** 21 nodes

## Members
- [[dot-Create()_2]] - code - internal/service/loyalty_test.go
- [[dot-GetByPhone()_1]] - code - internal/service/loyalty_test.go
- [[dot-set()]] - code - internal/service/loyalty_test.go
- [[T_1]] - code
- [[TestAccrue_ConcurrentClientCreateRaceRecovered()]] - code - internal/service/loyalty_test.go
- [[TestAccrue_ConflictingPhoneDoesNotCreateOrphanClient()]] - code - internal/service/loyalty_test.go
- [[TestAccrue_IdempotencyConflictOnMismatchedAmount()]] - code - internal/service/loyalty_test.go
- [[TestAccrue_ReplayReturnsOriginalResult()]] - code - internal/service/loyalty_test.go
- [[TestRedeem_CapAppliedBeforeBalanceCheck()]] - code - internal/service/loyalty_test.go
- [[TestRedeem_CapsAtMaxRedeemPercentAndRejectsInsufficientBalance()]] - code - internal/service/loyalty_test.go
- [[TestRedeem_CrossStoreClientRejected()]] - code - internal/service/loyalty_test.go
- [[TestRedeem_IdempotencyConflictOnMismatchedPoints()]] - code - internal/service/loyalty_test.go
- [[fakeBalanceRepo]] - code - internal/service/loyalty_test.go
- [[fakeClientRepo]] - code - internal/service/loyalty_test.go
- [[fakeLedgerRepo]] - code - internal/service/loyalty_test.go
- [[loyalty_test.go]] - code - internal/service/loyalty_test.go
- [[newFakeBalanceRepo()]] - code - internal/service/loyalty_test.go
- [[newFakeClientRepo()]] - code - internal/service/loyalty_test.go
- [[newTestService()]] - code - internal/service/loyalty_test.go
- [[pointsConfig()]] - code - internal/service/loyalty_test.go
- [[testDeps]] - code - internal/service/loyalty_test.go

## Live Query (requires Dataview plugin)

```dataview
TABLE source_file, type FROM #community/Loyalty_Service_Tests
SORT file.name ASC
```

## Connections to other communities
- 9 edges to [[_COMMUNITY_Balances and Loyalty Configuration]]
- 3 edges to [[_COMMUNITY_Client Persistence]]
- 3 edges to [[_COMMUNITY_Transaction Persistence]]
- 2 edges to [[_COMMUNITY_Loyalty Service Core]]

## Top bridge nodes
- [[testDeps]] - degree 7, connects to 3 communities
- [[loyalty_test.go]] - degree 18, connects to 2 communities
- [[newTestService()]] - degree 14, connects to 2 communities
- [[fakeClientRepo]] - degree 7, connects to 2 communities
- [[dot-Create()_2]] - degree 7, connects to 2 communities