// cmd/settings.go
package cmd

import (
	"fmt"

	"github.com/kashemere/kashemere-cli/internal/api"
	"github.com/spf13/cobra"
)

var settingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "Manage account settings",
}

var settingsGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get current settings",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.APIBaseURL, cfg.APIKey, encKey)
		var settings api.Settings
		if err := client.Get("/settings", &settings); err != nil {
			outputError(err, 0)
		}
		outputJSON(settings)
		return nil
	},
}

var (
	settingsPerson1Name         string
	settingsPerson2Name         string
	settingsAccountType         string
	settingsDisplayCurrency     string
	settingsOnboardingDismissed bool
)

var settingsUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update settings (fetches current, merges flags, puts full object)",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.APIBaseURL, cfg.APIKey, encKey)

		updates := map[string]any{}
		if cmd.Flags().Changed("person1-name") {
			updates["person1Name"] = settingsPerson1Name
		}
		if cmd.Flags().Changed("person2-name") {
			updates["person2Name"] = settingsPerson2Name
		}
		if cmd.Flags().Changed("account-type") {
			updates["accountType"] = settingsAccountType
		}
		if cmd.Flags().Changed("display-currency") {
			updates["displayCurrency"] = settingsDisplayCurrency
		}
		if cmd.Flags().Changed("onboarding-dismissed") {
			updates["onboardingDismissed"] = settingsOnboardingDismissed
		}
		if len(updates) == 0 {
			return fmt.Errorf("no flags provided — nothing to update")
		}

		var result api.Settings
		if err := client.MergeAndUpdate("/settings", updates, &result); err != nil {
			outputError(err, 0)
		}
		outputJSON(result)
		return nil
	},
}

func init() {
	settingsUpdateCmd.Flags().StringVar(&settingsPerson1Name, "person1-name", "", "Name for person 1")
	settingsUpdateCmd.Flags().StringVar(&settingsPerson2Name, "person2-name", "", "Name for person 2")
	settingsUpdateCmd.Flags().StringVar(&settingsAccountType, "account-type", "", "Account type: single|couple")
	settingsUpdateCmd.Flags().StringVar(&settingsDisplayCurrency, "display-currency", "", "Display currency: CAD|USD")
	settingsUpdateCmd.Flags().BoolVar(&settingsOnboardingDismissed, "onboarding-dismissed", false, "Mark onboarding as dismissed")

	settingsCmd.AddCommand(settingsGetCmd, settingsUpdateCmd)
	rootCmd.AddCommand(settingsCmd)
}
