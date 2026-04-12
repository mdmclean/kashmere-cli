package portfolio_test

import (
	"strings"
	"testing"

	"github.com/mdmclean/kashmere-cli/internal/api"
	"github.com/mdmclean/kashmere-cli/internal/portfolio"
)

func TestValidate_valid(t *testing.T) {
	p := api.Portfolio{
		Allocations: []api.Allocation{
			{Category: "US Equity", Percentage: 60},
			{Category: "Canadian Equity", Percentage: 40},
		},
		Assets: []api.Asset{
			{Category: "US Equity", TargetPercentage: ptr(60.0)},
			{Category: "US Equity", TargetPercentage: ptr(40.0)},
			{Category: "Canadian Equity", TargetPercentage: ptr(100.0)},
		},
	}
	if err := portfolio.Validate(p); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidate_allocationSumNot100(t *testing.T) {
	p := api.Portfolio{
		Allocations: []api.Allocation{
			{Category: "US Equity", Percentage: 60},
			{Category: "Canadian Equity", Percentage: 45},
		},
	}
	err := portfolio.Validate(p)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "105.00%") {
		t.Errorf("expected sum in error message, got: %v", err)
	}
}

func TestValidate_allocationEmpty_noError(t *testing.T) {
	p := api.Portfolio{Allocations: []api.Allocation{}}
	if err := portfolio.Validate(p); err != nil {
		t.Errorf("expected no error for empty allocations, got: %v", err)
	}
}

func TestValidate_assetTargetSumNot100(t *testing.T) {
	p := api.Portfolio{
		Allocations: []api.Allocation{
			{Category: "US Equity", Percentage: 100},
		},
		Assets: []api.Asset{
			{Category: "US Equity", TargetPercentage: ptr(60.0)},
			{Category: "US Equity", TargetPercentage: ptr(45.0)},
		},
	}
	err := portfolio.Validate(p)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `"US Equity"`) || !strings.Contains(err.Error(), "105.00%") {
		t.Errorf("expected class name and sum in error, got: %v", err)
	}
}

func TestValidate_assetTargetsPartial_noError(t *testing.T) {
	p := api.Portfolio{
		Allocations: []api.Allocation{
			{Category: "US Equity", Percentage: 100},
		},
		Assets: []api.Asset{
			{Category: "US Equity", TargetPercentage: ptr(60.0)},
			{Category: "US Equity", TargetPercentage: nil},
		},
	}
	if err := portfolio.Validate(p); err != nil {
		t.Errorf("expected no error for partial targets, got: %v", err)
	}
}

func TestValidate_assetTargetsAbsent_noError(t *testing.T) {
	p := api.Portfolio{
		Allocations: []api.Allocation{
			{Category: "US Equity", Percentage: 100},
		},
		Assets: []api.Asset{
			{Category: "US Equity", TargetPercentage: nil},
		},
	}
	if err := portfolio.Validate(p); err != nil {
		t.Errorf("expected no error when no targets set, got: %v", err)
	}
}

func TestValidate_floatRounding(t *testing.T) {
	p := api.Portfolio{
		Allocations: []api.Allocation{
			{Category: "A", Percentage: 33.33},
			{Category: "B", Percentage: 33.33},
			{Category: "C", Percentage: 33.34},
		},
		Assets: []api.Asset{
			{Category: "A", TargetPercentage: ptr(33.33)},
			{Category: "A", TargetPercentage: ptr(33.33)},
			{Category: "A", TargetPercentage: ptr(33.34)},
			{Category: "B", TargetPercentage: ptr(100.0)},
			{Category: "C", TargetPercentage: ptr(100.0)},
		},
	}
	if err := portfolio.Validate(p); err != nil {
		t.Errorf("expected no error for valid float rounding, got: %v", err)
	}
}
