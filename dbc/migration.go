package dbc

import (
	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"
	solanago "github.com/krazyTry/meteora-go/solana"

	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"
	"github.com/krazyTry/meteora-go/locker/locker"

	"context"

	"github.com/gagliardetto/solana-go"
	computebudget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/rpc"
)

func CreateLockerInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	payer solana.PublicKey,
	poolCreator solana.PublicKey,
	poolAddress solana.PublicKey,
	poolState *dbc.VirtualPool,
	configState *dbc.PoolConfig,
) ([]solana.Instruction, error) {

	switch poolState.MigrationProgress {
	case dbc.MigrationProgressLockedVesting:
		return nil, ErrDammV2LockerNotRequired
	case dbc.MigrationProgressPostBondingCurve:
	default:
		return nil, ErrMigrationProgressState
	}

	baseMint := poolState.BaseMint   // baseMint
	baseVault := poolState.BaseVault // dbc.DeriveTokenVaultPDA(pool, virtualPool.BaseMint)

	base, err := dbc.DeriveBaseKeyForLocker(poolAddress)
	if err != nil {
		return nil, err
	}

	escrow, err := dbc.DeriveEscrow(base)
	if err != nil {
		return nil, err
	}

	var instructions []solana.Instruction
	tokenEscrowAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, escrow, baseMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	lockerIx, err := dbc.NewCreateLockerInstruction(
		poolAddress,
		poolState.Config,
		poolAuthority,
		baseVault,
		baseMint,
		base,
		poolCreator,
		escrow,
		tokenEscrowAccount,
		payer,
		dbc.GetTokenProgram(configState.TokenType),
		locker.ProgramID,
		lockerEventAuthority,
		solana.SystemProgramID,
	)
	if err != nil {
		return nil, err
	}
	instructions = append(instructions, lockerIx)

	return instructions, nil
}

func (m *DBC) CreateLocker(
	ctx context.Context,
	payer *solana.Wallet,
	baseMint solana.PublicKey,
) (string, error) {
	poolState, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}

	configState, err := m.GetConfig(ctx, poolState.Config)
	if err != nil {
		return "", err
	}

	instructions, err := CreateLockerInstruction(
		ctx,
		m.rpcClient,
		payer.PublicKey(),
		m.poolCreator.PublicKey(),
		poolState.Address,
		poolState.VirtualPool,
		configState,
	)

	if err != nil {
		if err == ErrDammV2LockerNotRequired {
			return "***************************************************************************************", nil
		}
		return "", err
	}

	sig, err := solanago.SendTransaction(ctx,
		m.rpcClient,
		m.wsClient,
		instructions,
		payer.PublicKey(),
		func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(payer.PublicKey()):
				return &payer.PrivateKey
			default:
				return nil
			}
		},
	)
	if err != nil {
		return "", err
	}
	return sig.String(), nil
}

func MigrationDammV2CreateMetadataInstruction(
	ctx context.Context,
	payer solana.PublicKey,
	poolAddress solana.PublicKey,
	poolState *dbc.VirtualPool,
) ([]solana.Instruction, error) {
	switch poolState.MigrationProgress {
	case dbc.MigrationProgressCreatedPool:
		return nil, ErrMigrationProgressState
	default:
	}

	migrationMetadata, err := dbc.DeriveDammV2MigrationMetadataPDA(poolAddress)
	if err != nil {
		return nil, err
	}

	var instructions []solana.Instruction

	migrationIx, err := dbc.NewMigrationDammV2CreateMetadataInstruction(
		poolAddress,
		poolState.Config,
		migrationMetadata,
		payer,
		solana.SystemProgramID,
		eventAuthority,
		dbc.ProgramID,
	)
	if err != nil {
		return nil, err
	}

	instructions = append(instructions, migrationIx)
	return instructions, nil
}

func (m *DBC) MigrationDammV2CreateMetadata(
	ctx context.Context,
	payer *solana.Wallet,
	baseMint solana.PublicKey,
) (string, error) {

	poolState, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}

	instructions, err := MigrationDammV2CreateMetadataInstruction(
		ctx,
		payer.PublicKey(),
		poolState.Address,
		poolState.VirtualPool,
	)
	if err != nil {
		return "", err
	}

	sig, err := solanago.SendTransaction(ctx,
		m.rpcClient,
		m.wsClient,
		instructions,
		payer.PublicKey(),
		func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(payer.PublicKey()):
				return &payer.PrivateKey
			default:
				return nil
			}
		},
	)
	if err != nil {
		return "", err
	}
	return sig.String(), nil
}

func MigrationDammV2Instruction(
	ctx context.Context,
	payer solana.PublicKey,
	poolAddress solana.PublicKey,
	poolState *dbc.VirtualPool,
	configState *dbc.PoolConfig,
	partnerPositionNft *solana.Wallet,
	creatorPositionNft *solana.Wallet,
) ([]solana.Instruction, error) {

	switch poolState.MigrationProgress {
	case dbc.MigrationProgressLockedVesting:
	default:
		return nil, ErrMigrationProgressState
	}

	quoteMint := configState.QuoteMint // solana.WrappedSol
	baseMint := poolState.BaseMint     // baseMint
	baseVault := poolState.BaseVault   // dbc.DeriveTokenVaultPDA(pool, virtualPool.BaseMint)
	quoteVault := poolState.QuoteVault // dbc.DeriveTokenVaultPDA(pool, config.QuoteMint)

	migrationMetadata, err := dbc.DeriveDammV2MigrationMetadataPDA(poolAddress)
	if err != nil {
		return nil, err
	}

	dammConfig := dbc.GetDammV2Config(configState.MigrationFeeOption)

	dammPoolAddress, err := dbc.DeriveDammV2PoolPDA(dammConfig, baseMint, quoteMint)
	if err != nil {
		return nil, err
	}

	// partnerPositionNft := solana.NewWallet()

	partnerPosition, err := dbc.DerivePosition(partnerPositionNft.PublicKey())
	if err != nil {
		return nil, err
	}

	partnerPositionNftAccount, err := dbc.DerivePositionNftAccount(partnerPositionNft.PublicKey())
	if err != nil {
		return nil, err
	}

	// creatorPositionNft := solana.NewWallet()

	creatorPosition, err := dbc.DerivePosition(creatorPositionNft.PublicKey())
	if err != nil {
		return nil, err
	}

	creatorPositionNftAccount, err := dbc.DerivePositionNftAccount(creatorPositionNft.PublicKey())
	if err != nil {
		return nil, err
	}

	tokenBaseVault, err := dbc.DeriveDammV2TokenVaultPDA(dammPoolAddress, baseMint)
	if err != nil {
		return nil, err
	}

	tokenQuoteVault, err := dbc.DeriveDammV2TokenVaultPDA(dammPoolAddress, quoteMint)
	if err != nil {
		return nil, err
	}

	var instructions []solana.Instruction

	migrationIx, err := dbc.NewMigrationDammV2Instruction(
		poolAddress,
		migrationMetadata,
		poolState.Config,
		poolAuthority,
		dammPoolAddress,
		partnerPositionNft.PublicKey(),
		partnerPositionNftAccount,
		partnerPosition,
		creatorPositionNft.PublicKey(),
		creatorPositionNftAccount,
		creatorPosition,
		dammPoolAuthority,
		cp_amm.ProgramID,
		baseMint,
		quoteMint,
		tokenBaseVault,
		tokenQuoteVault,
		baseVault,
		quoteVault,
		payer,
		dbc.GetTokenProgram(configState.TokenType),
		dbc.GetTokenProgram(configState.QuoteTokenFlag),
		solana.Token2022ProgramID,
		dammEventAuthority,
		solana.SystemProgramID,
		[]*solana.AccountMeta{
			solana.NewAccountMeta(dammConfig, false, false),
		},
	)
	if err != nil {
		return nil, err
	}

	instructions = append(instructions, migrationIx)
	return instructions, nil
}

func (m *DBC) MigrationDammV2(
	ctx context.Context,
	payer *solana.Wallet,
	baseMint solana.PublicKey,
) (string, *solana.Wallet, *solana.Wallet, error) {
	poolState, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", nil, nil, err
	}

	configState, err := m.GetConfig(ctx, poolState.Config)
	if err != nil {
		return "", nil, nil, err
	}

	partnerPositionNft := solana.NewWallet()
	creatorPositionNft := solana.NewWallet()

	instructions, err := MigrationDammV2Instruction(
		ctx,
		payer.PublicKey(),
		poolState.Address,
		poolState.VirtualPool,
		configState,
		partnerPositionNft,
		creatorPositionNft,
	)
	if err != nil {
		return "", nil, nil, err
	}

	sig, err := solanago.SendTransaction(ctx,
		m.rpcClient,
		m.wsClient,
		append([]solana.Instruction{
			computebudget.NewSetComputeUnitLimitInstruction(500_000).Build(),
		}, instructions...),
		payer.PublicKey(),
		func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(payer.PublicKey()):
				return &payer.PrivateKey
			case key.Equals(partnerPositionNft.PublicKey()):
				return &partnerPositionNft.PrivateKey
			case key.Equals(creatorPositionNft.PublicKey()):
				return &creatorPositionNft.PrivateKey
			default:
				return nil
			}
		},
	)
	if err != nil {
		return "", nil, nil, err
	}

	return sig.String(), partnerPositionNft, creatorPositionNft, nil
}
