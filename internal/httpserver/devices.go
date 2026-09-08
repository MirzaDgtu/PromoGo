package httpserver

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/MirzaDgtu/PromoGo/internal/domain"
)

const (
	maxPushTokenLen = 4096
	maxPlatformLen  = 32
)

type registerDeviceRequestBody struct {
	Platform  string `json:"platform"`
	PushToken string `json:"push_token"`
}

type registerDeviceResponseBody struct {
	DeviceID int64 `json:"device_id"`
}

// handleRegisterDevice returns a handler for POST /api/v1/me/devices:
// registers or refreshes an FCM push token for the caller's CustomerAccount
// (Q-P0-103/DEC-014). Must run behind RequireCustomerSession.
func handleRegisterDevice(devices domain.CustomerDeviceRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		customerAccountID, ok := customerFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var body registerDeviceRequestBody
		if err := dec.Decode(&body); err != nil ||
			body.Platform == "" || len(body.Platform) > maxPlatformLen ||
			body.PushToken == "" || len(body.PushToken) > maxPushTokenLen {
			writeError(w, http.StatusBadRequest, "platform and push_token are required")
			return
		}

		device := &domain.CustomerDevice{
			CustomerAccountID: customerAccountID,
			Platform:          body.Platform,
			PushToken:         body.PushToken,
		}
		if err := devices.Upsert(r.Context(), device); err != nil {
			log.ErrorContext(r.Context(), "register device", "customer_account_id", customerAccountID, "error", err)
			writeError(w, http.StatusInternalServerError, "register device")
			return
		}

		writeJSON(w, http.StatusOK, registerDeviceResponseBody{DeviceID: device.ID})
	}
}

// handleRevokeDevice returns a handler for DELETE /api/v1/me/devices/{deviceID}:
// revokes one of the caller's own devices. Must run behind
// RequireCustomerSession. Revoke is scoped to the caller's own
// CustomerAccountID, so a device belonging to another account is reported
// as not found rather than forbidden — it must not confirm the ID exists.
func handleRevokeDevice(devices domain.CustomerDeviceRepository, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		customerAccountID, ok := customerFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		deviceID, err := strconv.ParseInt(r.PathValue("deviceID"), 10, 64)
		if err != nil || deviceID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid device id")
			return
		}

		err = devices.Revoke(r.Context(), deviceID, customerAccountID)
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "device not found")
			return
		}
		if err != nil {
			log.ErrorContext(r.Context(), "revoke device", "customer_account_id", customerAccountID, "device_id", deviceID, "error", err)
			writeError(w, http.StatusInternalServerError, "revoke device")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
