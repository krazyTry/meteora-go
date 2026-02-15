package dynamic_bonding_curve

import (
	"context"

	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/helpers"
	dbcidl "github.com/krazyTry/meteora-go/gen/dynamic_bonding_curve"
	dynamicvault "github.com/krazyTry/meteora-go/gen/dynamic_vault"

	solanago "github.com/gagliardetto/solana-go"
	computebudget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

type MigrationService struct {
	*DynamicBondingCurveProgram
	State *StateService
}

func NewMigrationService(rpcClient *rpc.Client, commitment rpc.CommitmentType) *MigrationService {
	return &MigrationService{
		DynamicBondingCurveProgram: NewDynamicBondingCurveProgram(rpcClient, commitment),
		State:                      NewStateService(rpcClient, commitment),
	}
}

// CreateLocker creates locker accounts if needed.
func (s *MigrationService) CreateLocker(ctx context.Context, params CreateLockerParams) (pre []solanago.Instruction, ix solanago.Instruction, post []solanago.Instruction, err error) {
	virtualPoolState, err := s.State.GetPool(ctx, params.VirtualPool)
	if err != nil {
		return nil, nil, nil, err
	}

	poolConfigState, err := s.State.GetPoolConfig(ctx, virtualPoolState.Config)
	if err != nil {
		return nil, nil, nil, err
	}

	lockerEventAuthority := helpers.DeriveLockerEventAuthority()
	base := helpers.DeriveBaseKeyForLocker(params.VirtualPool)
	escrow := helpers.DeriveEscrow(base)

	tokenProgram := helpers.GetTokenProgram(TokenType(poolConfigState.TokenType))
	escrowToken, createIx, err := helpers.GetOrCreateATAInstruction(ctx, s.RPC, virtualPoolState.BaseMint, escrow, params.Payer, tokenProgram)
	if err != nil {
		return nil, nil, nil, err
	}
	pre = []solanago.Instruction{}
	if createIx != nil {
		pre = append(pre, createIx)
	}

	ix, err = dbcidl.NewCreateLockerInstruction(
		params.VirtualPool,
		virtualPoolState.Config,
		s.PoolAuthority,
		virtualPoolState.BaseVault,
		virtualPoolState.BaseMint,
		base,
		virtualPoolState.Creator,
		escrow,
		escrowToken,
		params.Payer,
		tokenProgram,
		LockerProgramID,
		lockerEventAuthority,
		system.ProgramID,
	)
	if err != nil {
		return nil, nil, nil, err
	}
	return pre, ix, nil, nil
}

// WithdrawLeftover withdraws leftover base token to leftover receiver.
func (s *MigrationService) WithdrawLeftover(ctx context.Context, params WithdrawLeftoverParams) (pre []solanago.Instruction, ix solanago.Instruction, post []solanago.Instruction, err error) {
	poolState, err := s.State.GetPool(ctx, params.VirtualPool)
	if err != nil {
		return nil, nil, nil, err
	}

	poolConfigState, err := s.State.GetPoolConfig(ctx, poolState.Config)
	if err != nil {
		return nil, nil, nil, err
	}

	tokenBaseProgram := helpers.GetTokenProgram(TokenType(poolConfigState.TokenType))
	tokenBaseAccount, createIx, err := helpers.GetOrCreateATAInstruction(ctx, s.RPC, poolState.BaseMint, poolConfigState.LeftoverReceiver, params.Payer, tokenBaseProgram)
	if err != nil {
		return nil, nil, nil, err
	}
	pre = []solanago.Instruction{}
	if createIx != nil {
		pre = append(pre, createIx)
	}

	ix, err = dbcidl.NewWithdrawLeftoverInstruction(
		s.PoolAuthority,
		poolState.Config,
		params.VirtualPool,
		tokenBaseAccount,
		poolState.BaseVault,
		poolState.BaseMint,
		poolConfigState.LeftoverReceiver,
		tokenBaseProgram,
		helpers.DeriveDbcEventAuthority(),
		DynamicBondingCurveProgramID,
	)
	if err != nil {
		return nil, nil, nil, err
	}
	return pre, ix, nil, nil
}

// CreateDammV1MigrationMetadata creates migration metadata for DAMM V1.
func (s *MigrationService) CreateDammV1MigrationMetadata(ctx context.Context, params CreateDammV1MigrationMetadataParams) (solanago.Instruction, error) {
	migrationMetadata := helpers.DeriveDammV1MigrationMetadataAddress(params.VirtualPool)
	return dbcidl.NewMigrationMeteoraDammCreateMetadataInstruction(
		params.VirtualPool,
		params.Config,
		migrationMetadata,
		params.Payer,
		system.ProgramID,
		helpers.DeriveDbcEventAuthority(),
		DynamicBondingCurveProgramID,
	)
}

// MigrateToDammV1 builds migration instruction and optional vault init pre-instructions.
func (s *MigrationService) MigrateToDammV1(ctx context.Context, params MigrateToDammV1Params) (pre []solanago.Instruction, ix solanago.Instruction, post []solanago.Instruction, err error) {
	poolState, err := s.State.GetPool(ctx, params.VirtualPool)
	if err != nil {
		return nil, nil, nil, err
	}

	poolConfigState, err := s.State.GetPoolConfig(ctx, poolState.Config)
	if err != nil {
		return nil, nil, nil, err
	}

	migrationMetadata := helpers.DeriveDammV1MigrationMetadataAddress(params.VirtualPool)
	dammPool := helpers.DeriveDammV1PoolAddress(params.DammConfig, poolState.BaseMint, poolConfigState.QuoteMint)
	lpMint := helpers.DeriveDammV1LpMintAddress(dammPool)
	mintMetadata := helpers.DeriveMintMetadata(lpMint)
	protocolTokenAFee := helpers.DeriveDammV1ProtocolFeeAddress(poolState.BaseMint, dammPool)
	protocolTokenBFee := helpers.DeriveDammV1ProtocolFeeAddress(poolConfigState.QuoteMint, dammPool)

	pre = []solanago.Instruction{}

	aVault := helpers.DeriveVaultAddress(poolState.BaseMint, BaseAddress)
	aTokenVault := helpers.DeriveTokenVaultKey(aVault)
	aLpMintPda := helpers.DeriveVaultLpMintAddress(aVault)
	bVault := helpers.DeriveVaultAddress(poolConfigState.QuoteMint, BaseAddress)
	bTokenVault := helpers.DeriveTokenVaultKey(bVault)
	bLpMintPda := helpers.DeriveVaultLpMintAddress(bVault)

	aVaultLpMint := aLpMintPda
	bVaultLpMint := bLpMintPda

	if acc, err := s.RPC.GetAccountInfo(ctx, aVault); err == nil && acc != nil && acc.Value != nil {
		vaultAcc, err := dynamicvault.ParseAccount_Vault(acc.Value.Data.GetBinary())
		if err == nil {
			aVaultLpMint = vaultAcc.LpMint
		}
	} else {
		_, _, lpMintKey, createIx, err := helpers.CreateInitializePermissionlessDynamicVaultIx(poolState.BaseMint, params.Payer)
		if err != nil {
			return nil, nil, nil, err
		}
		if createIx != nil {
			pre = append(pre, createIx)
		}
		aVaultLpMint = lpMintKey
	}

	if acc, err := s.RPC.GetAccountInfo(ctx, bVault); err == nil && acc != nil && acc.Value != nil {
		vaultAcc, err := dynamicvault.ParseAccount_Vault(acc.Value.Data.GetBinary())
		if err == nil {
			bVaultLpMint = vaultAcc.LpMint
		}
	} else {
		_, _, lpMintKey, createIx, err := helpers.CreateInitializePermissionlessDynamicVaultIx(poolConfigState.QuoteMint, params.Payer)
		if err != nil {
			return nil, nil, nil, err
		}
		if createIx != nil {
			pre = append(pre, createIx)
		}
		bVaultLpMint = lpMintKey
	}

	aVaultLp := helpers.DeriveDammV1VaultLPAddress(aVault, dammPool)
	bVaultLp := helpers.DeriveDammV1VaultLPAddress(bVault, dammPool)

	virtualPoolLp, err := helpers.FindAssociatedTokenAddress(s.PoolAuthority, lpMint, token.ProgramID)
	if err != nil {
		return nil, nil, nil, err
	}

	ix, err = dbcidl.NewMigrateMeteoraDammInstruction(
		params.VirtualPool,
		migrationMetadata,
		poolState.Config,
		s.PoolAuthority,
		dammPool,
		params.DammConfig,
		lpMint,
		poolState.BaseMint,
		poolConfigState.QuoteMint,
		aVault,
		bVault,
		aTokenVault,
		bTokenVault,
		aVaultLpMint,
		bVaultLpMint,
		aVaultLp,
		bVaultLp,
		poolState.BaseVault,
		poolState.QuoteVault,
		virtualPoolLp,
		protocolTokenAFee,
		protocolTokenBFee,
		params.Payer,
		solanago.SysVarRentPubkey,
		mintMetadata,
		MetaplexProgramID,
		DammV1ProgramID,
		VaultProgramID,
		token.ProgramID,
		solanago.SPLAssociatedTokenAccountProgramID,
		system.ProgramID,
	)
	if err != nil {
		return nil, nil, nil, err
	}

	// add compute budget
	cuIx := computebudget.NewSetComputeUnitLimitInstructionBuilder().SetUnits(500000).Build()
	pre = append([]solanago.Instruction{cuIx}, pre...)

	return pre, ix, nil, nil
}

// LockDammV1LpToken locks DAMM V1 LP tokens for creator or partner.
func (s *MigrationService) LockDammV1LpToken(ctx context.Context, params DammLpTokenParams) (pre []solanago.Instruction, ix solanago.Instruction, post []solanago.Instruction, err error) {
	poolState, err := s.State.GetPool(ctx, params.VirtualPool)
	if err != nil {
		return nil, nil, nil, err
	}

	poolConfigState, err := s.State.GetPoolConfig(ctx, poolState.Config)
	if err != nil {
		return nil, nil, nil, err
	}

	dammPool := helpers.DeriveDammV1PoolAddress(params.DammConfig, poolState.BaseMint, poolConfigState.QuoteMint)
	migrationMetadata := helpers.DeriveDammV1MigrationMetadataAddress(params.VirtualPool)

	aVault := helpers.DeriveVaultAddress(poolState.BaseMint, BaseAddress)
	aLpMintPda := helpers.DeriveVaultLpMintAddress(aVault)
	bVault := helpers.DeriveVaultAddress(poolConfigState.QuoteMint, BaseAddress)
	bLpMintPda := helpers.DeriveVaultLpMintAddress(bVault)

	aVaultLpMint := aLpMintPda
	bVaultLpMint := bLpMintPda

	pre = []solanago.Instruction{}

	if acc, err := s.RPC.GetAccountInfo(ctx, aVault); err == nil && acc != nil && acc.Value != nil {
		vaultAcc, err := dynamicvault.ParseAccount_Vault(acc.Value.Data.GetBinary())
		if err == nil {
			aVaultLpMint = vaultAcc.LpMint
		}
	} else {
		_, _, lpMintKey, createIx, err := helpers.CreateInitializePermissionlessDynamicVaultIx(poolState.BaseMint, params.Payer)
		if err != nil {
			return nil, nil, nil, err
		}
		if createIx != nil {
			pre = append(pre, createIx)
		}
		aVaultLpMint = lpMintKey
	}

	if acc, err := s.RPC.GetAccountInfo(ctx, bVault); err == nil && acc != nil && acc.Value != nil {
		vaultAcc, err := dynamicvault.ParseAccount_Vault(acc.Value.Data.GetBinary())
		if err == nil {
			bVaultLpMint = vaultAcc.LpMint
		}
	} else {
		_, _, lpMintKey, createIx, err := helpers.CreateInitializePermissionlessDynamicVaultIx(poolConfigState.QuoteMint, params.Payer)
		if err != nil {
			return nil, nil, nil, err
		}
		if createIx != nil {
			pre = append(pre, createIx)
		}
		bVaultLpMint = lpMintKey
	}

	aVaultLp := helpers.DeriveDammV1VaultLPAddress(aVault, dammPool)
	bVaultLp := helpers.DeriveDammV1VaultLPAddress(bVault, dammPool)
	lpMint := helpers.DeriveDammV1LpMintAddress(dammPool)

	var lockEscrowKey solanago.PublicKey
	if params.IsPartner {
		lockEscrowKey = helpers.DeriveDammV1LockEscrowAddress(dammPool, poolConfigState.FeeClaimer)
		if info, _ := s.RPC.GetAccountInfo(ctx, lockEscrowKey); info == nil || info.Value == nil {
			lockIx, err := helpers.CreateLockEscrowIx(params.Payer, dammPool, lpMint, poolConfigState.FeeClaimer, lockEscrowKey)
			if err != nil {
				return nil, nil, nil, err
			}
			pre = append(pre, lockIx)
		}
	} else {
		lockEscrowKey = helpers.DeriveDammV1LockEscrowAddress(dammPool, poolState.Creator)
		if info, _ := s.RPC.GetAccountInfo(ctx, lockEscrowKey); info == nil || info.Value == nil {
			lockIx, err := helpers.CreateLockEscrowIx(params.Payer, dammPool, lpMint, poolState.Creator, lockEscrowKey)
			if err != nil {
				return nil, nil, nil, err
			}
			pre = append(pre, lockIx)
		}
	}

	escrowVault, createEscrowVaultIx, err := helpers.GetOrCreateATAInstruction(ctx, s.RPC, lpMint, lockEscrowKey, params.Payer, token.ProgramID)
	if err != nil {
		return nil, nil, nil, err
	}
	if createEscrowVaultIx != nil {
		pre = append(pre, createEscrowVaultIx)
	}

	sourceTokens, err := helpers.FindAssociatedTokenAddress(s.PoolAuthority, lpMint, token.ProgramID)
	if err != nil {
		return nil, nil, nil, err
	}

	owner := poolState.Creator
	if params.IsPartner {
		owner = poolConfigState.FeeClaimer
	}

	ix, err = dbcidl.NewMigrateMeteoraDammLockLpTokenInstruction(
		params.VirtualPool,
		migrationMetadata,
		s.PoolAuthority,
		dammPool,
		lpMint,
		lockEscrowKey,
		owner,
		sourceTokens,
		escrowVault,
		DammV1ProgramID,
		aVault,
		bVault,
		aVaultLp,
		bVaultLp,
		aVaultLpMint,
		bVaultLpMint,
		token.ProgramID,
	)
	if err != nil {
		return nil, nil, nil, err
	}
	return pre, ix, nil, nil
}

// ClaimDammV1LpToken claims DAMM V1 LP tokens for creator or partner.
func (s *MigrationService) ClaimDammV1LpToken(ctx context.Context, params DammLpTokenParams) (pre []solanago.Instruction, ix solanago.Instruction, post []solanago.Instruction, err error) {
	virtualPoolState, err := s.State.GetPool(ctx, params.VirtualPool)
	if err != nil {
		return nil, nil, nil, err
	}

	poolConfigState, err := s.State.GetPoolConfig(ctx, virtualPoolState.Config)
	if err != nil {
		return nil, nil, nil, err
	}

	dammPool := helpers.DeriveDammV1PoolAddress(params.DammConfig, virtualPoolState.BaseMint, poolConfigState.QuoteMint)
	migrationMetadata := helpers.DeriveDammV1MigrationMetadataAddress(params.VirtualPool)
	lpMint := helpers.DeriveDammV1LpMintAddress(dammPool)

	pre = []solanago.Instruction{}
	owner := virtualPoolState.Creator
	if params.IsPartner {
		owner = poolConfigState.FeeClaimer
	}
	destinationToken, createDestinationIx, err := helpers.GetOrCreateATAInstruction(ctx, s.RPC, lpMint, owner, params.Payer, token.ProgramID)
	if err != nil {
		return nil, nil, nil, err
	}
	if createDestinationIx != nil {
		pre = append(pre, createDestinationIx)
	}

	sourceToken, err := helpers.FindAssociatedTokenAddress(s.PoolAuthority, lpMint, token.ProgramID)
	if err != nil {
		return nil, nil, nil, err
	}

	ix, err = dbcidl.NewMigrateMeteoraDammClaimLpTokenInstruction(
		params.VirtualPool,
		migrationMetadata,
		s.PoolAuthority,
		lpMint,
		sourceToken,
		destinationToken,
		owner,
		params.Payer,
		token.ProgramID,
	)
	if err != nil {
		return nil, nil, nil, err
	}
	return pre, ix, nil, nil
}

// CreateDammV2MigrationMetadata creates migration metadata for DAMM V2.
func (s *MigrationService) CreateDammV2MigrationMetadata(ctx context.Context, params CreateDammV2MigrationMetadataParams) (solanago.Instruction, error) {
	migrationMetadata := helpers.DeriveDammV2MigrationMetadataAddress(params.VirtualPool)
	return dbcidl.NewMigrationDammV2CreateMetadataInstruction(
		params.VirtualPool,
		params.Config,
		migrationMetadata,
		params.Payer,
		system.ProgramID,
		helpers.DeriveDbcEventAuthority(),
		DynamicBondingCurveProgramID,
	)
}

// MigrateToDammV2 builds DAMM V2 migration transaction and returns position NFT keypairs.
func (s *MigrationService) MigrateToDammV2(ctx context.Context, params MigrateToDammV2Params) (MigrateToDammV2Response, error) {
	virtualPoolState, err := s.State.GetPool(ctx, params.VirtualPool)
	if err != nil {
		return MigrateToDammV2Response{}, err
	}

	poolConfigState, err := s.State.GetPoolConfig(ctx, virtualPoolState.Config)
	if err != nil {
		return MigrateToDammV2Response{}, err
	}

	dammPoolAuthority := helpers.DeriveDammV2PoolAuthority()
	dammEventAuthority := helpers.DeriveDammV2EventAuthority()
	migrationMetadata := helpers.DeriveDammV2MigrationMetadataAddress(params.VirtualPool)
	dammPool := helpers.DeriveDammV2PoolAddress(params.DammConfig, virtualPoolState.BaseMint, poolConfigState.QuoteMint)

	firstKP, err := solanago.NewRandomPrivateKey()
	if err != nil {
		return MigrateToDammV2Response{}, err
	}
	secondKP, err := solanago.NewRandomPrivateKey()
	if err != nil {
		return MigrateToDammV2Response{}, err
	}

	firstPosition := helpers.DerivePositionAddress(firstKP.PublicKey())
	firstPositionNftAccount := helpers.DerivePositionNftAccount(firstKP.PublicKey())
	secondPosition := helpers.DerivePositionAddress(secondKP.PublicKey())
	secondPositionNftAccount := helpers.DerivePositionNftAccount(secondKP.PublicKey())

	tokenAVault := helpers.DeriveDammV2TokenVaultAddress(dammPool, virtualPoolState.BaseMint)
	tokenBVault := helpers.DeriveDammV2TokenVaultAddress(dammPool, poolConfigState.QuoteMint)

	tokenBaseProgram := helpers.GetTokenProgram(TokenType(poolConfigState.TokenType))
	tokenQuoteProgram := helpers.GetTokenProgram(TokenType(poolConfigState.QuoteTokenFlag))

	firstPositionVesting := helpers.DeriveDammV2PositionVestingAccount(firstPosition)
	secondPositionVesting := helpers.DeriveDammV2PositionVestingAccount(secondPosition)

	ix, err := dbcidl.NewMigrationDammV2Instruction(
		params.VirtualPool,
		migrationMetadata,
		virtualPoolState.Config,
		s.PoolAuthority,
		dammPool,
		firstKP.PublicKey(),
		firstPositionNftAccount,
		firstPosition,
		secondKP.PublicKey(),
		secondPositionNftAccount,
		secondPosition,
		dammPoolAuthority,
		DammV2ProgramID,
		virtualPoolState.BaseMint,
		poolConfigState.QuoteMint,
		tokenAVault,
		tokenBVault,
		virtualPoolState.BaseVault,
		virtualPoolState.QuoteVault,
		params.Payer,
		tokenBaseProgram,
		tokenQuoteProgram,
		solanago.Token2022ProgramID,
		dammEventAuthority,
		system.ProgramID,
	)
	if err != nil {
		return MigrateToDammV2Response{}, err
	}

	// Append remaining accounts: damm_config, position vesting accounts.
	if inst, ok := ix.(*solanago.GenericInstruction); ok {
		inst.AccountValues = append(inst.AccountValues,
			solanago.NewAccountMeta(params.DammConfig, false, false),
			solanago.NewAccountMeta(firstPositionVesting, true, false),
			solanago.NewAccountMeta(secondPositionVesting, true, false),
		)
	}

	cuIx := computebudget.NewSetComputeUnitLimitInstructionBuilder().SetUnits(600000).Build()
	tx, err := solanago.NewTransaction([]solanago.Instruction{cuIx, ix}, solanago.Hash{}, solanago.TransactionPayer(params.Payer))
	if err != nil {
		return MigrateToDammV2Response{}, err
	}

	return MigrateToDammV2Response{
		Transaction:       tx,
		FirstPositionNFT:  firstKP,
		SecondPositionNFT: secondKP,
	}, nil
}
