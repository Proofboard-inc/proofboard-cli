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
		Short: "Authenticate Proofboard CLI",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := loadRuntime(ctx)
			if err != nil {
				return fmt.Errorf("auth: %w", err)
			}
			emailHash, err := authEmailHash(ctx)
			if err != nil {
				return fmt.Errorf("auth email bridge: %w", err)
			}
			service := pbauth.NewService(runtime.credentials, runtime.api)
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
			name := credentials.Username
			if name == "" {
				name = "Proofboard user"
			}
			_, err = fmt.Fprintf(out, "Authenticated as %s. Run proofboard link inside a repository to get started.\n", name)
			return err
		},
	}
	cmd.Flags().BoolVar(&rotateKey, "rotate-key", false, "generate a new device signing key")
	return cmd
}
