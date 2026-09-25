package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/proofboard/proofboard/internal/api"
	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/proofboard/proofboard/internal/crypto"
	"github.com/proofboard/proofboard/internal/logging"
	"github.com/proofboard/proofboard/internal/model"
)

// transmitSyncPayload signs payload with the device key and sends it via
// runtime.api.Sync, threading the same auth-retry/spinner machinery and
// no-linked-project org/repo-hash swap that every sync (including a
// `sync --resync` replay) relies on. Returns the receipt and the payload as
// actually signed and transmitted (including the final DeviceKeyID/
// DeviceSignature) so callers can cache exactly what left the machine.
func transmitSyncPayload(ctx context.Context, out io.Writer, runtime runtimeContext, identity model.RemoteIdentity, triggerSource string, fromAgent bool, payload model.SyncPayload, refreshIdentity func(*model.SyncPayload) error) (model.SyncReceipt, model.SyncPayload, error) {
	var receipt model.SyncReceipt
	var transmitted model.SyncPayload
	transmit := func() error {
		freshCredentials, err := runtime.credentials.Load(ctx)
		if err != nil {
			return fmt.Errorf("reload credentials: %w", err)
		}
		if freshCredentials.Token == "" {
			return fmt.Errorf("missing authentication token")
		}
		signedPayload := payload
		// Device signing is mandatory: the backend unconditionally
		// rejects any sync payload missing deviceKeyId/deviceSignature
		// (cli-ingest.service.ts), so there is no optional path here.
		keyStore := pbauth.NewDeviceKeyStore(runtime.homeDir)
		deviceKey, keyErr := keyStore.Ensure(ctx, runtime.api, freshCredentials.Token, false)
		if keyErr != nil {
			_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource,
				"register device key", "warning", keyErr.Error())
			return fmt.Errorf("register device signing key: %w", keyErr)
		}
		freshCredentials.DeviceKeyID = deviceKey.DeviceKeyID
		if err := runtime.credentials.Save(ctx, freshCredentials); err != nil {
			return fmt.Errorf("persist device key id: %w", err)
		}
		signedPayload.DeviceKeyID = deviceKey.DeviceKeyID

		// The signature has to cover the payload exactly as sent, so it
		// is recomputed whenever the payload changes.
		sign := func(candidate model.SyncPayload) (model.SyncPayload, error) {
			candidate.DeviceSignature = ""
			signingBytes, err := crypto.CanonicalJSON(candidate)
			if err != nil {
				return candidate, fmt.Errorf("marshal sync payload for signing: %w", err)
			}
			signature, err := keyStore.Sign(ctx, signingBytes)
			if err != nil {
				return candidate, fmt.Errorf("sign sync payload: %w", err)
			}
			candidate.DeviceSignature = signature
			return candidate, nil
		}

		signedPayload, signErr := sign(signedPayload)
		if signErr != nil {
			return signErr
		}
		syncReceipt, syncErr := runtime.api.Sync(ctx, freshCredentials.Token, signedPayload)
		if isNoLinkedProjectError(syncErr) {
			signedPayload.OrgHash, signedPayload.RepoHash = signedPayload.RepoHash, signedPayload.OrgHash
			signedPayload, signErr = sign(signedPayload)
			if signErr != nil {
				return signErr
			}
			syncReceipt, syncErr = runtime.api.Sync(ctx, freshCredentials.Token, signedPayload)
		}
		if syncErr == nil {
			receipt = syncReceipt
			transmitted = signedPayload
		}
		return syncErr
	}
	attempt := func() error {
		if fromAgent {
			return retryAfterAuthForAgent(ctx, out, runtime, transmit)
		}
		return retryAfterAuth(ctx, out, "project synchronization", transmit)
	}
	err := withSpinner(out, "Transmitting proof…", triggerSource == "manual", func() error {
		err := attempt()
		if !isNoLinkedProjectError(err) {
			return err
		}
		// The backend has no project for this repository under the account
		// that is now authenticated. That happens most often straight after a
		// mid-sync reconnect: the session is repaired, the repository link is
		// not, and the sync fails with a bare 400 that says nothing a user can
		// act on. Everything needed to repair it is already in hand, so link
		// and retransmit rather than reporting a status code.
		_ = logging.WriteSyncLog(runtime.homeDir, identity.RepoHash, triggerSource,
			"Phase 7: Transmission", "warning", "no linked project; connecting this repository")
		if _, printErr := fmt.Fprintln(out, "This repository is not connected to your Proofboard account yet. Connecting it now..."); printErr != nil {
			return printErr
		}
		if linkErr := runLinkFlow(ctx, out); linkErr != nil {
			return linkErr
		}
		// The payload's emailHash is an HMAC keyed with the project's
		// emailHashKey, which the link response supplies. Reconnecting can
		// land on a different account with a different key, so a payload built
		// before linking carries a hash the backend cannot attribute to
		// anyone. Rebuild it before resending.
		if refreshIdentity != nil {
			if refreshErr := refreshIdentity(&payload); refreshErr != nil {
				return refreshErr
			}
		}
		retryErr := attempt()
		if isNoLinkedProjectError(retryErr) {
			// Linking reported success and the backend still does not see a
			// project. Say that, rather than repeating "run proofboard link"
			// for something that was just run.
			return errors.New("this repository was connected but the server still reports no linked project for it; check that the repository's remote matches the project on proofboard.io")
		}
		return retryErr
	})
	return receipt, transmitted, err
}

// formatRetryDuration renders a Retry-After duration as a whole-minute,
// human-readable hint ("15m"), rounding up so the hint never undersells how
// long the throttle window actually is.
func formatRetryDuration(d time.Duration) string {
	minutes := int((d + time.Minute - 1) / time.Minute)
	if minutes < 1 {
		minutes = 1
	}
	return fmt.Sprintf("%dm", minutes)
}

func isNoLinkedProjectError(err error) bool {
	var apiErr *api.Error
	if errors.As(err, &apiErr) {
		return strings.Contains(strings.ToLower(apiErr.Message), "no linked project")
	}
	return false
}
