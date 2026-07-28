package httpserver

import "github.com/MirzaDgtu/PromoGo/internal/auth"

// maskPhoneIf returns auth.MaskPhone(phone) when masked is true, phone
// otherwise. Handlers call this with staff/service.StaffPrincipal.IsSupportViewerOnly
// for the caller's scope — support_viewer gets read access to client data
// but never a full phone number.
func maskPhoneIf(phone string, masked bool) string {
	if masked {
		return auth.MaskPhone(phone)
	}
	return phone
}
