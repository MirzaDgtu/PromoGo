---
type: community
members: 27
---

# Loyalty Service Core

**Members:** 27 nodes

## Members
- [[dot-Accrue()_1]] - code - internal/service/loyalty.go
- [[dot-Redeem()]] - code - internal/service/loyalty.go
- [[dot-replayedAccrue()]] - code - internal/service/loyalty.go
- [[dot-replayedRedeem()]] - code - internal/service/loyalty.go
- [[dot-resolveOrCreateClient()]] - code - internal/service/loyalty.go
- [[AccrueRequest]] - code - internal/service/loyalty.go
- [[AccrueResult]] - code - internal/service/loyalty.go
- [[BalanceRepository_4]] - code
- [[Build()]] - code - internal/mechanicbuild/mechanicbuild.go
- [[ClientRepository_4]] - code
- [[Context_13]] - code
- [[Decimal_3]] - code
- [[LedgerRepository_2]] - code
- [[Logger_7]] - code
- [[LoyaltyConfigRepository_2]] - code
- [[LoyaltyService]] - code - internal/service/loyalty.go
- [[Mechanic_2]] - code
- [[New()_5]] - code - internal/service/loyalty.go
- [[NotificationChannel]] - code - internal/domain/notification.go
- [[RedeemRequest]] - code - internal/service/loyalty.go
- [[RedeemResult]] - code - internal/service/loyalty.go
- [[TransactionRepository_2]] - code
- [[accrualFingerprint()]] - code - internal/service/loyalty.go
- [[loyalty.go]] - code - internal/service/loyalty.go
- [[mechanicbuild.go]] - code - internal/mechanicbuild/mechanicbuild.go
- [[notification.go]] - code - internal/domain/notification.go
- [[redeemFingerprint()]] - code - internal/service/loyalty.go

## Live Query (requires Dataview plugin)

```dataview
TABLE source_file, type FROM #community/Loyalty_Service_Core
SORT file.name ASC
```

## Connections to other communities
- 3 edges to [[_COMMUNITY_HTTP API Layer]]
- 2 edges to [[_COMMUNITY_Loyalty Service Tests]]
- 1 edge to [[_COMMUNITY_Client Persistence]]

## Top bridge nodes
- [[LoyaltyService]] - degree 18, connects to 2 communities
- [[New()_5]] - degree 10, connects to 1 community
- [[dot-resolveOrCreateClient()]] - degree 4, connects to 1 community