// internal/portfolio/validate.go
package portfolio

import (
	"fmt"
	"math"

	"github.com/mdmclean/kashmere-cli/internal/api"
)

const percentageTolerance = 0.01

// Validate checks that a portfolio's allocations and asset targets are internally
// consistent before the portfolio is sent to the API.
//
// Checks performed:
//  1. portfolio-level allocation percentages sum to 100 (skipped if empty)
//  2. asset targetPercentage values sum to 100 within each category,
//     but only for categories where ALL assets have a target set
func Validate(p api.Portfolio) error {
	if err := validateAllocationSum(p.Allocations); err != nil {
		return err
	}
	return validateAssetTargetSums(p.Assets)
}

func validateAllocationSum(allocations []api.Allocation) error {
	if len(allocations) == 0 {
		return nil
	}
	sum := 0.0
	for _, a := range allocations {
		sum += a.Percentage
	}
	if math.Abs(sum-100) > percentageTolerance {
		return fmt.Errorf("allocation percentages sum to %.2f%%, must equal 100%%", sum)
	}
	return nil
}

func validateAssetTargetSums(assets []api.Asset) error {
	type classInfo struct {
		sum    float64
		count  int
		hasNil bool
	}
	classes := map[string]*classInfo{}
	for _, a := range assets {
		if _, ok := classes[a.Category]; !ok {
			classes[a.Category] = &classInfo{}
		}
		info := classes[a.Category]
		info.count++
		if a.TargetPercentage == nil {
			info.hasNil = true
		} else {
			info.sum += *a.TargetPercentage
		}
	}

	for category, info := range classes {
		if info.hasNil {
			continue
		}
		if info.count == 0 {
			continue
		}
		if math.Abs(info.sum-100) > percentageTolerance {
			return fmt.Errorf("asset target percentages for %q sum to %.2f%%, must equal 100%%", category, info.sum)
		}
	}
	return nil
}
