---
source_file: "internal/service/loyalty_test.go"
type: "code"
community: "Loyalty Service Tests"
location: "L174"
tags:
  - graphify/code
  - graphify/EXTRACTED
  - community/Loyalty_Service_Tests
---

# pointsConfig()

## Connections
- [[LoyaltyConfig]] - `references` [EXTRACTED]
- [[TestAccrue_ConcurrentClientCreateRaceRecovered()]] - `calls` [EXTRACTED]
- [[TestAccrue_ConflictingPhoneDoesNotCreateOrphanClient()]] - `calls` [EXTRACTED]
- [[TestAccrue_IdempotencyConflictOnMismatchedAmount()]] - `calls` [EXTRACTED]
- [[TestAccrue_ReplayReturnsOriginalResult()]] - `calls` [EXTRACTED]
- [[TestRedeem_CapAppliedBeforeBalanceCheck()]] - `calls` [EXTRACTED]
- [[TestRedeem_CapsAtMaxRedeemPercentAndRejectsInsufficientBalance()]] - `calls` [EXTRACTED]
- [[TestRedeem_CrossStoreClientRejected()]] - `calls` [EXTRACTED]
- [[TestRedeem_IdempotencyConflictOnMismatchedPoints()]] - `calls` [EXTRACTED]
- [[loyalty_test.go]] - `contains` [EXTRACTED]

#graphify/code #graphify/EXTRACTED #community/Loyalty_Service_Tests