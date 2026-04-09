// cmd/mortgage.go
package cmd

import (
	"fmt"
	"strconv"

	"github.com/mdmclean/kashmere-cli/internal/api"
	"github.com/spf13/cobra"
)

var mortgageCmd = &cobra.Command{
	Use:   "mortgage",
	Short: "Manage mortgages",
}

var mortgageListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all mortgages",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.APIBaseURL, cfg.APIKey, encKey)
		var mortgages []api.Mortgage
		if err := client.Get("/mortgages", &mortgages); err != nil {
			outputError(err, 0)
		}
		outputJSON(mortgages)
		return nil
	},
}

var mortgageGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a mortgage by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.APIBaseURL, cfg.APIKey, encKey)
		var m api.Mortgage
		if err := client.Get("/mortgages/"+args[0], &m); err != nil {
			outputError(err, 0)
		}
		outputJSON(m)
		return nil
	},
}

var (
	mName              string
	mDescription       string
	mOwner             string
	mInstitution       string
	mOriginalPrincipal string
	mCurrentBalance    string
	mInterestRate      string
	mPaymentAmount     string
	mPaymentFrequency  string
	mStartDate         string
	mTermEndDate       string
	mAmortizationYears string
	mExtraPayment      string
)

var mortgageCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new mortgage",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.APIBaseURL, cfg.APIKey, encKey)

		origPrincipal, err := strconv.ParseFloat(mOriginalPrincipal, 64)
		if err != nil {
			return fmt.Errorf("--original-principal must be a number: %w", err)
		}
		currentBal, err := strconv.ParseFloat(mCurrentBalance, 64)
		if err != nil {
			return fmt.Errorf("--current-balance must be a number: %w", err)
		}
		interestRate, err := strconv.ParseFloat(mInterestRate, 64)
		if err != nil {
			return fmt.Errorf("--interest-rate must be a number: %w", err)
		}
		paymentAmount, err := strconv.ParseFloat(mPaymentAmount, 64)
		if err != nil {
			return fmt.Errorf("--payment-amount must be a number: %w", err)
		}
		amortYears, err := strconv.Atoi(mAmortizationYears)
		if err != nil {
			return fmt.Errorf("--amortization-years must be an integer: %w", err)
		}

		body := map[string]any{
			"name":              mName,
			"owner":             mOwner,
			"institution":       mInstitution,
			"originalPrincipal": origPrincipal,
			"currentBalance":    currentBal,
			"interestRate":      interestRate,
			"paymentAmount":     paymentAmount,
			"paymentFrequency":  mPaymentFrequency,
			"startDate":         mStartDate,
			"termEndDate":       mTermEndDate,
			"amortizationYears": amortYears,
		}
		if mDescription != "" {
			body["description"] = mDescription
		}
		if mExtraPayment != "" {
			v, err := strconv.ParseFloat(mExtraPayment, 64)
			if err != nil {
				return fmt.Errorf("--extra-payment must be a number: %w", err)
			}
			body["extraPayment"] = v
		}

		var mortgage api.Mortgage
		if err := client.Post("/mortgages", body, &mortgage); err != nil {
			outputError(err, 0)
		}
		outputJSON(mortgage)
		return nil
	},
}

var mortgageUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a mortgage (fetches current, merges flags, puts full object)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.APIBaseURL, cfg.APIKey, encKey)

		updates := map[string]any{}
		if cmd.Flags().Changed("name") {
			updates["name"] = mName
		}
		if cmd.Flags().Changed("description") {
			updates["description"] = mDescription
		}
		if cmd.Flags().Changed("owner") {
			updates["owner"] = mOwner
		}
		if cmd.Flags().Changed("institution") {
			updates["institution"] = mInstitution
		}
		if cmd.Flags().Changed("original-principal") {
			v, _ := strconv.ParseFloat(mOriginalPrincipal, 64)
			updates["originalPrincipal"] = v
		}
		if cmd.Flags().Changed("current-balance") {
			v, _ := strconv.ParseFloat(mCurrentBalance, 64)
			updates["currentBalance"] = v
		}
		if cmd.Flags().Changed("interest-rate") {
			v, _ := strconv.ParseFloat(mInterestRate, 64)
			updates["interestRate"] = v
		}
		if cmd.Flags().Changed("payment-amount") {
			v, _ := strconv.ParseFloat(mPaymentAmount, 64)
			updates["paymentAmount"] = v
		}
		if cmd.Flags().Changed("payment-frequency") {
			updates["paymentFrequency"] = mPaymentFrequency
		}
		if cmd.Flags().Changed("start-date") {
			updates["startDate"] = mStartDate
		}
		if cmd.Flags().Changed("term-end-date") {
			updates["termEndDate"] = mTermEndDate
		}
		if cmd.Flags().Changed("amortization-years") {
			v, _ := strconv.Atoi(mAmortizationYears)
			updates["amortizationYears"] = v
		}
		if cmd.Flags().Changed("extra-payment") {
			v, _ := strconv.ParseFloat(mExtraPayment, 64)
			updates["extraPayment"] = v
		}
		if len(updates) == 0 {
			return fmt.Errorf("no flags provided — nothing to update")
		}

		var result api.Mortgage
		if err := client.MergeAndUpdate("/mortgages/"+args[0], updates, &result); err != nil {
			outputError(err, 0)
		}
		outputJSON(result)
		return nil
	},
}

func init() {
	for _, c := range []*cobra.Command{mortgageCreateCmd, mortgageUpdateCmd} {
		c.Flags().StringVar(&mName, "name", "", "Mortgage name")
		c.Flags().StringVar(&mDescription, "description", "", "Description")
		c.Flags().StringVar(&mOwner, "owner", "", "Owner: person1|person2|joint")
		c.Flags().StringVar(&mInstitution, "institution", "", "Institution name")
		c.Flags().StringVar(&mOriginalPrincipal, "original-principal", "", "Original principal amount")
		c.Flags().StringVar(&mCurrentBalance, "current-balance", "", "Current balance")
		c.Flags().StringVar(&mInterestRate, "interest-rate", "", "Annual interest rate (e.g. 5.25)")
		c.Flags().StringVar(&mPaymentAmount, "payment-amount", "", "Payment amount")
		c.Flags().StringVar(&mPaymentFrequency, "payment-frequency", "", "Frequency: monthly|bi-weekly|accelerated-bi-weekly|weekly")
		c.Flags().StringVar(&mStartDate, "start-date", "", "Start date YYYY-MM-DD")
		c.Flags().StringVar(&mTermEndDate, "term-end-date", "", "Term end date YYYY-MM-DD")
		c.Flags().StringVar(&mAmortizationYears, "amortization-years", "", "Amortization period in years")
		c.Flags().StringVar(&mExtraPayment, "extra-payment", "", "Extra payment per period")
	}
	mortgageCreateCmd.MarkFlagRequired("name")
	mortgageCreateCmd.MarkFlagRequired("owner")
	mortgageCreateCmd.MarkFlagRequired("institution")
	mortgageCreateCmd.MarkFlagRequired("original-principal")
	mortgageCreateCmd.MarkFlagRequired("current-balance")
	mortgageCreateCmd.MarkFlagRequired("interest-rate")
	mortgageCreateCmd.MarkFlagRequired("payment-amount")
	mortgageCreateCmd.MarkFlagRequired("payment-frequency")
	mortgageCreateCmd.MarkFlagRequired("start-date")
	mortgageCreateCmd.MarkFlagRequired("term-end-date")
	mortgageCreateCmd.MarkFlagRequired("amortization-years")

	mortgageCmd.AddCommand(mortgageListCmd, mortgageGetCmd, mortgageCreateCmd, mortgageUpdateCmd)
	rootCmd.AddCommand(mortgageCmd)
}
