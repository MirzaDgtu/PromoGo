package domain

// Role is one of PromoGo's fixed MVP staff roles. Handlers and middleware
// must never switch on Role directly — always resolve to Permission via
// RolePermissions (see httpserver.RequireStaff) so adding a role or
// reshuffling what it can do never requires touching handler code.
type Role string

const (
	RolePlatformAdmin Role = "platform_admin"
	RoleRetailerAdmin Role = "retailer_admin"
	RoleStoreManager  Role = "store_manager"
	RoleSupportViewer Role = "support_viewer"
)

// Permission is a single grantable capability, checked by httpserver
// handlers via RequireStaff(perm) instead of role-name comparisons.
type Permission string

const (
	PermOrganizationsManage Permission = "organizations.manage"
	PermStoresRead          Permission = "stores.read"
	PermStoresManage        Permission = "stores.manage"
	PermStaffManage         Permission = "staff.manage"
	PermClientsRead         Permission = "clients.read"
	PermTransactionsRead    Permission = "transactions.read"
	PermLoyaltyConfigRead   Permission = "loyalty_config.read"
	PermLoyaltyConfigWrite  Permission = "loyalty_config.write"
	PermAPIKeysRead         Permission = "api_keys.read"
	PermAPIKeysRotate       Permission = "api_keys.rotate"
	PermAuditRead           Permission = "audit.read"
)

// RolePermissions is the fixed MVP mapping from Role to the Permissions it
// grants. platform_admin gets every permission (network-wide operator);
// retailer_admin gets full control within its own organization but not
// cross-organization management; store_manager is read/write on its own
// store's operational data but can't manage staff or API keys;
// support_viewer is read-only everywhere and additionally has its client
// data masked at the handler layer (see httpserver.maskPII).
var RolePermissions = map[Role][]Permission{
	RolePlatformAdmin: {
		PermOrganizationsManage, PermStoresRead, PermStoresManage,
		PermStaffManage, PermClientsRead, PermTransactionsRead,
		PermLoyaltyConfigRead, PermLoyaltyConfigWrite,
		PermAPIKeysRead, PermAPIKeysRotate, PermAuditRead,
	},
	RoleRetailerAdmin: {
		PermStoresRead, PermStoresManage, PermStaffManage,
		PermClientsRead, PermTransactionsRead,
		PermLoyaltyConfigRead, PermLoyaltyConfigWrite,
		PermAPIKeysRead, PermAPIKeysRotate, PermAuditRead,
	},
	RoleStoreManager: {
		PermStoresRead, PermClientsRead, PermTransactionsRead,
		PermLoyaltyConfigRead, PermLoyaltyConfigWrite,
	},
	RoleSupportViewer: {
		PermStoresRead, PermClientsRead, PermTransactionsRead,
		PermLoyaltyConfigRead, PermAPIKeysRead, PermAuditRead,
	},
}

// HasPermission reports whether role grants perm.
func HasPermission(role Role, perm Permission) bool {
	for _, p := range RolePermissions[role] {
		if p == perm {
			return true
		}
	}
	return false
}
