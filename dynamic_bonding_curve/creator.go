package dynamic_bonding_curve

import (
	"context"
	"fmt"

	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/helpers"
	dbcidl "github.com/krazyTry/meteora-go/gen/dynamic_bonding_curve"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
)

type claimCreatorTradingFeeAccounts struct {
	PoolAuthority     solanago.PublicKey
	Pool              solanago.PublicKey
	TokenAAccount     solanago.PublicKey
	TokenBAccount     solanago.PublicKey
	BaseVault         solanago.PublicKey
	QuoteVault        solanago.PublicKey
	BaseMint          solanago.PublicKey
	QuoteMint         solanago.PublicKey
	Creator           solanago.PublicKey
	TokenBaseProgram  solanago.PublicKey
	TokenQuoteProgram solanago.PublicKey
	EventAuthority    solanago.PublicKey
	Program           solanago.PublicKey
}

func (s *DynamicBondingCurve) CreatePoolMetadata(ctx context.Context, params CreateVirtualPoolMetadataParams) (solanago.Instruction, error) {
	meta := CreateVirtualPoolMetadataParameters{
		Padding: [96]uint8{},
		Name:    params.Name,
		Website: params.Website,
		Logo:    params.Logo,
	}
	virtualPoolMetadata := helpers.DeriveVirtualPoolMetadata(params.VirtualPool, 0)
	return dbcidl.NewCreateVirtualPoolMetadataInstruction(
		meta,
		params.VirtualPool,
		virtualPoolMetadata,
		params.Creator,
		params.Payer,
		system.ProgramID,
		helpers.DeriveDbcEventAuthority(),
		helpers.DynamicBondingCurveProgramID,
	)
}

func (s *DynamicBondingCurve) TransferPoolCreator(ctx context.Context, params TransferPoolCreatorParams) (solanago.Instruction, error) {
	virtualPoolState, err := s.GetPool(ctx, params.VirtualPool)
	if err != nil {
		return nil, err
	}
	migrationMetadata := helpers.DeriveDammV1MigrationMetadataAddress(params.VirtualPool)

	ix, err := dbcidl.NewTransferPoolCreatorInstruction(
		params.VirtualPool,
		virtualPoolState.Config,
		params.Creator,
		params.NewCreator,
		helpers.DeriveDbcEventAuthority(),
		helpers.DynamicBondingCurveProgramID,
	)
	if err != nil {
		return nil, err
	}
	if inst, ok := ix.(*solanago.GenericInstruction); ok {
		inst.AccountValues = append(inst.AccountValues, solanago.NewAccountMeta(migrationMetadata, false, false))
	}
	return ix, nil
}

func (s *DynamicBondingCurve) CreatorWithdrawSurplus(ctx context.Context, params CreatorWithdrawSurplusParams) ([]solanago.Instruction, error) {
	poolState, err := s.GetPool(ctx, params.VirtualPool)
	if err != nil {
		return nil, err
	}

	poolConfigState, err := s.GetPoolConfig(ctx, poolState.Config)
	if err != nil {
		return nil, err
	}

	tokenQuoteProgram := helpers.GetTokenProgram(TokenType(poolConfigState.QuoteTokenFlag))

	pre := []solanago.Instruction{}
	post := []solanago.Instruction{}

	tokenQuoteAccount, createIx, err := helpers.GetOrCreateATAInstruction(ctx, s.RPC, poolConfigState.QuoteMint, params.Creator, params.Creator, tokenQuoteProgram)
	if err != nil {
		return nil, err
	}
	if createIx != nil {
		pre = append(pre, createIx)
	}

	if helpers.IsNativeSol(poolConfigState.QuoteMint) {
		unwrapIx, err := helpers.UnwrapSOLInstruction(params.Creator, params.Creator, true)
		if err != nil {
			return nil, err
		}
		post = append(post, unwrapIx)
	}

	ix, err := dbcidl.NewCreatorWithdrawSurplusInstruction(
		s.PoolAuthority,
		poolState.Config,
		params.VirtualPool,
		tokenQuoteAccount,
		poolState.QuoteVault,
		poolConfigState.QuoteMint,
		params.Creator,
		tokenQuoteProgram,
		helpers.DeriveDbcEventAuthority(),
		helpers.DynamicBondingCurveProgramID,
	)
	if err != nil {
		return nil, err
	}
	out := append(pre, ix)
	out = append(out, post...)
	return out, nil
}

func (s *DynamicBondingCurve) WithdrawMigrationFee(ctx context.Context, params WithdrawMigrationFeeParams) ([]solanago.Instruction, error) {
	virtualPoolState, err := s.GetPool(ctx, params.VirtualPool)
	if err != nil {
		return nil, err
	}

	configState, err := s.GetPoolConfig(ctx, virtualPoolState.Config)
	if err != nil {
		return nil, err
	}

	tokenQuoteProgram := helpers.GetTokenProgram(TokenType(configState.QuoteTokenFlag))

	pre := []solanago.Instruction{}
	post := []solanago.Instruction{}

	tokenQuoteAccount, createIx, err := helpers.GetOrCreateATAInstruction(ctx, s.RPC, configState.QuoteMint, params.Sender, params.Sender, tokenQuoteProgram)
	if err != nil {
		return nil, err
	}
	if createIx != nil {
		pre = append(pre, createIx)
	}

	if helpers.IsNativeSol(configState.QuoteMint) {
		unwrapIx, err := helpers.UnwrapSOLInstruction(params.Sender, params.Sender, true)
		if err != nil {
			return nil, err
		}
		post = append(post, unwrapIx)
	}

	ix, err := dbcidl.NewWithdrawMigrationFeeInstruction(
		1, // 0. partner 1. creator
		s.PoolAuthority,
		virtualPoolState.Config,
		params.VirtualPool,
		tokenQuoteAccount,
		virtualPoolState.QuoteVault,
		configState.QuoteMint,
		params.Sender,
		tokenQuoteProgram,
		helpers.DeriveDbcEventAuthority(),
		helpers.DynamicBondingCurveProgramID,
	)
	if err != nil {
		return nil, err
	}
	out := append(pre, ix)
	out = append(out, post...)
	return out, nil
}

// claimWithQuoteMintSol prepares accounts and instructions for SOL-quote claim.
func (s *DynamicBondingCurve) claimCreatorWithQuoteMintSol(ctx context.Context, params ClaimCreatorTradingFeeWithQuoteMintSolParams) (accounts *claimCreatorTradingFeeAccounts, pre []solanago.Instruction, post []solanago.Instruction, err error) {
	pre = []solanago.Instruction{}
	post = []solanago.Instruction{}

	tokenBaseAccount, createBaseIx, err := helpers.GetOrCreateATAInstruction(ctx, s.RPC, params.PoolState.BaseMint, params.FeeReceiver, params.Payer, params.TokenBaseProgram)
	if err != nil {
		return nil, nil, nil, err
	}
	if createBaseIx != nil {
		pre = append(pre, createBaseIx)
	}

	tokenQuoteAccount, createQuoteIx, err := helpers.GetOrCreateATAInstruction(ctx, s.RPC, params.PoolConfigState.QuoteMint, params.TempWSolAcc, params.Payer, params.TokenQuoteProgram)
	if err != nil {
		return nil, nil, nil, err
	}
	if createQuoteIx != nil {
		pre = append(pre, createQuoteIx)
	}

	unwrapIx, err := helpers.UnwrapSOLInstruction(params.TempWSolAcc, params.FeeReceiver, true)
	if err != nil {
		return nil, nil, nil, err
	}
	post = append(post, unwrapIx)

	accounts = &claimCreatorTradingFeeAccounts{
		PoolAuthority:     s.PoolAuthority,
		Pool:              params.Pool,
		TokenAAccount:     tokenBaseAccount,
		TokenBAccount:     tokenQuoteAccount,
		BaseVault:         params.PoolState.BaseVault,
		QuoteVault:        params.PoolState.QuoteVault,
		BaseMint:          params.PoolState.BaseMint,
		QuoteMint:         params.PoolConfigState.QuoteMint,
		Creator:           params.Creator,
		TokenBaseProgram:  params.TokenBaseProgram,
		TokenQuoteProgram: params.TokenQuoteProgram,
		EventAuthority:    helpers.DeriveDbcEventAuthority(),
		Program:           helpers.DynamicBondingCurveProgramID,
	}
	return accounts, pre, post, nil
}

// claimWithQuoteMintNotSol prepares accounts and pre-instructions for non-SOL quote mint.
func (s *DynamicBondingCurve) claimCreatorWithQuoteMintNotSol(ctx context.Context, params ClaimCreatorTradingFeeWithQuoteMintNotSolParams) (accounts *claimCreatorTradingFeeAccounts, pre []solanago.Instruction, err error) {
	tokenBaseAccount, tokenQuoteAccount, pre, err := s.PrepareTokenAccounts(ctx, params.FeeReceiver, params.Payer, params.PoolState.BaseMint, params.PoolConfigState.QuoteMint, params.TokenBaseProgram, params.TokenQuoteProgram)
	if err != nil {
		return nil, nil, err
	}
	accounts = &claimCreatorTradingFeeAccounts{
		PoolAuthority:     s.PoolAuthority,
		Pool:              params.Pool,
		TokenAAccount:     tokenBaseAccount,
		TokenBAccount:     tokenQuoteAccount,
		BaseVault:         params.PoolState.BaseVault,
		QuoteVault:        params.PoolState.QuoteVault,
		BaseMint:          params.PoolState.BaseMint,
		QuoteMint:         params.PoolConfigState.QuoteMint,
		Creator:           params.Creator,
		TokenBaseProgram:  params.TokenBaseProgram,
		TokenQuoteProgram: params.TokenQuoteProgram,
		EventAuthority:    helpers.DeriveDbcEventAuthority(),
		Program:           helpers.DynamicBondingCurveProgramID,
	}
	return accounts, pre, nil
}

// ClaimCreatorTradingFee builds claim creator trading fee instructions.
func (s *DynamicBondingCurve) ClaimCreatorTradingFee(ctx context.Context, params ClaimCreatorTradingFeeParams) (pre []solanago.Instruction, ix solanago.Instruction, post []solanago.Instruction, err error) {
	poolState, err := s.GetPool(ctx, params.Pool)
	if err != nil {
		return nil, nil, nil, err
	}

	poolConfigState, err := s.GetPoolConfig(ctx, poolState.Config)
	if err != nil {
		return nil, nil, nil, err
	}

	tokenBaseProgram := helpers.GetTokenProgram(TokenType(poolConfigState.TokenType))
	tokenQuoteProgram := helpers.GetTokenProgram(TokenType(poolConfigState.QuoteTokenFlag))

	maxBase, err := helpers.BigIntToU64(params.MaxBaseAmount)
	if err != nil {
		return nil, nil, nil, err
	}
	maxQuote, err := helpers.BigIntToU64(params.MaxQuoteAmount)
	if err != nil {
		return nil, nil, nil, err
	}

	if helpers.IsNativeSol(poolConfigState.QuoteMint) {
		tempWSol := params.Creator
		if params.Receiver != nil && !params.Receiver.Equals(params.Creator) {
			if params.TempWSolAcc == nil {
				return nil, nil, nil, fmt.Errorf("tempWSolAcc required when receiver != creator")
			}
			tempWSol = *params.TempWSolAcc
		}
		feeReceiver := params.Creator
		if params.Receiver != nil {
			feeReceiver = *params.Receiver
		}

		accs, preIx, postIx, err := s.claimCreatorWithQuoteMintSol(ctx, ClaimCreatorTradingFeeWithQuoteMintSolParams{
			ClaimCreatorTradingFeeWithQuoteMintNotSolParams: ClaimCreatorTradingFeeWithQuoteMintNotSolParams{
				Creator:           params.Creator,
				Payer:             params.Payer,
				FeeReceiver:       feeReceiver,
				Pool:              params.Pool,
				PoolState:         poolState,
				PoolConfigState:   poolConfigState,
				TokenBaseProgram:  tokenBaseProgram,
				TokenQuoteProgram: tokenQuoteProgram,
			},
			TempWSolAcc: tempWSol,
		})
		if err != nil {
			return nil, nil, nil, err
		}

		ix, err = dbcidl.NewClaimCreatorTradingFeeInstruction(
			maxBase,
			maxQuote,
			accs.PoolAuthority,
			accs.Pool,
			accs.TokenAAccount,
			accs.TokenBAccount,
			accs.BaseVault,
			accs.QuoteVault,
			accs.BaseMint,
			accs.QuoteMint,
			accs.Creator,
			accs.TokenBaseProgram,
			accs.TokenQuoteProgram,
			accs.EventAuthority,
			accs.Program,
		)
		if err != nil {
			return nil, nil, nil, err
		}
		return preIx, ix, postIx, nil
	}

	feeReceiver := params.Creator
	if params.Receiver != nil {
		feeReceiver = *params.Receiver
	}
	accs, preIx, err := s.claimCreatorWithQuoteMintNotSol(ctx, ClaimCreatorTradingFeeWithQuoteMintNotSolParams{
		Creator:           params.Creator,
		Payer:             params.Payer,
		FeeReceiver:       feeReceiver,
		Pool:              params.Pool,
		PoolState:         poolState,
		PoolConfigState:   poolConfigState,
		TokenBaseProgram:  tokenBaseProgram,
		TokenQuoteProgram: tokenQuoteProgram,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	ix, err = dbcidl.NewClaimCreatorTradingFeeInstruction(
		maxBase,
		maxQuote,
		accs.PoolAuthority,
		accs.Pool,
		accs.TokenAAccount,
		accs.TokenBAccount,
		accs.BaseVault,
		accs.QuoteVault,
		accs.BaseMint,
		accs.QuoteMint,
		accs.Creator,
		accs.TokenBaseProgram,
		accs.TokenQuoteProgram,
		accs.EventAuthority,
		accs.Program,
	)
	if err != nil {
		return nil, nil, nil, err
	}
	return preIx, ix, nil, nil
}

// ClaimCreatorTradingFee2 builds claim creator trading fee instructions for explicit receiver.
func (s *DynamicBondingCurve) ClaimCreatorTradingFee2(ctx context.Context, params ClaimCreatorTradingFee2Params) (pre []solanago.Instruction, ix solanago.Instruction, post []solanago.Instruction, err error) {
	poolState, err := s.GetPool(ctx, params.Pool)
	if err != nil {
		return nil, nil, nil, err
	}

	poolConfigState, err := s.GetPoolConfig(ctx, poolState.Config)
	if err != nil {
		return nil, nil, nil, err
	}

	tokenBaseProgram := helpers.GetTokenProgram(TokenType(poolConfigState.TokenType))
	tokenQuoteProgram := helpers.GetTokenProgram(TokenType(poolConfigState.QuoteTokenFlag))

	maxBase, err := helpers.BigIntToU64(params.MaxBaseAmount)
	if err != nil {
		return nil, nil, nil, err
	}
	maxQuote, err := helpers.BigIntToU64(params.MaxQuoteAmount)
	if err != nil {
		return nil, nil, nil, err
	}

	if helpers.IsNativeSol(poolConfigState.QuoteMint) {
		pre = []solanago.Instruction{}
		post = []solanago.Instruction{}

		tokenBaseAccount, createBaseIx, err := helpers.GetOrCreateATAInstruction(ctx, s.RPC, poolState.BaseMint, params.Receiver, params.Payer, tokenBaseProgram)
		if err != nil {
			return nil, nil, nil, err
		}
		if createBaseIx != nil {
			pre = append(pre, createBaseIx)
		}

		tokenQuoteAccount, createQuoteIx, err := helpers.GetOrCreateATAInstruction(ctx, s.RPC, poolConfigState.QuoteMint, params.Creator, params.Payer, tokenQuoteProgram)
		if err != nil {
			return nil, nil, nil, err
		}
		if createQuoteIx != nil {
			pre = append(pre, createQuoteIx)
		}

		unwrapIx, err := helpers.UnwrapSOLInstruction(params.Creator, params.Receiver, true)
		if err != nil {
			return nil, nil, nil, err
		}
		post = append(post, unwrapIx)

		ix, err = dbcidl.NewClaimCreatorTradingFeeInstruction(
			maxBase,
			maxQuote,
			s.PoolAuthority,
			params.Pool,
			tokenBaseAccount,
			tokenQuoteAccount,
			poolState.BaseVault,
			poolState.QuoteVault,
			poolState.BaseMint,
			poolConfigState.QuoteMint,
			params.Creator,
			tokenBaseProgram,
			tokenQuoteProgram,
			helpers.DeriveDbcEventAuthority(),
			helpers.DynamicBondingCurveProgramID,
		)
		if err != nil {
			return nil, nil, nil, err
		}
		return pre, ix, post, nil
	}

	accs, preIx, err := s.claimCreatorWithQuoteMintNotSol(ctx, ClaimCreatorTradingFeeWithQuoteMintNotSolParams{
		Creator:           params.Creator,
		Payer:             params.Payer,
		FeeReceiver:       params.Receiver,
		Pool:              params.Pool,
		PoolState:         poolState,
		PoolConfigState:   poolConfigState,
		TokenBaseProgram:  tokenBaseProgram,
		TokenQuoteProgram: tokenQuoteProgram,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	ix, err = dbcidl.NewClaimCreatorTradingFeeInstruction(
		maxBase,
		maxQuote,
		accs.PoolAuthority,
		accs.Pool,
		accs.TokenAAccount,
		accs.TokenBAccount,
		accs.BaseVault,
		accs.QuoteVault,
		accs.BaseMint,
		accs.QuoteMint,
		accs.Creator,
		accs.TokenBaseProgram,
		accs.TokenQuoteProgram,
		accs.EventAuthority,
		accs.Program,
	)
	if err != nil {
		return nil, nil, nil, err
	}
	return preIx, ix, nil, nil
}
