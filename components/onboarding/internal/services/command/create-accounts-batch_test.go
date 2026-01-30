package command

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/LerianStudio/midaz/v3/components/onboarding/internal/adapters/mongodb"
	"github.com/LerianStudio/midaz/v3/components/onboarding/internal/adapters/postgres/account"
	"github.com/LerianStudio/midaz/v3/components/onboarding/internal/adapters/postgres/accounttype"
	"github.com/LerianStudio/midaz/v3/components/onboarding/internal/adapters/postgres/asset"
	"github.com/LerianStudio/midaz/v3/pkg/mbootstrap"
	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// setupBatchTest creates a new UseCase with mocks for batch testing
func setupBatchTest(ctrl *gomock.Controller) (*UseCase, *asset.MockRepository, *account.MockRepository, *mongodb.MockRepository, *accounttype.MockRepository, *mbootstrap.MockBalancePort) {
	mockAssetRepo := asset.NewMockRepository(ctrl)
	mockAccountRepo := account.NewMockRepository(ctrl)
	mockMetadataRepo := mongodb.NewMockRepository(ctrl)
	mockAccountTypeRepo := accounttype.NewMockRepository(ctrl)
	mockBalanceGRPC := mbootstrap.NewMockBalancePort(ctrl)

	uc := &UseCase{
		AssetRepo:       mockAssetRepo,
		AccountRepo:     mockAccountRepo,
		MetadataRepo:    mockMetadataRepo,
		AccountTypeRepo: mockAccountTypeRepo,
		BalancePort:     mockBalanceGRPC,
	}

	return uc, mockAssetRepo, mockAccountRepo, mockMetadataRepo, mockAccountTypeRepo, mockBalanceGRPC
}

// TestCreateAccountsBatch_PartialMode_AllSuccess tests partial mode with all accounts succeeding
func TestCreateAccountsBatch_PartialMode_AllSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, mockAssetRepo, mockAccountRepo, mockMetadataRepo, _, mockBalance := setupBatchTest(ctrl)

	ctx := context.Background()
	organizationID := uuid.New()
	ledgerID := uuid.New()

	req := &mmodel.CreateAccountBatchRequest{
		Atomic: false,
		Accounts: []mmodel.CreateAccountBatchItem{
			{
				ID:        "req_1",
				Name:      "Account 1",
				Type:      "deposit",
				AssetCode: "USD",
			},
			{
				ID:        "req_2",
				Name:      "Account 2",
				Type:      "savings",
				AssetCode: "USD",
			},
		},
	}

	// Mock health check
	mockBalance.EXPECT().
		CheckHealth(gomock.Any()).
		Return(nil).Times(3) // Once for batch, twice for individual creates

	// Mock asset validation
	mockAssetRepo.EXPECT().
		FindByNameOrCode(gomock.Any(), organizationID, ledgerID, "", "USD").
		Return(true, nil).Times(3) // Once for batch validation, twice for individual creates

	// Mock FindByAlias for individual account creation
	mockAccountRepo.EXPECT().
		FindByAlias(gomock.Any(), organizationID, ledgerID, gomock.Any()).
		Return(false, nil).AnyTimes()

	// Mock account creation - called twice (partial mode uses CreateAccount which calls Create)
	mockAccountRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, acc *mmodel.Account) (*mmodel.Account, error) {
			return acc, nil
		}).Times(2)

	// Mock balance creation
	mockBalance.EXPECT().
		CreateBalanceSync(gomock.Any(), gomock.Any()).
		Return(nil, nil).Times(2)

	// Mock metadata creation
	mockMetadataRepo.EXPECT().
		Create(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()

	response, err := uc.CreateAccountsBatch(ctx, organizationID, ledgerID, req, "test-token")

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, 2, response.SuccessCount)
	assert.Equal(t, 0, response.FailureCount)
	assert.Len(t, response.Results, 2)

	for _, result := range response.Results {
		assert.Equal(t, http.StatusCreated, result.Status)
		assert.NotNil(t, result.Account)
		assert.Nil(t, result.Error)
	}
}

// TestCreateAccountsBatch_PartialMode_PartialSuccess tests partial mode with some failures
func TestCreateAccountsBatch_PartialMode_PartialSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, mockAssetRepo, mockAccountRepo, mockMetadataRepo, _, mockBalance := setupBatchTest(ctrl)

	ctx := context.Background()
	organizationID := uuid.New()
	ledgerID := uuid.New()

	existingAlias := "@existing_alias"
	req := &mmodel.CreateAccountBatchRequest{
		Atomic: false,
		Accounts: []mmodel.CreateAccountBatchItem{
			{
				ID:        "req_1",
				Name:      "Account 1",
				Type:      "deposit",
				AssetCode: "USD",
			},
			{
				ID:        "req_2",
				Name:      "Account 2",
				Type:      "savings",
				AssetCode: "USD",
				Alias:     &existingAlias, // This alias already exists
			},
		},
	}

	// Mock health check
	mockBalance.EXPECT().
		CheckHealth(gomock.Any()).
		Return(nil).Times(2) // Once for batch, once for successful account

	// Mock FindByAliases - return existing alias
	mockAccountRepo.EXPECT().
		FindByAliases(gomock.Any(), organizationID, ledgerID, []string{existingAlias}).
		Return([]string{existingAlias}, nil).Times(1)

	// Mock asset validation
	mockAssetRepo.EXPECT().
		FindByNameOrCode(gomock.Any(), organizationID, ledgerID, "", "USD").
		Return(true, nil).Times(2) // Once for batch validation, once for successful account

	// Mock FindByAlias for successful account
	mockAccountRepo.EXPECT().
		FindByAlias(gomock.Any(), organizationID, ledgerID, gomock.Any()).
		Return(false, nil).AnyTimes()

	// Mock account creation - only one succeeds
	mockAccountRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, acc *mmodel.Account) (*mmodel.Account, error) {
			return acc, nil
		}).Times(1)

	// Mock balance creation for successful account
	mockBalance.EXPECT().
		CreateBalanceSync(gomock.Any(), gomock.Any()).
		Return(nil, nil).Times(1)

	// Mock metadata creation
	mockMetadataRepo.EXPECT().
		Create(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()

	response, err := uc.CreateAccountsBatch(ctx, organizationID, ledgerID, req, "test-token")

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, 1, response.SuccessCount)
	assert.Equal(t, 1, response.FailureCount)
	assert.Len(t, response.Results, 2)

	// Check first result (should succeed)
	assert.Equal(t, "req_1", response.Results[0].ID)
	assert.Equal(t, http.StatusCreated, response.Results[0].Status)
	assert.NotNil(t, response.Results[0].Account)

	// Check second result (should fail due to alias conflict)
	assert.Equal(t, "req_2", response.Results[1].ID)
	assert.Equal(t, http.StatusConflict, response.Results[1].Status)
	assert.NotNil(t, response.Results[1].Error)
	assert.Equal(t, "ALIAS_UNAVAILABLE", response.Results[1].Error.Code)
}

// TestCreateAccountsBatch_AtomicMode_AllSuccess tests atomic mode with all accounts succeeding
func TestCreateAccountsBatch_AtomicMode_AllSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, mockAssetRepo, mockAccountRepo, mockMetadataRepo, _, mockBalance := setupBatchTest(ctrl)

	ctx := context.Background()
	organizationID := uuid.New()
	ledgerID := uuid.New()

	req := &mmodel.CreateAccountBatchRequest{
		Atomic: true,
		Accounts: []mmodel.CreateAccountBatchItem{
			{
				ID:        "req_1",
				Name:      "Account 1",
				Type:      "deposit",
				AssetCode: "USD",
			},
			{
				ID:        "req_2",
				Name:      "Account 2",
				Type:      "savings",
				AssetCode: "USD",
			},
		},
	}

	// Mock health check
	mockBalance.EXPECT().
		CheckHealth(gomock.Any()).
		Return(nil).Times(1)

	// Mock asset validation
	mockAssetRepo.EXPECT().
		FindByNameOrCode(gomock.Any(), organizationID, ledgerID, "", "USD").
		Return(true, nil).Times(1)

	// Mock batch account creation
	mockAccountRepo.EXPECT().
		CreateBatch(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, accounts []*mmodel.Account) ([]*mmodel.Account, error) {
			return accounts, nil
		}).Times(1)

	// Mock balance creation for each account
	mockBalance.EXPECT().
		CreateBalanceSync(gomock.Any(), gomock.Any()).
		Return(nil, nil).Times(2)

	// Mock metadata creation
	mockMetadataRepo.EXPECT().
		Create(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()

	response, err := uc.CreateAccountsBatch(ctx, organizationID, ledgerID, req, "test-token")

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, 2, response.SuccessCount)
	assert.Equal(t, 0, response.FailureCount)
	assert.Len(t, response.Results, 2)

	for _, result := range response.Results {
		assert.Equal(t, http.StatusCreated, result.Status)
		assert.NotNil(t, result.Account)
		assert.Nil(t, result.Error)
	}
}

// TestCreateAccountsBatch_AtomicMode_ValidationFails tests atomic mode when validation fails
func TestCreateAccountsBatch_AtomicMode_ValidationFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, mockAssetRepo, mockAccountRepo, _, _, mockBalance := setupBatchTest(ctrl)

	ctx := context.Background()
	organizationID := uuid.New()
	ledgerID := uuid.New()

	existingAlias := "@existing_alias"
	req := &mmodel.CreateAccountBatchRequest{
		Atomic: true,
		Accounts: []mmodel.CreateAccountBatchItem{
			{
				ID:        "req_1",
				Name:      "Account 1",
				Type:      "deposit",
				AssetCode: "USD",
			},
			{
				ID:        "req_2",
				Name:      "Account 2",
				Type:      "savings",
				AssetCode: "USD",
				Alias:     &existingAlias, // This alias already exists
			},
		},
	}

	// Mock health check
	mockBalance.EXPECT().
		CheckHealth(gomock.Any()).
		Return(nil).Times(1)

	// Mock FindByAliases - return existing alias
	mockAccountRepo.EXPECT().
		FindByAliases(gomock.Any(), organizationID, ledgerID, []string{existingAlias}).
		Return([]string{existingAlias}, nil).Times(1)

	// Mock asset validation
	mockAssetRepo.EXPECT().
		FindByNameOrCode(gomock.Any(), organizationID, ledgerID, "", "USD").
		Return(true, nil).Times(1)

	// In atomic mode, no accounts should be created when validation fails
	// CreateBatch should NOT be called

	response, err := uc.CreateAccountsBatch(ctx, organizationID, ledgerID, req, "test-token")

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, 0, response.SuccessCount)
	assert.Equal(t, 2, response.FailureCount) // Both fail in atomic mode
	assert.Len(t, response.Results, 2)

	// All results should be failures
	for _, result := range response.Results {
		assert.Equal(t, http.StatusConflict, result.Status)
		assert.Nil(t, result.Account)
		assert.NotNil(t, result.Error)
	}
}

// TestCreateAccountsBatch_AtomicMode_BalanceCreationFails tests atomic mode when balance creation fails
func TestCreateAccountsBatch_AtomicMode_BalanceCreationFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, mockAssetRepo, mockAccountRepo, _, _, mockBalance := setupBatchTest(ctrl)

	ctx := context.Background()
	organizationID := uuid.New()
	ledgerID := uuid.New()

	req := &mmodel.CreateAccountBatchRequest{
		Atomic: true,
		Accounts: []mmodel.CreateAccountBatchItem{
			{
				ID:        "req_1",
				Name:      "Account 1",
				Type:      "deposit",
				AssetCode: "USD",
			},
			{
				ID:        "req_2",
				Name:      "Account 2",
				Type:      "savings",
				AssetCode: "USD",
			},
		},
	}

	// Mock health check
	mockBalance.EXPECT().
		CheckHealth(gomock.Any()).
		Return(nil).Times(1)

	// Mock asset validation
	mockAssetRepo.EXPECT().
		FindByNameOrCode(gomock.Any(), organizationID, ledgerID, "", "USD").
		Return(true, nil).Times(1)

	// Mock batch account creation - succeeds
	mockAccountRepo.EXPECT().
		CreateBatch(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, accounts []*mmodel.Account) ([]*mmodel.Account, error) {
			return accounts, nil
		}).Times(1)

	// Mock balance creation - first succeeds, second fails
	gomock.InOrder(
		mockBalance.EXPECT().
			CreateBalanceSync(gomock.Any(), gomock.Any()).
			Return(nil, nil).Times(1),
		mockBalance.EXPECT().
			CreateBalanceSync(gomock.Any(), gomock.Any()).
			Return(nil, errors.New("balance service error")).Times(1),
	)

	// Mock balance rollback - delete balances for the first account (which succeeded)
	mockBalance.EXPECT().
		DeleteAllBalancesByAccountID(gomock.Any(), organizationID, ledgerID, gomock.Any(), gomock.Any()).
		Return(nil).Times(1)

	// Mock account rollback - delete both accounts that were created
	mockAccountRepo.EXPECT().
		Delete(gomock.Any(), organizationID, ledgerID, gomock.Nil(), gomock.Any()).
		Return(nil).Times(2)

	response, err := uc.CreateAccountsBatch(ctx, organizationID, ledgerID, req, "test-token")

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, 0, response.SuccessCount)
	assert.Equal(t, 2, response.FailureCount)
	assert.Len(t, response.Results, 2)

	// All results should be failures
	for _, result := range response.Results {
		assert.Equal(t, http.StatusInternalServerError, result.Status)
		assert.Nil(t, result.Account)
		assert.NotNil(t, result.Error)
		assert.Equal(t, "BALANCE_CREATION_FAILED", result.Error.Code)
	}
}

// TestCreateAccountsBatch_DuplicateAliasInBatch tests that duplicate aliases within the batch are detected
func TestCreateAccountsBatch_DuplicateAliasInBatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, mockAssetRepo, mockAccountRepo, mockMetadataRepo, _, mockBalance := setupBatchTest(ctrl)

	ctx := context.Background()
	organizationID := uuid.New()
	ledgerID := uuid.New()

	duplicateAlias := "@duplicate"
	req := &mmodel.CreateAccountBatchRequest{
		Atomic: false,
		Accounts: []mmodel.CreateAccountBatchItem{
			{
				ID:        "req_1",
				Name:      "Account 1",
				Type:      "deposit",
				AssetCode: "USD",
				Alias:     &duplicateAlias,
			},
			{
				ID:        "req_2",
				Name:      "Account 2",
				Type:      "savings",
				AssetCode: "USD",
				Alias:     &duplicateAlias, // Same alias as req_1
			},
		},
	}

	// Mock health check
	mockBalance.EXPECT().
		CheckHealth(gomock.Any()).
		Return(nil).Times(2) // Once for batch, once for successful account

	// Mock FindByAliases - alias doesn't exist in DB
	mockAccountRepo.EXPECT().
		FindByAliases(gomock.Any(), organizationID, ledgerID, []string{duplicateAlias}).
		Return([]string{}, nil).Times(1)

	// Mock asset validation
	mockAssetRepo.EXPECT().
		FindByNameOrCode(gomock.Any(), organizationID, ledgerID, "", "USD").
		Return(true, nil).Times(2)

	// Mock FindByAlias for successful account
	mockAccountRepo.EXPECT().
		FindByAlias(gomock.Any(), organizationID, ledgerID, duplicateAlias).
		Return(false, nil).AnyTimes()

	// Mock account creation - only first should be created
	mockAccountRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, acc *mmodel.Account) (*mmodel.Account, error) {
			return acc, nil
		}).Times(1)

	// Mock balance creation for successful account
	mockBalance.EXPECT().
		CreateBalanceSync(gomock.Any(), gomock.Any()).
		Return(nil, nil).Times(1)

	// Mock metadata creation
	mockMetadataRepo.EXPECT().
		Create(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()

	response, err := uc.CreateAccountsBatch(ctx, organizationID, ledgerID, req, "test-token")

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, 1, response.SuccessCount)
	assert.Equal(t, 1, response.FailureCount)

	// Check first result (should succeed - first use of alias)
	assert.Equal(t, "req_1", response.Results[0].ID)
	assert.Equal(t, http.StatusCreated, response.Results[0].Status)

	// Check second result (should fail - duplicate alias in batch)
	assert.Equal(t, "req_2", response.Results[1].ID)
	assert.Equal(t, http.StatusConflict, response.Results[1].Status)
	assert.NotNil(t, response.Results[1].Error)
	assert.Equal(t, "DUPLICATE_ALIAS_IN_BATCH", response.Results[1].Error.Code)
}

// TestCreateAccountsBatch_AssetCodeNotFound tests that invalid asset codes are detected
func TestCreateAccountsBatch_AssetCodeNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, mockAssetRepo, mockAccountRepo, mockMetadataRepo, _, mockBalance := setupBatchTest(ctrl)

	ctx := context.Background()
	organizationID := uuid.New()
	ledgerID := uuid.New()

	req := &mmodel.CreateAccountBatchRequest{
		Atomic: false,
		Accounts: []mmodel.CreateAccountBatchItem{
			{
				ID:        "req_1",
				Name:      "Account 1",
				Type:      "deposit",
				AssetCode: "USD",
			},
			{
				ID:        "req_2",
				Name:      "Account 2",
				Type:      "savings",
				AssetCode: "INVALID",
			},
		},
	}

	// Mock health check
	mockBalance.EXPECT().
		CheckHealth(gomock.Any()).
		Return(nil).Times(2)

	// Mock asset validation - USD exists, INVALID does not
	mockAssetRepo.EXPECT().
		FindByNameOrCode(gomock.Any(), organizationID, ledgerID, "", "USD").
		Return(true, nil).Times(2)
	mockAssetRepo.EXPECT().
		FindByNameOrCode(gomock.Any(), organizationID, ledgerID, "", "INVALID").
		Return(false, nil).Times(1)

	// Mock FindByAlias for successful account
	mockAccountRepo.EXPECT().
		FindByAlias(gomock.Any(), organizationID, ledgerID, gomock.Any()).
		Return(false, nil).AnyTimes()

	// Mock account creation for valid account
	mockAccountRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, acc *mmodel.Account) (*mmodel.Account, error) {
			return acc, nil
		}).Times(1)

	// Mock balance creation
	mockBalance.EXPECT().
		CreateBalanceSync(gomock.Any(), gomock.Any()).
		Return(nil, nil).Times(1)

	// Mock metadata creation
	mockMetadataRepo.EXPECT().
		Create(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()

	response, err := uc.CreateAccountsBatch(ctx, organizationID, ledgerID, req, "test-token")

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, 1, response.SuccessCount)
	assert.Equal(t, 1, response.FailureCount)

	// Check first result (should succeed)
	assert.Equal(t, "req_1", response.Results[0].ID)
	assert.Equal(t, http.StatusCreated, response.Results[0].Status)

	// Check second result (should fail - asset not found)
	assert.Equal(t, "req_2", response.Results[1].ID)
	assert.Equal(t, http.StatusConflict, response.Results[1].Status)
	assert.NotNil(t, response.Results[1].Error)
	assert.Equal(t, "ASSET_CODE_NOT_FOUND", response.Results[1].Error.Code)
}

// TestCreateAccountsBatch_AtomicMode_AliasCheckFails tests atomic mode when alias check fails
func TestCreateAccountsBatch_AtomicMode_AliasCheckFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, mockAccountRepo, _, _, mockBalance := setupBatchTest(ctrl)

	ctx := context.Background()
	organizationID := uuid.New()
	ledgerID := uuid.New()

	alias1 := "@alias1"
	alias2 := "@alias2"
	req := &mmodel.CreateAccountBatchRequest{
		Atomic: true,
		Accounts: []mmodel.CreateAccountBatchItem{
			{
				ID:        "req_1",
				Name:      "Account 1",
				Type:      "deposit",
				AssetCode: "USD",
				Alias:     &alias1,
			},
			{
				ID:        "req_2",
				Name:      "Account 2",
				Type:      "savings",
				AssetCode: "USD",
				Alias:     &alias2,
			},
		},
	}

	// Mock health check
	mockBalance.EXPECT().
		CheckHealth(gomock.Any()).
		Return(nil).Times(1)

	// Mock FindByAliases - returns error (database failure)
	mockAccountRepo.EXPECT().
		FindByAliases(gomock.Any(), organizationID, ledgerID, gomock.Any()).
		Return(nil, errors.New("database connection error")).Times(1)

	// In atomic mode with alias check failure, batch should fail immediately
	// No account creation should happen

	response, err := uc.CreateAccountsBatch(ctx, organizationID, ledgerID, req, "test-token")

	assert.NoError(t, err) // Returns response, not error
	assert.NotNil(t, response)
	assert.Equal(t, 0, response.SuccessCount)
	assert.Equal(t, 2, response.FailureCount)
	assert.Len(t, response.Results, 2)

	// All results should be failures with ALIAS_CHECK_FAILED
	for _, result := range response.Results {
		assert.Equal(t, http.StatusConflict, result.Status)
		assert.Nil(t, result.Account)
		assert.NotNil(t, result.Error)
		// Items with aliases should have ALIAS_CHECK_FAILED, others have ATOMIC_BATCH_FAILED
		assert.Contains(t, []string{"ALIAS_CHECK_FAILED", "ATOMIC_BATCH_FAILED"}, result.Error.Code)
	}
}

// TestCreateAccountsBatch_PartialMode_AliasCheckFails tests partial mode when alias check fails
func TestCreateAccountsBatch_PartialMode_AliasCheckFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, mockAssetRepo, mockAccountRepo, mockMetadataRepo, _, mockBalance := setupBatchTest(ctrl)

	ctx := context.Background()
	organizationID := uuid.New()
	ledgerID := uuid.New()

	alias1 := "@alias1"
	req := &mmodel.CreateAccountBatchRequest{
		Atomic: false,
		Accounts: []mmodel.CreateAccountBatchItem{
			{
				ID:        "req_1",
				Name:      "Account 1",
				Type:      "deposit",
				AssetCode: "USD",
				Alias:     &alias1, // Has alias - should fail
			},
			{
				ID:        "req_2",
				Name:      "Account 2",
				Type:      "savings",
				AssetCode: "USD",
				// No alias - should succeed
			},
		},
	}

	// Mock health check
	mockBalance.EXPECT().
		CheckHealth(gomock.Any()).
		Return(nil).Times(2) // Once for batch, once for successful account

	// Mock FindByAliases - returns error (database failure)
	mockAccountRepo.EXPECT().
		FindByAliases(gomock.Any(), organizationID, ledgerID, []string{alias1}).
		Return(nil, errors.New("database connection error")).Times(1)

	// Mock asset validation - still happens for items without alias errors
	mockAssetRepo.EXPECT().
		FindByNameOrCode(gomock.Any(), organizationID, ledgerID, "", "USD").
		Return(true, nil).Times(2) // Once for batch validation, once for successful account

	// Mock FindByAlias for successful account (no alias provided, uses generated ID)
	mockAccountRepo.EXPECT().
		FindByAlias(gomock.Any(), organizationID, ledgerID, gomock.Any()).
		Return(false, nil).AnyTimes()

	// Mock account creation - only one should succeed (the one without alias)
	mockAccountRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, acc *mmodel.Account) (*mmodel.Account, error) {
			return acc, nil
		}).Times(1)

	// Mock balance creation for successful account
	mockBalance.EXPECT().
		CreateBalanceSync(gomock.Any(), gomock.Any()).
		Return(nil, nil).Times(1)

	// Mock metadata creation
	mockMetadataRepo.EXPECT().
		Create(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()

	response, err := uc.CreateAccountsBatch(ctx, organizationID, ledgerID, req, "test-token")

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, 1, response.SuccessCount)
	assert.Equal(t, 1, response.FailureCount)
	assert.Len(t, response.Results, 2)

	// Check first result (should fail - alias check failed)
	assert.Equal(t, "req_1", response.Results[0].ID)
	assert.Equal(t, http.StatusConflict, response.Results[0].Status)
	assert.NotNil(t, response.Results[0].Error)
	assert.Equal(t, "ALIAS_CHECK_FAILED", response.Results[0].Error.Code)

	// Check second result (should succeed - no alias)
	assert.Equal(t, "req_2", response.Results[1].ID)
	assert.Equal(t, http.StatusCreated, response.Results[1].Status)
	assert.NotNil(t, response.Results[1].Account)
}

// TestCreateAccountsBatch_BalanceServiceUnavailable tests that unavailable balance service is handled
func TestCreateAccountsBatch_BalanceServiceUnavailable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _, _, mockBalance := setupBatchTest(ctrl)

	ctx := context.Background()
	organizationID := uuid.New()
	ledgerID := uuid.New()

	req := &mmodel.CreateAccountBatchRequest{
		Atomic: false,
		Accounts: []mmodel.CreateAccountBatchItem{
			{
				ID:        "req_1",
				Name:      "Account 1",
				Type:      "deposit",
				AssetCode: "USD",
			},
		},
	}

	// Mock health check - fails
	mockBalance.EXPECT().
		CheckHealth(gomock.Any()).
		Return(errors.New("balance service unavailable")).Times(1)

	response, err := uc.CreateAccountsBatch(ctx, organizationID, ledgerID, req, "test-token")

	assert.Error(t, err)
	assert.Nil(t, response)
}

// TestCreateAccountsBatch_EmptyBatch tests that empty batch is handled correctly
func TestCreateAccountsBatch_EmptyBatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc, _, _, _, _, mockBalance := setupBatchTest(ctrl)

	ctx := context.Background()
	organizationID := uuid.New()
	ledgerID := uuid.New()

	req := &mmodel.CreateAccountBatchRequest{
		Atomic:   false,
		Accounts: []mmodel.CreateAccountBatchItem{},
	}

	// Mock health check
	mockBalance.EXPECT().
		CheckHealth(gomock.Any()).
		Return(nil).Times(1)

	response, err := uc.CreateAccountsBatch(ctx, organizationID, ledgerID, req, "test-token")

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, 0, response.SuccessCount)
	assert.Equal(t, 0, response.FailureCount)
	assert.Empty(t, response.Results)
}
