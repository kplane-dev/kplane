package cli

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
)

func newDoctorCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check local prerequisites for kplane",
		RunE: func(cmd *cobra.Command, args []string) error {
			checks := []string{"kind", "kubectl", "docker"}
			var missing []string
			for _, bin := range checks {
				if _, err := exec.LookPath(bin); err != nil {
					missing = append(missing, bin)
				}
			}
			if len(missing) > 0 {
				return fmt.Errorf("missing dependencies: %s", missing)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	}
	return cmd
}
