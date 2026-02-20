package dynamic_bonding_curve

import (
	"context"
	"fmt"

	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/helpers"
	dbcidl "github.com/krazyTry/meteora-go/gen/dynamic_bonding_curve"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
)

type PartnerService struct {
	*DynamicBondingCurveProgram
	State *StateService
}

type claimPartnerTradingFeeAccounts struct {
	PoolAuthority     solanago.PublicKey
	Config            solanago.PublicKey
	Pool              solanago.PublicKey
	TokenAAccount     solanago.PublicKey
	TokenBAccount     solanago.PublicKey
	BaseVault         solanago.PublicKey
	QuoteVault        solanago.PublicKey
	BaseMint          solanago.PublicKey
	QuoteMint         solanago.PublicKey
	FeeClaimer        solanago.PublicKey
	TokenBaseProgram  solanago.PublicKey
	TokenQuoteProgram solanago.PublicKey
	EventAuthority    solanago.PublicKey
	Program           solanago.PublicKey
}

func NewPartnerService(rpcClient *rpc.Client, commitment rpc.CommitmentType) *PartnerService {
	return &PartnerService{
		DynamicBondingCurveProgram: NewDynamicBondingCurveProgram(rpcClient, commitment),
		State:                      NewStateService(rpcClient, commitment),
	}
}

func (s *PartnerService) CreateConfig(ctx context.Context, params CreateConfigParams) (solanago.Instruction, error) {
	if err := helpers.ValidateConfigParameters(params); err != nil {
		return nil, err
	}
	return dbcidl.NewCreateConfigInstruction(
		params.ConfigParameters,
		params.Config,
		params.FeeClaimer,
		params.LeftoverReceiver,
		params.QuoteMint,
		params.Payer,
		system.ProgramID,
		helpers.DeriveDbcEventAuthority(),
		helpers.DynamicBondingCurveProgramID,
	)
}

func (s *PartnerService) CreatePartnerMetadata(ctx context.Context, params CreatePartnerMetadataParams) (solanago.Instruction, error) {
	partnerMetadata := helpers.DerivePartnerMetadata(params.FeeClaimer)
	meta := CreatePartnerMetadataParameters{
		Padding: [96]uint8{},
		Name:    params.Name,
		Website: params.Website,
		Logo:    params.Logo,
	}
	return dbcidl.NewCreatePartnerMetadataInstruction(
		meta,
		partnerMetadata,
		params.Payer,
		params.FeeClaimer,
		system.ProgramID,
		helpers.DeriveDbcEventAuthority(),
		helpers.DynamicBondingCurveProgramID,
	)
}

// claimWithQuoteMintSol prepares accounts and instructions for SOL-quote claim.
func (s *PartnerService) claimWithQuoteMintSol(ctx context.Context, params ClaimPartnerTradingFeeWithQuoteMintSolParams) (accounts *claimPartnerTradingFeeAccounts, pre []solanago.Instruction, post []solanago.Instruction, err error) {
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

	accounts = &claimPartnerTradingFeeAccounts{
		PoolAuthority:     s.PoolAuthority,
		Config:            params.Config,
		Pool:              params.Pool,
		TokenAAccount:     tokenBaseAccount,
		TokenBAccount:     tokenQuoteAccount,
		BaseVault:         params.PoolState.BaseVault,
		QuoteVault:        params.PoolState.QuoteVault,
		BaseMint:          params.PoolState.BaseMint,
		QuoteMint:         params.PoolConfigState.QuoteMint,
		FeeClaimer:        params.FeeClaimer,
		TokenBaseProgram:  params.TokenBaseProgram,
		TokenQuoteProgram: params.TokenQuoteProgram,
		EventAuthority:    helpers.DeriveDbcEventAuthority(),
		Program:           helpers.DynamicBondingCurveProgramID,
	}
	return accounts, pre, post, nil
}

// claimWithQuoteMintNotSol prepares accounts and pre-instructions for non-SOL quote mint.
func (s *PartnerService) claimWithQuoteMintNotSol(ctx context.Context, params ClaimPartnerTradingFeeWithQuoteMintNotSolParams) (accounts *claimPartnerTradingFeeAccounts, pre []solanago.Instruction, err error) {
	tokenBaseAccount, tokenQuoteAccount, pre, err := s.PrepareTokenAccounts(ctx, params.FeeReceiver, params.Payer, params.PoolState.BaseMint, params.PoolConfigState.QuoteMint, params.TokenBaseProgram, params.TokenQuoteProgram)
	if err != nil {
		return nil, nil, err
	}
	accounts = &claimPartnerTradingFeeAccounts{
		PoolAuthority:     s.PoolAuthority,
		Config:            params.Config,
		Pool:              params.Pool,
		TokenAAccount:     tokenBaseAccount,
		TokenBAccount:     tokenQuoteAccount,
		BaseVault:         params.PoolState.BaseVault,
		QuoteVault:        params.PoolState.QuoteVault,
		BaseMint:          params.PoolState.BaseMint,
		QuoteMint:         params.PoolConfigState.QuoteMint,
		FeeClaimer:        params.FeeClaimer,
		TokenBaseProgram:  params.TokenBaseProgram,
		TokenQuoteProgram: params.TokenQuoteProgram,
		EventAuthority:    helpers.DeriveDbcEventAuthority(),
		Program:           helpers.DynamicBondingCurveProgramID,
	}
	return accounts, pre, nil
}

// ClaimPartnerTradingFee builds claim partner trading fee instructions.
func (s *PartnerService) ClaimPartnerTradingFee(ctx context.Context, params ClaimTradingFeeParams) (pre []solanago.Instruction, ix solanago.Instruction, post []solanago.Instruction, err error) {
	poolState, err := s.State.GetPool(ctx, params.Pool)
	if err != nil {
		return nil, nil, nil, err
	}

	poolConfigState, err := s.State.GetPoolConfig(ctx, poolState.Config)
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
		tempWSol := params.FeeClaimer
		if params.Receiver != nil && !params.Receiver.Equals(params.FeeClaimer) {
			if params.TempWSolAcc == nil {
				return nil, nil, nil, fmt.Errorf("tempWSolAcc required when receiver != feeClaimer")
			}
			tempWSol = *params.TempWSolAcc
		}
		feeReceiver := params.FeeClaimer
		if params.Receiver != nil {
			feeReceiver = *params.Receiver
		}

		accs, preIx, postIx, err := s.claimWithQuoteMintSol(ctx, ClaimPartnerTradingFeeWithQuoteMintSolParams{
			ClaimPartnerTradingFeeWithQuoteMintNotSolParams: ClaimPartnerTradingFeeWithQuoteMintNotSolParams{
				FeeClaimer:        params.FeeClaimer,
				Payer:             params.Payer,
				FeeReceiver:       feeReceiver,
				Config:            poolState.Config,
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

		ix, err = dbcidl.NewClaimTradingFeeInstruction(
			maxBase,
			maxQuote,
			accs.PoolAuthority,
			accs.Config,
			accs.Pool,
			accs.TokenAAccount,
			accs.TokenBAccount,
			accs.BaseVault,
			accs.QuoteVault,
			accs.BaseMint,
			accs.QuoteMint,
			accs.FeeClaimer,
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

	feeReceiver := params.FeeClaimer
	if params.Receiver != nil {
		feeReceiver = *params.Receiver
	}

	accs, preIx, err := s.claimWithQuoteMintNotSol(ctx, ClaimPartnerTradingFeeWithQuoteMintNotSolParams{
		FeeClaimer:        params.FeeClaimer,
		Payer:             params.Payer,
		FeeReceiver:       feeReceiver,
		Config:            poolState.Config,
		Pool:              params.Pool,
		PoolState:         poolState,
		PoolConfigState:   poolConfigState,
		TokenBaseProgram:  tokenBaseProgram,
		TokenQuoteProgram: tokenQuoteProgram,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	ix, err = dbcidl.NewClaimTradingFeeInstruction(
		maxBase,
		maxQuote,
		accs.PoolAuthority,
		accs.Config,
		accs.Pool,
		accs.TokenAAccount,
		accs.TokenBAccount,
		accs.BaseVault,
		accs.QuoteVault,
		accs.BaseMint,
		accs.QuoteMint,
		accs.FeeClaimer,
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

// ClaimPartnerTradingFee2 builds claim partner trading fee instructions for explicit receiver.
func (s *PartnerService) ClaimPartnerTradingFee2(ctx context.Context, params ClaimTradingFee2Params) (pre []solanago.Instruction, ix solanago.Instruction, post []solanago.Instruction, err error) {
	poolState, err := s.State.GetPool(ctx, params.Pool)
	if err != nil {
		return nil, nil, nil, err
	}

	poolConfigState, err := s.State.GetPoolConfig(ctx, poolState.Config)
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

		tokenQuoteAccount, createQuoteIx, err := helpers.GetOrCreateATAInstruction(ctx, s.RPC, poolConfigState.QuoteMint, params.FeeClaimer, params.Payer, tokenQuoteProgram)
		if err != nil {
			return nil, nil, nil, err
		}
		if createQuoteIx != nil {
			pre = append(pre, createQuoteIx)
		}

		unwrapIx, err := helpers.UnwrapSOLInstruction(params.FeeClaimer, params.Receiver, true)
		if err != nil {
			return nil, nil, nil, err
		}
		post = append(post, unwrapIx)

		ix, err = dbcidl.NewClaimTradingFeeInstruction(
			maxBase,
			maxQuote,
			s.PoolAuthority,
			poolState.Config,
			params.Pool,
			tokenBaseAccount,
			tokenQuoteAccount,
			poolState.BaseVault,
			poolState.QuoteVault,
			poolState.BaseMint,
			poolConfigState.QuoteMint,
			params.FeeClaimer,
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

	accs, preIx, err := s.claimWithQuoteMintNotSol(ctx, ClaimPartnerTradingFeeWithQuoteMintNotSolParams{
		FeeClaimer:        params.FeeClaimer,
		Payer:             params.Payer,
		FeeReceiver:       params.Receiver,
		Config:            poolState.Config,
		Pool:              params.Pool,
		PoolState:         poolState,
		PoolConfigState:   poolConfigState,
		TokenBaseProgram:  tokenBaseProgram,
		TokenQuoteProgram: tokenQuoteProgram,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	ix, err = dbcidl.NewClaimTradingFeeInstruction(
		maxBase,
		maxQuote,
		accs.PoolAuthority,
		accs.Config,
		accs.Pool,
		accs.TokenAAccount,
		accs.TokenBAccount,
		accs.BaseVault,
		accs.QuoteVault,
		accs.BaseMint,
		accs.QuoteMint,
		accs.FeeClaimer,
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

// PartnerWithdrawSurplus builds partner withdraw surplus instructions.
func (s *PartnerService) PartnerWithdrawSurplus(ctx context.Context, params PartnerWithdrawSurplusParams) ([]solanago.Instruction, error) {
	poolState, err := s.State.GetPool(ctx, params.VirtualPool)
	if err != nil {
		return nil, err
	}

	poolConfigState, err := s.State.GetPoolConfig(ctx, poolState.Config)
	if err != nil {
		return nil, err
	}

	tokenQuoteProgram := helpers.GetTokenProgram(TokenType(poolConfigState.QuoteTokenFlag))

	pre := []solanago.Instruction{}
	post := []solanago.Instruction{}

	tokenQuoteAccount, createQuoteIx, err := helpers.GetOrCreateATAInstruction(ctx, s.RPC, poolConfigState.QuoteMint, params.FeeClaimer, params.FeeClaimer, tokenQuoteProgram)
	if err != nil {
		return nil, err
	}
	if createQuoteIx != nil {
		pre = append(pre, createQuoteIx)
	}

	if helpers.IsNativeSol(poolConfigState.QuoteMint) {
		unwrapIx, err := helpers.UnwrapSOLInstruction(params.FeeClaimer, params.FeeClaimer, true)
		if err != nil {
			return nil, err
		}
		post = append(post, unwrapIx)
	}

	ix, err := dbcidl.NewPartnerWithdrawSurplusInstruction(
		s.PoolAuthority,
		poolState.Config,
		params.VirtualPool,
		tokenQuoteAccount,
		poolState.QuoteVault,
		poolConfigState.QuoteMint,
		params.FeeClaimer,
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

// PartnerWithdrawMigrationFee builds partner withdraw migration fee instructions.
func (s *PartnerService) PartnerWithdrawMigrationFee(ctx context.Context, params WithdrawMigrationFeeParams) ([]solanago.Instruction, error) {
	virtualPoolState, err := s.State.GetPool(ctx, params.VirtualPool)
	if err != nil {
		return nil, err
	}

	configState, err := s.State.GetPoolConfig(ctx, virtualPoolState.Config)
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
		0, // 0. partner 1. creator
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

// ClaimPartnerPoolCreationFee builds claim partner pool creation fee instruction.
func (s *PartnerService) ClaimPartnerPoolCreationFee(ctx context.Context, params ClaimPartnerPoolCreationFeeParams) (solanago.Instruction, error) {
	virtualPoolState, err := s.State.GetPool(ctx, params.VirtualPool)
	if err != nil {
		return nil, err
	}

	configState, err := s.State.GetPoolConfig(ctx, virtualPoolState.Config)
	if err != nil {
		return nil, err
	}

	return dbcidl.NewClaimPartnerPoolCreationFeeInstruction(
		virtualPoolState.Config,
		params.VirtualPool,
		configState.FeeClaimer,
		params.FeeReceiver,
		helpers.DeriveDbcEventAuthority(),
		helpers.DynamicBondingCurveProgramID,
	)
}
