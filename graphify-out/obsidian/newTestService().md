---
source_file: "internal/service/loyalty_test.go"
type: "code"
community: "Loyalty Service Tests"
location: "L157"
tags:
  - graphify/code
  - graphify/EXTRACTED
  - community/Loyalty_Service_Tests
---

# newTestService()

## Connections
- [[LoyaltyConfig]] - `references` [EXTRACTED]
- [[New()_5]] - `calls` [INFERRED]
- [[TestAccrue_ConcurrentClientCreateRaceRecovered()]] - `calls` [EXTRACTED]
- [[TestAccrue_ConflictingPhoneDoesNotCreateOrphanClient()]] - `calls` [EXTRACTED]
- [[TestAccrue_IdempotencyConflictOnMismatchedAmount()]] - `calls` [EXTRACTED]
- [[TestAccrue_ReplayReturnsOriginalResult()]] - `calls` [EXTRACTED]
- [[TestRedeem_CapAppliedBeforeBalanceCheck()]] - `calls` [EXTRACTED]
- [[TestRedeem_CapsAtMaxRedeemPercentAndRejectsInsufficientBalance()]] - `calls` [EXTRACTED]
- [[TestRedeem_CrossStoreClientRejected()]] - `calls` [EXTRACTED]
- [[TestRedeem_IdempotencyConflictOnMismatchedPoints()]] - `calls` [EXTRACTED]
- [[loyalty_test.go]] - `contains` [EXTRACTED]
- [[newFakeBalanceRepo()]] - `calls` [EXTRACTED]
- [[newFakeClientRepo()]] - `calls` [EXTRACTED]
- [[testDeps]] - `references` [EXTRACTED]

#graphify/code #graphify/EXTRACTED #community/Loyalty_Service_Tests