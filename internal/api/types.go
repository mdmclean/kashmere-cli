// internal/api/types.go
package api

// Portfolio mirrors the TypeScript Portfolio type from @fp/shared.
type Portfolio struct {
	ID                     string       `json:"id"`
	AccountID              string       `json:"accountId"`
	Name                   string       `json:"name"`
	Description            string       `json:"description,omitempty"`
	Institution            string       `json:"institution"`
	Owner                  string       `json:"owner"` // person1|person2|joint
	ManagementType         string       `json:"managementType"` // self|auto
	GoalID                 string       `json:"goalId,omitempty"`
	TotalValue             float64      `json:"totalValue"`
	MarketValue            *float64     `json:"marketValue,omitempty"`
	Allocations            []Allocation `json:"allocations"`
	Assets                 []Asset      `json:"assets"`
	MinTransactionAmount   *float64     `json:"minTransactionAmount,omitempty"`
	MinTransactionCurrency string       `json:"minTransactionCurrency,omitempty"`
	CreatedAt              string       `json:"createdAt"`
	UpdatedAt              string       `json:"updatedAt"`
}

type Allocation struct {
	Category   string  `json:"category"`
	Percentage float64 `json:"percentage"`
}

type Asset struct {
	ID               string   `json:"id"`
	Ticker           string   `json:"ticker"`
	Name             string   `json:"name"`
	Category         string   `json:"category"`
	Currency         string   `json:"currency,omitempty"`
	Exchange         string   `json:"exchange,omitempty"`
	Quantity         float64  `json:"quantity"`
	TargetPercentage *float64 `json:"targetPercentage,omitempty"`
	BookValue        *float64 `json:"bookValue,omitempty"`
}

// Goal mirrors the TypeScript Goal type.
type Goal struct {
	ID          string       `json:"id"`
	AccountID   string       `json:"accountId"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Target      *GoalTarget  `json:"target,omitempty"`
	Allocations []Allocation `json:"allocations,omitempty"`
	CreatedAt   string       `json:"createdAt"`
	UpdatedAt   string       `json:"updatedAt"`
}

type GoalTarget struct {
	Type  string  `json:"type"`  // fixed|percentage
	Value float64 `json:"value"`
}

// CashFlow mirrors the TypeScript CashFlow type.
type CashFlow struct {
	ID            string  `json:"id"`
	PortfolioID   string  `json:"portfolioId"`
	AccountID     string  `json:"accountId,omitempty"`
	Type          string  `json:"type"` // deposit|withdrawal
	Amount        float64 `json:"amount"`
	Date          string  `json:"date"`
	CashAssetID   string  `json:"cashAssetId"`
	CashAssetName string  `json:"cashAssetName"`
	TransferID    string  `json:"transferId,omitempty"`
	Description   string  `json:"description,omitempty"`
	CreatedAt     string  `json:"createdAt"`
}

// Mortgage mirrors the TypeScript Mortgage type.
type Mortgage struct {
	ID                string   `json:"id"`
	AccountID         string   `json:"accountId"`
	Name              string   `json:"name"`
	Description       string   `json:"description,omitempty"`
	Owner             string   `json:"owner"` // person1|person2|joint
	Institution       string   `json:"institution"`
	OriginalPrincipal float64  `json:"originalPrincipal"`
	CurrentBalance    float64  `json:"currentBalance"`
	InterestRate      float64  `json:"interestRate"`
	PaymentAmount     float64  `json:"paymentAmount"`
	PaymentFrequency  string   `json:"paymentFrequency"` // monthly|bi-weekly|accelerated-bi-weekly|weekly
	StartDate         string   `json:"startDate"`
	TermEndDate       string   `json:"termEndDate"`
	AmortizationYears int      `json:"amortizationYears"`
	ExtraPayment      *float64 `json:"extraPayment,omitempty"`
	CreatedAt         string   `json:"createdAt"`
	UpdatedAt         string   `json:"updatedAt"`
}

// Settings mirrors the TypeScript Settings type.
type Settings struct {
	ID                  string `json:"id"`
	AccountID           string `json:"accountId"`
	Person1Name         string `json:"person1Name"`
	Person2Name         string `json:"person2Name"`
	AccountType         string `json:"accountType"`    // single|couple
	DisplayCurrency     string `json:"displayCurrency"` // CAD|USD
	UpdatedAt           string `json:"updatedAt"`
	OnboardingDismissed bool   `json:"onboardingDismissed"`
}

// PortfolioSnapshot mirrors TypeScript PortfolioSnapshot.
type PortfolioSnapshot struct {
	ID          string  `json:"id"`
	PortfolioID string  `json:"portfolioId"`
	AccountID   string  `json:"accountId"`
	Timestamp   string  `json:"timestamp"`
	TotalValue  float64 `json:"totalValue"`
	CreatedAt   string  `json:"createdAt"`
}

// TickerPrice mirrors TypeScript TickerPrice.
type TickerPrice struct {
	Ticker           string   `json:"ticker"`
	Exchange         string   `json:"exchange,omitempty"`
	Currency         string   `json:"currency,omitempty"`
	Name             string   `json:"name,omitempty"`
	Category         string   `json:"category,omitempty"`
	PreviousClose    *float64 `json:"previousClose"`
	LatestPrice      *float64 `json:"latestPrice"`
	Open             *float64 `json:"open"`
	High             *float64 `json:"high"`
	Low              *float64 `json:"low"`
	FiftyTwoWeekHigh *float64 `json:"fiftyTwoWeekHigh"`
	FiftyTwoWeekLow  *float64 `json:"fiftyTwoWeekLow"`
	LastUpdated      string   `json:"lastUpdated"`
	MarketTime       string   `json:"marketTime,omitempty"`
}

// encryptedDoc is the wire format for encrypted payloads.
type encryptedDoc struct {
	ID        string `json:"id,omitempty"`
	AccountID string `json:"accountId,omitempty"`
	Encrypted string `json:"_encrypted"`
}
