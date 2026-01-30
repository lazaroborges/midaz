package command

import (
	"context"
	"net/http"
	"reflect"
	"time"

	libCommons "github.com/LerianStudio/lib-commons/v2/commons"
	libConstant "github.com/LerianStudio/lib-commons/v2/commons/constants"
	libOpentelemetry "github.com/LerianStudio/lib-commons/v2/commons/opentelemetry"
	"github.com/LerianStudio/midaz/v3/pkg"
	"github.com/LerianStudio/midaz/v3/pkg/constant"
	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
)

// CreateAccountsBatch creates multiple accounts in a single batch operation.
// Supports two modes:
//   - Atomic (atomic=true): All accounts must succeed or all fail (transactional)
//   - Partial (atomic=false): Each account is processed independently
//
// The balance for each account is created via the BalancePort interface.
func (uc *UseCase) CreateAccountsBatch(ctx context.Context, organizationID, ledgerID uuid.UUID, req *mmodel.CreateAccountBatchRequest, token string) (*mmodel.CreateAccountBatchResponse, error) {
	logger, tracer, _, _ := libCommons.NewTrackingFromContext(ctx)

	ctx, span := tracer.Start(ctx, "command.create_accounts_batch")
	defer span.End()

	logger.Infof("Starting batch account creation: %d accounts, atomic=%v", len(req.Accounts), req.Atomic)

	// Fail-fast: Check balance service health before proceeding
	if err := uc.BalancePort.CheckHealth(ctx); err != nil {
		libOpentelemetry.HandleSpanBusinessErrorEvent(&span, "Balance service health check failed", err)
		logger.Errorf("Balance service is unavailable: %v", err)

		return nil, pkg.ValidateBusinessError(constant.ErrGRPCServiceUnavailable, reflect.TypeOf(mmodel.Account{}).Name())
	}

	// Pre-validation phase: validate all items before any creation
	validationErrors := uc.preValidateBatch(ctx, organizationID, ledgerID, req.Accounts, req.Atomic)

	// If atomic mode and any validation errors, fail immediately
	if req.Atomic && len(validationErrors) > 0 {
		libOpentelemetry.HandleSpanBusinessErrorEvent(&span, "Batch validation failed in atomic mode", nil)
		logger.Warnf("Batch validation failed in atomic mode: %d errors", len(validationErrors))

		return buildValidationErrorResponse(req.Accounts, validationErrors), nil
	}

	// Execute based on mode
	if req.Atomic {
		return uc.createAccountsBatchAtomic(ctx, organizationID, ledgerID, req.Accounts, token)
	}

	return uc.createAccountsBatchPartial(ctx, organizationID, ledgerID, req.Accounts, validationErrors, token)
}

// preValidateBatch performs validation on all batch items before creation.
// Returns a map of item ID to error for items that failed validation.
// The atomic parameter controls error handling: in atomic mode, infrastructure errors
// (like failing to check aliases) will mark all affected items as failed to prevent
// inconsistent state.
func (uc *UseCase) preValidateBatch(ctx context.Context, organizationID, ledgerID uuid.UUID, items []mmodel.CreateAccountBatchItem, atomic bool) map[string]*mmodel.BatchItemError {
	logger, tracer, _, _ := libCommons.NewTrackingFromContext(ctx)

	ctx, span := tracer.Start(ctx, "command.pre_validate_batch")
	defer span.End()

	errors := make(map[string]*mmodel.BatchItemError)

	// Collect all aliases and asset codes
	_, aliasesToCheck, assetCodes := uc.collectValidationData(items, errors)

	// Validate aliases
	if shouldAbort := uc.validateAliases(ctx, organizationID, ledgerID, items, aliasesToCheck, errors, atomic); shouldAbort {
		return errors
	}

	// Validate asset codes
	uc.validateAssetCodes(ctx, organizationID, ledgerID, items, assetCodes, errors)

	// Validate account types
	uc.validateAccountTypes(ctx, organizationID, ledgerID, items, errors)

	logger.Infof("Pre-validation complete: %d errors found out of %d items", len(errors), len(items))

	return errors
}

// collectValidationData collects aliases and asset codes from batch items and checks for duplicate aliases within the batch.
func (uc *UseCase) collectValidationData(items []mmodel.CreateAccountBatchItem, errors map[string]*mmodel.BatchItemError) (map[string]string, []string, map[string]bool) {
	aliasMap := make(map[string]string) // alias -> item ID (for duplicate detection within batch)
	aliasesToCheck := make([]string, 0) // aliases to check against database
	assetCodes := make(map[string]bool) // unique asset codes to validate

	for _, item := range items {
		// Check for duplicate aliases within the batch
		if item.Alias != nil && *item.Alias != "" {
			if existingID, exists := aliasMap[*item.Alias]; exists {
				errors[item.ID] = &mmodel.BatchItemError{
					Code:    "DUPLICATE_ALIAS_IN_BATCH",
					Message: "Alias '" + *item.Alias + "' is duplicated in the batch (first in item: " + existingID + ")",
				}

				continue
			}

			aliasMap[*item.Alias] = item.ID
			aliasesToCheck = append(aliasesToCheck, *item.Alias)
		}

		// Collect unique asset codes
		assetCodes[item.AssetCode] = true
	}

	return aliasMap, aliasesToCheck, assetCodes
}

// validateAliases checks aliases against the database and returns true if validation should abort (atomic mode).
func (uc *UseCase) validateAliases(ctx context.Context, organizationID, ledgerID uuid.UUID, items []mmodel.CreateAccountBatchItem, aliasesToCheck []string, errors map[string]*mmodel.BatchItemError, atomic bool) bool {
	logger, _tracer, _requestID, _metricFactory := libCommons.NewTrackingFromContext(ctx)
	_ = _tracer
	_ = _requestID
	_ = _metricFactory

	if len(aliasesToCheck) == 0 {
		return false
	}

	existingAliases, err := uc.AccountRepo.FindByAliases(ctx, organizationID, ledgerID, aliasesToCheck)
	if err != nil {
		logger.Errorf("Failed to check aliases: %v", err)

		// Cannot proceed safely without alias verification
		// Mark all items with aliases as failed
		for _, item := range items {
			if item.Alias != nil && *item.Alias != "" {
				if _, hasError := errors[item.ID]; !hasError {
					errors[item.ID] = &mmodel.BatchItemError{
						Code:    "ALIAS_CHECK_FAILED",
						Message: "Unable to verify alias availability: " + err.Error(),
					}
				}
			}
		}

		if atomic {
			// In atomic mode, return immediately - this will abort the entire batch
			// to prevent potential duplicate alias creation
			logger.Warnf("Aborting atomic batch due to alias check failure")
			return true
		}

		// In partial mode, continue with other validations
		return false
	}

	// Mark items with existing aliases as errors
	existingAliasSet := make(map[string]bool)
	for _, alias := range existingAliases {
		existingAliasSet[alias] = true
	}

	for _, item := range items {
		if item.Alias != nil && existingAliasSet[*item.Alias] {
			errors[item.ID] = &mmodel.BatchItemError{
				Code:    "ALIAS_UNAVAILABLE",
				Message: "Alias '" + *item.Alias + "' is already taken",
			}
		}
	}

	return false
}

// validateAssetCodes validates that all asset codes exist in the database.
func (uc *UseCase) validateAssetCodes(ctx context.Context, organizationID, ledgerID uuid.UUID, items []mmodel.CreateAccountBatchItem, assetCodes map[string]bool, errors map[string]*mmodel.BatchItemError) {
	for assetCode := range assetCodes {
		isAsset, _ := uc.AssetRepo.FindByNameOrCode(ctx, organizationID, ledgerID, "", assetCode)
		if !isAsset {
			// Mark all items with this asset code as errors
			for _, item := range items {
				if item.AssetCode == assetCode {
					if _, hasError := errors[item.ID]; !hasError {
						errors[item.ID] = &mmodel.BatchItemError{
							Code:    "ASSET_CODE_NOT_FOUND",
							Message: "Asset code '" + assetCode + "' not found",
						}
					}
				}
			}
		}
	}
}

// validateAccountTypes validates account types for each item.
func (uc *UseCase) validateAccountTypes(ctx context.Context, organizationID, ledgerID uuid.UUID, items []mmodel.CreateAccountBatchItem, errors map[string]*mmodel.BatchItemError) {
	for _, item := range items {
		if _, hasError := errors[item.ID]; hasError {
			continue
		}

		if err := uc.applyAccountingValidations(ctx, organizationID, ledgerID, item.Type); err != nil {
			errors[item.ID] = &mmodel.BatchItemError{
				Code:    "INVALID_ACCOUNT_TYPE",
				Message: "Invalid account type: " + item.Type,
			}
		}
	}
}

// createAccountsBatchAtomic creates all accounts in a single transaction.
// If any account fails, all accounts are rolled back.
func (uc *UseCase) createAccountsBatchAtomic(ctx context.Context, organizationID, ledgerID uuid.UUID, items []mmodel.CreateAccountBatchItem, token string) (*mmodel.CreateAccountBatchResponse, error) {
	logger, tracer, requestID, _ := libCommons.NewTrackingFromContext(ctx)

	ctx, span := tracer.Start(ctx, "command.create_accounts_batch_atomic")
	defer span.End()

	logger.Infof("Creating %d accounts in atomic mode", len(items))

	// Prepare all account entities
	accounts := make([]*mmodel.Account, 0, len(items))
	itemMap := make(map[string]*mmodel.CreateAccountBatchItem) // account ID -> original item

	for i := range items {
		item := &items[i]
		input := item.ToCreateAccountInput()

		// Determine name if not provided
		if libCommons.IsNilOrEmpty(&input.Name) {
			input.Name = input.AssetCode + " " + input.Type + " account"
		}

		// Determine status
		status := uc.determineStatusFromBatchItem(item)

		// Generate ID
		accountID := libCommons.GenerateUUIDv7().String()

		// Resolve alias (use ID if not provided)
		var alias *string
		if !libCommons.IsNilOrEmpty(item.Alias) {
			alias = item.Alias
		} else {
			alias = &accountID
		}

		blocked := item.Blocked != nil && *item.Blocked

		account := &mmodel.Account{
			ID:              accountID,
			AssetCode:       item.AssetCode,
			Alias:           alias,
			Name:            input.Name,
			Type:            item.Type,
			Blocked:         &blocked,
			ParentAccountID: item.ParentAccountID,
			SegmentID:       item.SegmentID,
			OrganizationID:  organizationID.String(),
			PortfolioID:     item.PortfolioID,
			LedgerID:        ledgerID.String(),
			EntityID:        item.EntityID,
			Status:          status,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}

		accounts = append(accounts, account)
		itemMap[accountID] = item
	}

	// Create all accounts in batch
	createdAccounts, err := uc.AccountRepo.CreateBatch(ctx, accounts)
	if err != nil {
		libOpentelemetry.HandleSpanBusinessErrorEvent(&span, "Failed to create accounts batch", err)
		logger.Errorf("Failed to create accounts batch: %v", err)

		// Return all items as failed
		return buildAllFailedResponse(items, "ACCOUNT_CREATION_FAILED", "Failed to create accounts: "+err.Error()), nil
	}

	// Create balances for all accounts
	ctx = metadata.AppendToOutgoingContext(ctx, libConstant.MetadataAuthorization, token)

	// Track created accounts and accounts with successfully created balances for rollback
	createdAccountIDs := make([]uuid.UUID, 0, len(createdAccounts))
	balanceCreatedForAccountIDs := make([]uuid.UUID, 0, len(createdAccounts))

	for _, acc := range createdAccounts {
		createdAccountIDs = append(createdAccountIDs, uuid.MustParse(acc.ID))
	}

	for _, acc := range createdAccounts {
		item := itemMap[acc.ID]

		balanceInput := mmodel.CreateBalanceInput{
			RequestID:      requestID,
			OrganizationID: organizationID,
			LedgerID:       ledgerID,
			AccountID:      uuid.MustParse(acc.ID),
			Alias:          *acc.Alias,
			Key:            constant.DefaultBalanceKey,
			AssetCode:      item.AssetCode,
			AccountType:    item.Type,
			AllowSending:   true,
			AllowReceiving: true,
		}

		_, err = uc.BalancePort.CreateBalanceSync(ctx, balanceInput)
		if err != nil {
			libOpentelemetry.HandleSpanBusinessErrorEvent(&span, "Failed to create balance for account", err)
			logger.Errorf("Failed to create balance for account %s: %v", acc.ID, err)

			// Rollback: first delete balances created for previous accounts (1..N-1)
			uc.rollbackBalances(ctx, organizationID, ledgerID, balanceCreatedForAccountIDs, requestID)

			// Then delete all created accounts
			uc.rollbackAccounts(ctx, organizationID, ledgerID, createdAccountIDs)

			if isAuthorizationError(err) {
				return nil, err
			}

			return buildAllFailedResponse(items, "BALANCE_CREATION_FAILED", "Failed to create balance: "+err.Error()), nil
		}

		// Track successful balance creation for potential rollback
		balanceCreatedForAccountIDs = append(balanceCreatedForAccountIDs, uuid.MustParse(acc.ID))
	}

	// Create metadata for all accounts
	for _, acc := range createdAccounts {
		item := itemMap[acc.ID]

		if len(item.Metadata) > 0 {
			metadataDoc, err := uc.CreateMetadata(ctx, reflect.TypeOf(mmodel.Account{}).Name(), acc.ID, item.Metadata)
			if err != nil {
				logger.Warnf("Failed to create metadata for account %s: %v", acc.ID, err)
				// Don't fail the whole batch for metadata errors in atomic mode
				// The accounts and balances are created successfully
			} else {
				acc.Metadata = metadataDoc
			}
		}
	}

	// Build success response
	results := make([]mmodel.CreateAccountBatchResult, 0, len(createdAccounts))
	for _, acc := range createdAccounts {
		item := itemMap[acc.ID]
		results = append(results, mmodel.CreateAccountBatchResult{
			ID:      item.ID,
			Status:  http.StatusCreated,
			Account: acc,
		})
	}

	logger.Infof("Successfully created %d accounts in atomic mode", len(createdAccounts))

	return &mmodel.CreateAccountBatchResponse{
		SuccessCount: len(createdAccounts),
		FailureCount: 0,
		Results:      results,
	}, nil
}

// createAccountsBatchPartial creates accounts independently, allowing partial success.
func (uc *UseCase) createAccountsBatchPartial(ctx context.Context, organizationID, ledgerID uuid.UUID, items []mmodel.CreateAccountBatchItem, validationErrors map[string]*mmodel.BatchItemError, token string) (*mmodel.CreateAccountBatchResponse, error) {
	logger, tracer, _, _ := libCommons.NewTrackingFromContext(ctx)

	ctx, span := tracer.Start(ctx, "command.create_accounts_batch_partial")
	defer span.End()

	logger.Infof("Creating %d accounts in partial mode (pre-validation errors: %d)", len(items), len(validationErrors))

	results := make([]mmodel.CreateAccountBatchResult, 0, len(items))
	successCount := 0
	failureCount := 0

	for i := range items {
		item := &items[i]

		// Skip items that failed pre-validation
		if validationErr, hasError := validationErrors[item.ID]; hasError {
			results = append(results, mmodel.CreateAccountBatchResult{
				ID:     item.ID,
				Status: http.StatusConflict,
				Error:  validationErr,
			})
			failureCount++

			continue
		}

		// Create individual account using existing single-account logic
		input := item.ToCreateAccountInput()

		account, err := uc.CreateAccount(ctx, organizationID, ledgerID, input, token)
		if err != nil {
			logger.Warnf("Failed to create account for item %s: %v", item.ID, err)
			results = append(results, mmodel.CreateAccountBatchResult{
				ID:     item.ID,
				Status: http.StatusInternalServerError,
				Error: &mmodel.BatchItemError{
					Code:    "ACCOUNT_CREATION_FAILED",
					Message: err.Error(),
				},
			})
			failureCount++

			continue
		}

		results = append(results, mmodel.CreateAccountBatchResult{
			ID:      item.ID,
			Status:  http.StatusCreated,
			Account: account,
		})
		successCount++
	}

	logger.Infof("Partial batch complete: %d succeeded, %d failed", successCount, failureCount)

	return &mmodel.CreateAccountBatchResponse{
		SuccessCount: successCount,
		FailureCount: failureCount,
		Results:      results,
	}, nil
}

// rollbackAccounts deletes accounts that were created during a failed atomic batch operation.
func (uc *UseCase) rollbackAccounts(ctx context.Context, organizationID, ledgerID uuid.UUID, accountIDs []uuid.UUID) {
	logger, tracer, _, _ := libCommons.NewTrackingFromContext(ctx)

	ctx, span := tracer.Start(ctx, "command.rollback_accounts")
	defer span.End()

	logger.Infof("Rolling back %d accounts", len(accountIDs))

	for _, accountID := range accountIDs {
		err := uc.AccountRepo.Delete(ctx, organizationID, ledgerID, nil, accountID)
		if err != nil {
			logger.Errorf("Failed to rollback account %s: %v", accountID.String(), err)
		}
	}
}

// rollbackBalances deletes balances that were created during a failed atomic batch operation.
// This is called before rollbackAccounts to ensure balances are cleaned up first.
func (uc *UseCase) rollbackBalances(ctx context.Context, organizationID, ledgerID uuid.UUID, accountIDs []uuid.UUID, requestID string) {
	logger, tracer, _, _ := libCommons.NewTrackingFromContext(ctx)

	ctx, span := tracer.Start(ctx, "command.rollback_balances")
	defer span.End()

	if len(accountIDs) == 0 {
		return
	}

	logger.Infof("Rolling back balances for %d accounts", len(accountIDs))

	for _, accountID := range accountIDs {
		err := uc.BalancePort.DeleteAllBalancesByAccountID(ctx, organizationID, ledgerID, accountID, requestID)
		if err != nil {
			// Log error but continue - best effort rollback to clean up as much as possible
			logger.Errorf("Failed to rollback balances for account %s: %v", accountID.String(), err)
		}
	}
}

// determineStatusFromBatchItem determines the status from a batch item.
func (uc *UseCase) determineStatusFromBatchItem(item *mmodel.CreateAccountBatchItem) mmodel.Status {
	var status mmodel.Status
	if item.Status.IsEmpty() || libCommons.IsNilOrEmpty(&item.Status.Code) {
		status = mmodel.Status{
			Code: "ACTIVE",
		}
	} else {
		status = item.Status
	}

	status.Description = item.Status.Description

	return status
}

// buildValidationErrorResponse builds a response when pre-validation fails in atomic mode.
func buildValidationErrorResponse(items []mmodel.CreateAccountBatchItem, errors map[string]*mmodel.BatchItemError) *mmodel.CreateAccountBatchResponse {
	results := make([]mmodel.CreateAccountBatchResult, 0, len(items))

	for _, item := range items {
		if err, hasError := errors[item.ID]; hasError {
			results = append(results, mmodel.CreateAccountBatchResult{
				ID:     item.ID,
				Status: http.StatusConflict,
				Error:  err,
			})
		} else {
			// Items without errors also fail due to atomic mode
			results = append(results, mmodel.CreateAccountBatchResult{
				ID:     item.ID,
				Status: http.StatusConflict,
				Error: &mmodel.BatchItemError{
					Code:    "ATOMIC_BATCH_FAILED",
					Message: "Batch operation failed due to other items in the batch",
				},
			})
		}
	}

	return &mmodel.CreateAccountBatchResponse{
		SuccessCount: 0,
		FailureCount: len(items),
		Results:      results,
	}
}

// buildAllFailedResponse builds a response when all items fail with the same error.
func buildAllFailedResponse(items []mmodel.CreateAccountBatchItem, code, message string) *mmodel.CreateAccountBatchResponse {
	results := make([]mmodel.CreateAccountBatchResult, 0, len(items))

	for _, item := range items {
		results = append(results, mmodel.CreateAccountBatchResult{
			ID:     item.ID,
			Status: http.StatusInternalServerError,
			Error: &mmodel.BatchItemError{
				Code:    code,
				Message: message,
			},
		})
	}

	return &mmodel.CreateAccountBatchResponse{
		SuccessCount: 0,
		FailureCount: len(items),
		Results:      results,
	}
}
