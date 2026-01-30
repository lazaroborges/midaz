package mmodel

import (
	"time"

	"github.com/google/uuid"
)

// CreateAccountInput is a struct designed to encapsulate request create payload data.
//
// swagger:model CreateAccountInput
//
//	@Description	Request payload for creating a new account within a ledger. Accounts represent individual financial entities such as bank accounts, credit cards, expense categories, or any other financial buckets within a ledger. Accounts are identified by a unique ID, can have aliases for easy reference, and are associated with a specific asset type.
//
//	@example		{
//	  "name": "Corporate Checking Account",
//	  "assetCode": "USD",
//	  "status": {
//	    "code": "ACTIVE"
//	  },
//	  "alias": "@treasury_checking",
//	  "type": "deposit",
//	  "metadata": {
//	    "department": "Treasury",
//	    "purpose": "Operating Expenses",
//	    "region": "Global"
//	  }
//	}
type CreateAccountInput struct {
	// Human-readable name of the account
	// required: false
	// example: Corporate Checking Account
	// maxLength: 256
	Name string `json:"name" validate:"max=256" example:"Corporate Checking Account" maxLength:"256"`

	// ID of the parent account if this is a subaccount (optional)
	// required: false
	// format: uuid
	ParentAccountID *string `json:"parentAccountId" validate:"omitempty,uuid" format:"uuid"`

	// Optional external identifier for linking to external systems
	// required: false
	// example: EXT-ACC-12345
	// maxLength: 256
	EntityID *string `json:"entityId" validate:"omitempty,max=256" example:"EXT-ACC-12345" maxLength:"256"`

	// Asset code that this account will use for balances and transactions
	// required: true
	// example: USD
	// maxLength: 100
	AssetCode string `json:"assetCode" validate:"required,max=100" example:"USD" maxLength:"100"`

	// ID of the portfolio this account belongs to (optional)
	// required: false
	// format: uuid
	PortfolioID *string `json:"portfolioId" validate:"omitempty,uuid" format:"uuid"`

	// ID of the segment this account belongs to (optional)
	// required: false
	// format: uuid
	SegmentID *string `json:"segmentId" validate:"omitempty,uuid" format:"uuid"`

	// Current operating status of the account
	// required: false
	Status Status `json:"status"`

	// Unique alias for the account (optional, must follow alias format rules)
	// required: false
	// example: @treasury_checking
	// maxLength: 100
	Alias *string `json:"alias" validate:"omitempty,max=100,prohibitedexternalaccountprefix,invalidaliascharacters" example:"@treasury_checking" maxLength:"100"`

	// Type of the account
	// required: true
	// example: deposit
	// maxLength: 256
	Type string `json:"type" validate:"required,max=256,invalidstrings=external" example:"deposit"`

	// Whether the account should start blocked
	// required: false
	// default: false
	Blocked *bool `json:"blocked"`

	// Custom key-value pairs for extending the account information
	// required: false
	// example: {"department": "Treasury", "purpose": "Operating Expenses", "region": "Global"}
	Metadata map[string]any `json:"metadata" validate:"dive,keys,keymax=100,endkeys,nonested,valuemax=2000"`
} //	@name	CreateAccountInput

// UpdateAccountInput is a struct designed to encapsulate request update payload data.
//
// swagger:model UpdateAccountInput
//
//	@Description	Request payload for updating an existing account. All fields are optional - only specified fields will be updated. Omitted fields will remain unchanged. This allows partial updates to account properties such as name, status, portfolio, segment, and metadata.
//
//	@example		{
//	  "name": "Primary Corporate Checking Account",
//	  "status": {
//	    "code": "ACTIVE"
//	  },
//	  "metadata": {
//	    "department": "Global Treasury",
//	    "purpose": "Primary Operations",
//	    "region": "Global"
//	  }
//	}
type UpdateAccountInput struct {
	// Updated name of the account
	// required: false
	// example: Primary Corporate Checking Account
	// maxLength: 256
	Name string `json:"name" validate:"max=256" example:"Primary Corporate Checking Account" maxLength:"256"`

	// Updated segment ID for the account
	// required: false
	// format: uuid
	SegmentID *string `json:"segmentId" validate:"omitempty,uuid" format:"uuid"`

	// Updated portfolio ID for the account
	// required: false
	// format: uuid
	PortfolioID *string `json:"portfolioId" validate:"omitempty,uuid" format:"uuid"`

	// Optional external identifier for linking to external systems
	// required: false
	// example: EXT-ACC-12345
	// maxLength: 256
	EntityID *string `json:"entityId" validate:"omitempty,max=256" example:"EXT-ACC-12345" maxLength:"256"`

	// Updated status of the account
	// required: false
	Status Status `json:"status"`

	// Whether the account should be blocked
	// required: false
	Blocked *bool `json:"blocked"`

	// Updated custom key-value pairs for extending the account information
	// required: false
	// example: {"department": "Global Treasury", "purpose": "Primary Operations", "region": "Global"}
	Metadata map[string]any `json:"metadata" validate:"dive,keys,keymax=100,endkeys,omitempty,nonested,valuemax=2000"`
} //	@name	UpdateAccountInput

// Account is a struct designed to encapsulate response payload data.
//
// swagger:model Account
//
//	@Description	Complete account entity containing all fields including system-generated fields like ID, creation timestamps, and metadata. This is the response format for account operations. Accounts represent individual financial entities (bank accounts, cards, expense categories, etc.) within a ledger and are the primary structures for tracking balances and transactions.
//
//	@example		{
//	  "id": "a1b2c3d4-e5f6-7890-abcd-1234567890ab",
//	  "name": "Corporate Checking Account",
//	  "assetCode": "USD",
//	  "organizationId": "b2c3d4e5-f6a1-7890-bcde-2345678901cd",
//	  "ledgerId": "c3d4e5f6-a1b2-7890-cdef-3456789012de",
//	  "portfolioId": "d4e5f6a1-b2c3-7890-defg-4567890123ef",
//	  "segmentId": "e5f6a1b2-c3d4-7890-efgh-5678901234fg",
//	  "status": {
//	    "code": "ACTIVE"
//	  },
//	  "alias": "@treasury_checking",
//	  "type": "deposit",
//	  "createdAt": "2022-04-15T09:30:00Z",
//	  "updatedAt": "2022-04-15T09:30:00Z",
//	  "metadata": {
//	    "department": "Treasury",
//	    "purpose": "Operating Expenses",
//	    "region": "Global"
//	  }
//	}
type Account struct {
	// Unique identifier for the account (UUID format)
	// example: 00000000-0000-0000-0000-000000000000
	// format: uuid
	ID string `json:"id" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`

	// Human-readable name of the account
	// example: Corporate Checking Account
	// maxLength: 256
	Name string `json:"name" example:"Corporate Checking Account" maxLength:"256"`

	// ID of the parent account if this is a sub-account (UUID format)
	// example: 00000000-0000-0000-0000-000000000000
	// format: uuid
	ParentAccountID *string `json:"parentAccountId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`

	// Optional external identifier for linking to external systems
	// example: EXT-ACC-12345
	// maxLength: 256
	EntityID *string `json:"entityId" example:"EXT-ACC-12345" maxLength:"256"`

	// Asset code associated with this account (determines currency/asset type)
	// example: USD
	// maxLength: 100
	AssetCode string `json:"assetCode" example:"USD" maxLength:"100"`

	// ID of the organization that owns this account (UUID format)
	// example: 00000000-0000-0000-0000-000000000000
	// format: uuid
	OrganizationID string `json:"organizationId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`

	// ID of the ledger this account belongs to (UUID format)
	// example: 00000000-0000-0000-0000-000000000000
	// format: uuid
	LedgerID string `json:"ledgerId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`

	// ID of the portfolio this account belongs to (UUID format)
	// example: 00000000-0000-0000-0000-000000000000
	// format: uuid
	PortfolioID *string `json:"portfolioId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`

	// ID of the segment this account belongs to (UUID format)
	// example: 00000000-0000-0000-0000-000000000000
	// format: uuid
	SegmentID *string `json:"segmentId" example:"00000000-0000-0000-0000-000000000000" format:"uuid"`

	// Current operating status of the account
	Status Status `json:"status"`

	// Unique alias for the account (makes referencing easier)
	// example: @treasury_checking
	// maxLength: 100
	Alias *string `json:"alias" example:"@treasury_checking" maxLength:"100"`

	// Type of the account.
	// example: deposit
	Type string `json:"type" example:"deposit"`

	// Indicates if the account is blocked
	Blocked *bool `json:"blocked"`

	// Timestamp when the account was created (RFC3339 format)
	// example: 2021-01-01T00:00:00Z
	// format: date-time
	CreatedAt time.Time `json:"createdAt" example:"2021-01-01T00:00:00Z" format:"date-time"`

	// Timestamp when the account was last updated (RFC3339 format)
	// example: 2021-01-01T00:00:00Z
	// format: date-time
	UpdatedAt time.Time `json:"updatedAt" example:"2021-01-01T00:00:00Z" format:"date-time"`

	// Timestamp when the account was soft deleted, null if not deleted (RFC3339 format)
	// example: null
	// format: date-time
	DeletedAt *time.Time `json:"deletedAt" example:"2021-01-01T00:00:00Z" format:"date-time"`

	// Custom key-value pairs for extending the account information
	// example: {"department": "Treasury", "purpose": "Operating Expenses", "region": "Global"}
	Metadata map[string]any `json:"metadata,omitempty"`
} //	@name	Account

// IDtoUUID converts the account's string ID to a UUID object
//
// Returns the UUID representation of the account's ID
func (a *Account) IDtoUUID() uuid.UUID {
	return uuid.MustParse(a.ID)
}

// Accounts struct to return a paginated list of accounts.
//
// swagger:model Accounts
//
//	@Description	Paginated list of accounts with metadata about the current page, limit, and the account items themselves. Used for list operations.
//
//	@example		{
//	  "items": [
//	    {
//	      "id": "a1b2c3d4-e5f6-7890-abcd-1234567890ab",
//	      "name": "Corporate Checking Account",
//	      "assetCode": "USD",
//	      "ledgerId": "c3d4e5f6-a1b2-7890-cdef-3456789012de",
//	      "status": {
//	        "code": "ACTIVE"
//	      },
//	      "alias": "@treasury_checking",
//	      "type": "deposit",
//	      "createdAt": "2022-04-15T09:30:00Z",
//	      "updatedAt": "2022-04-15T09:30:00Z"
//	    },
//	    {
//	      "id": "f6a1b2c3-d4e5-7890-fghi-6789012345gh",
//	      "name": "Operating Expenses",
//	      "assetCode": "USD",
//	      "ledgerId": "c3d4e5f6-a1b2-7890-cdef-3456789012de",
//	      "status": {
//	        "code": "ACTIVE"
//	      },
//	      "alias": "@operating_expenses",
//	      "type": "expense",
//	      "createdAt": "2022-04-16T10:15:00Z",
//	      "updatedAt": "2022-04-16T10:15:00Z"
//	    }
//	  ],
//	  "page": 1,
//	  "limit": 10
//	}
type Accounts struct {
	// Array of account records returned in this page
	// example: [{"id":"00000000-0000-0000-0000-000000000000","name":"Corporate Checking Account","assetCode":"USD","status":{"code": "ACTIVE"}}]
	Items []Account `json:"items"`

	// Current page number in the pagination
	// example: 1
	// minimum: 1
	Page int `json:"page" example:"1" minimum:"1"`

	// Maximum number of items per page
	// example: 10
	// minimum: 1
	// maximum: 100
	Limit int `json:"limit" example:"10" minimum:"1" maximum:"100"`
} //	@name	Accounts

// AccountResponse represents a success response containing a single account.
//
// swagger:response AccountResponse
//
//	@Description	Successful response containing a single account entity.
type AccountResponse struct {
	// in: body
	Body Account
}

// AccountsResponse represents a success response containing a paginated list of accounts.
//
// swagger:response AccountsResponse
//
//	@Description	Successful response containing a paginated list of accounts.
type AccountsResponse struct {
	// in: body
	Body Accounts
}

// AccountErrorResponse represents an error response for account operations.
//
// swagger:response AccountErrorResponse
//
//	@Description	Error response for account operations with error code and message.
//
//	@example		{
//	  "code": 400001,
//	  "message": "Invalid input: field 'assetCode' is required",
//	  "details": {
//	    "field": "assetCode",
//	    "violation": "required"
//	  }
//	}
type AccountErrorResponse struct {
	// in: body
	Body struct {
		// Error code identifying the specific error
		// example: 400001
		Code int `json:"code"`

		// Human-readable error message
		// example: Invalid input: field 'assetCode' is required
		Message string `json:"message"`

		// Additional error details if available
		// example: {"field": "assetCode", "violation": "required"}
		Details map[string]any `json:"details,omitempty"`
	}
}

// CreateAccountBatchRequest represents a batch account creation request.
//
// swagger:model CreateAccountBatchRequest
//
//	@Description	Request payload for creating multiple accounts in a single batch operation.
//	@Description	Supports atomic mode (all-or-nothing) or partial mode (independent operations).
//
//	@example		{
//	  "atomic": false,
//	  "accounts": [
//	    {
//	      "id": "req_1",
//	      "name": "Checking Account",
//	      "assetCode": "USD",
//	      "type": "deposit",
//	      "alias": "@checking_001"
//	    },
//	    {
//	      "id": "req_2",
//	      "name": "Savings Account",
//	      "assetCode": "USD",
//	      "type": "savings",
//	      "alias": "@savings_001"
//	    }
//	  ]
//	}
type CreateAccountBatchRequest struct {
	// When true, all accounts must succeed or all fail (transactional).
	// When false, each account is processed independently.
	// required: false
	// default: false
	Atomic bool `json:"atomic"`

	// Array of account creation items to process
	// required: true
	// minItems: 1
	// maxItems: 100
	Accounts []CreateAccountBatchItem `json:"accounts" validate:"required,min=1,max=100,dive"`
} //	@name	CreateAccountBatchRequest

// CreateAccountBatchItem wraps CreateAccountInput with a client-generated ID for request/response correlation.
//
// swagger:model CreateAccountBatchItem
//
//	@Description	A single account creation item within a batch request, including a client ID for tracking.
type CreateAccountBatchItem struct {
	// Client-generated ID for correlating requests with responses
	// required: true
	// example: req_1
	// maxLength: 100
	ID string `json:"id" validate:"required,max=100" example:"req_1" maxLength:"100"`

	// Human-readable name of the account
	// required: false
	// example: Corporate Checking Account
	// maxLength: 256
	Name string `json:"name" validate:"max=256" example:"Corporate Checking Account" maxLength:"256"`

	// ID of the parent account if this is a subaccount (optional)
	// required: false
	// format: uuid
	ParentAccountID *string `json:"parentAccountId" validate:"omitempty,uuid" format:"uuid"`

	// Optional external identifier for linking to external systems
	// required: false
	// example: EXT-ACC-12345
	// maxLength: 256
	EntityID *string `json:"entityId" validate:"omitempty,max=256" example:"EXT-ACC-12345" maxLength:"256"`

	// Asset code that this account will use for balances and transactions
	// required: true
	// example: USD
	// maxLength: 100
	AssetCode string `json:"assetCode" validate:"required,max=100" example:"USD" maxLength:"100"`

	// ID of the portfolio this account belongs to (optional)
	// required: false
	// format: uuid
	PortfolioID *string `json:"portfolioId" validate:"omitempty,uuid" format:"uuid"`

	// ID of the segment this account belongs to (optional)
	// required: false
	// format: uuid
	SegmentID *string `json:"segmentId" validate:"omitempty,uuid" format:"uuid"`

	// Current operating status of the account
	// required: false
	Status Status `json:"status"`

	// Unique alias for the account (optional, must follow alias format rules)
	// required: false
	// example: @treasury_checking
	// maxLength: 100
	Alias *string `json:"alias" validate:"omitempty,max=100,prohibitedexternalaccountprefix,invalidaliascharacters" example:"@treasury_checking" maxLength:"100"`

	// Type of the account
	// required: true
	// example: deposit
	// maxLength: 256
	Type string `json:"type" validate:"required,max=256,invalidstrings=external" example:"deposit"`

	// Whether the account should start blocked
	// required: false
	// default: false
	Blocked *bool `json:"blocked"`

	// Custom key-value pairs for extending the account information
	// required: false
	// example: {"department": "Treasury", "purpose": "Operating Expenses", "region": "Global"}
	Metadata map[string]any `json:"metadata" validate:"dive,keys,keymax=100,endkeys,nonested,valuemax=2000"`
} //	@name	CreateAccountBatchItem

// ToCreateAccountInput converts a CreateAccountBatchItem to CreateAccountInput.
func (item *CreateAccountBatchItem) ToCreateAccountInput() *CreateAccountInput {
	return &CreateAccountInput{
		Name:            item.Name,
		ParentAccountID: item.ParentAccountID,
		EntityID:        item.EntityID,
		AssetCode:       item.AssetCode,
		PortfolioID:     item.PortfolioID,
		SegmentID:       item.SegmentID,
		Status:          item.Status,
		Alias:           item.Alias,
		Type:            item.Type,
		Blocked:         item.Blocked,
		Metadata:        item.Metadata,
	}
}

// CreateAccountBatchResponse represents the batch account creation response.
//
// swagger:model CreateAccountBatchResponse
//
//	@Description	Response payload for batch account creation, containing results for each item.
//
//	@example		{
//	  "successCount": 2,
//	  "failureCount": 0,
//	  "results": [
//	    {
//	      "id": "req_1",
//	      "status": 201,
//	      "account": {"id": "...", "name": "Checking Account", ...}
//	    },
//	    {
//	      "id": "req_2",
//	      "status": 201,
//	      "account": {"id": "...", "name": "Savings Account", ...}
//	    }
//	  ]
//	}
type CreateAccountBatchResponse struct {
	// Number of accounts successfully created
	// example: 2
	SuccessCount int `json:"successCount" example:"2"`

	// Number of accounts that failed to create
	// example: 0
	FailureCount int `json:"failureCount" example:"0"`

	// Array of results for each account in the batch
	Results []CreateAccountBatchResult `json:"results"`
} //	@name	CreateAccountBatchResponse

// CreateAccountBatchResult represents the result of a single account creation within a batch.
//
// swagger:model CreateAccountBatchResult
//
//	@Description	Result for a single account creation within a batch operation.
type CreateAccountBatchResult struct {
	// Client-generated ID from the request for correlation
	// example: req_1
	ID string `json:"id" example:"req_1"`

	// HTTP status code for this specific operation (201 for success, 4xx/5xx for errors)
	// example: 201
	Status int `json:"status" example:"201"`

	// The created account (present only on success)
	Account *Account `json:"account,omitempty"`

	// Error details (present only on failure)
	Error *BatchItemError `json:"error,omitempty"`
} //	@name	CreateAccountBatchResult

// BatchItemError represents an error for a single item in a batch operation.
//
// swagger:model BatchItemError
//
//	@Description	Error details for a failed item in a batch operation.
type BatchItemError struct {
	// Error code identifying the specific error
	// example: ALIAS_UNAVAILABLE
	Code string `json:"code" example:"ALIAS_UNAVAILABLE"`

	// Human-readable error message
	// example: Alias @savings_001 is already taken
	Message string `json:"message" example:"Alias @savings_001 is already taken"`
} //	@name	BatchItemError
