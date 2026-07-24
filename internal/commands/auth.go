package commands

import (
	"context"
	"fmt"
	"io"

	pbauth "github.com/proofboard/proofboard/internal/auth"
	"github.com/spf13/cobra"
)

func newAuthCommand(ctx context.Context, out io.Writer) *cobra.Command {
	var rotateKey bool
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Connect the Proofboard Career Agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("auth: %w", err)
			}
			emailHash, emailErr := authEmailHash(ctx)
			if emailErr != nil {
				if existing, loadErr := runtime.credentials.Load(ctx); loadErr == nil && existing.EmailHash != "" {
					emailHash = existing.EmailHash
				} else {
					emailHash = ""
				}
			}
			service := pbauth.NewService(runtime.credentials, runtime.api, runtime.config.AgentAuthURL)
			credentials, err := service.Login(ctx, emailHash)
			if err != nil {
				return fmt.Errorf("auth login: %w", err)
			}
			keyStore := pbauth.NewDeviceKeyStore(runtime.homeDir)
			deviceKey, err := keyStore.Ensure(ctx, runtime.api, credentials.Token, rotateKey)
			if err != nil {
				return fmt.Errorf("ensure device key: %w", err)
			}
			credentials.DeviceKeyID = deviceKey.DeviceKeyID
			if err := runtime.credentials.Save(ctx, credentials); err != nil {
				return fmt.Errorf("persist device key id: %w", err)
			}
			if current, loadErr := runtime.state.Load(ctx); loadErr == nil {
				current.AuthReconnectPrompted = false
				_ = runtime.state.Save(ctx, current)
			}
			name := credentials.Username
			if name == "" {
				name = "Proofboard user"
			}
			_, err = fmt.Fprintf(out, "Authenticated as %s. Proofboard Career Agent is connected and projects will sync automatically.\n", name)
			return err
		},
	}
	cmd.Flags().BoolVar(&rotateKey, "rotate-key", false, "generate a new device signing key")
	return cmd
}
