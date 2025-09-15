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
	"github.com/gagliardetto/solana-go/rpc/ws"
)

// CreateLockerInstruction generates the instruction for locking a dbc pool.
//
// Example:
//
// poolState, _ := m.GetPoolByBaseMint(ctx, baseMint)
//
// configState, _ := m.GetConfig(ctx, poolState.Config)
//
// instructions, err := CreateLockerInstruction(
//
//	ctx,
//	m.rpcClient,
//	payer.PublicKey(), // payer account
//	poolState.Creator, // creator
//	poolState.Address, // dbc pool address
//	poolState.VirtualPool,// dbc pool state
//	configState, // dbc pool config
//
// )
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

	baseMint := poolState.BaseMint
	baseVault := poolState.BaseVault

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

// CreateLocker creates a new locker account when migrating from the Dynamic Bonding Curve to DAMM V1 or DAMM V2.
// This function is called when lockedVestingParam is enabled in the config key.
// This function is blocking and will wait for on-chain confirmation before returning.
//
// It is used for manually migrating dbc to damm.v2 and is the first step in the migration process.
//
// Example:
//
// sig, _ := meteoraDBC.CreateLocker(
//
//	ctx,
//	wsClient,
//	payer,
//	baseMint, // baseMintToken
//
// )
func (m *DBC) CreateLocker(
	ctx context.Context,
	wsClient *ws.Client,
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
		poolState.Creator,
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
		wsClient,
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

// MigrationDammV2CreateMetadataInstruction generates the instruction for creating Metadata.
//
// Example:
//
// poolState, _ := m.GetPoolByBaseMint(ctx, baseMint)
//
// instructions, _ := MigrationDammV2CreateMetadataInstruction(
//
//	ctx,
//	payer.PublicKey(),
//	poolState.Address, // pool address
//	poolState.VirtualPool, // dbc pool state
//
// )
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

// MigrationDammV2CreateMetadata creates a new DAMM V2 migration metadata account.
// This function is blocking and will wait for on-chain confirmation before returning.
//
// It is used for manually migrating dbc to damm.v2 and is the second step in the migration process.
//
// Example:
//
// sig, _ := meteoraDBC.MigrationDammV2CreateMetadata(
//
//	ctx,
//	wsClient,
//	payer,
//	baseMint, // baseMintToken
//
// )
func (m *DBC) MigrationDammV2CreateMetadata(
	ctx context.Context,
	wsClient *ws.Client,
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
		wsClient,
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

// MigrationDammV2Instruction generates the instruction needed for migrating to dammv2.
//
// Example:
//
// poolState, _ := m.GetPoolByBaseMint(ctx, baseMint)
//
// configState, _ := m.GetConfig(ctx, poolState.Config)
//
// partnerPositionNft := solana.NewWallet()
// creatorPositionNft := solana.NewWallet()
//
// instructions, _ := MigrationDammV2Instruction(
//
//	ctx,
//	payer.PublicKey(), // payer account
//	poolState.Address, // dbc pool address
//	poolState.VirtualPool, // dbc pool state
//	configState, // dbc pool config
//	partnerPositionNft,
//	creatorPositionNft,
//
// )
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

	quoteMint := configState.QuoteMint
	baseMint := poolState.BaseMint
	baseVault := poolState.BaseVault
	quoteVault := poolState.QuoteVault

	migrationMetadata, err := dbc.DeriveDammV2MigrationMetadataPDA(poolAddress)
	if err != nil {
		return nil, err
	}

	dammConfig := dbc.GetDammV2Config(configState.MigrationFeeOption)

	dammPoolAddress, err := dbc.DeriveDammV2PoolPDA(dammConfig, baseMint, quoteMint)
	if err != nil {
		return nil, err
	}

	partnerPosition, err := dbc.DerivePosition(partnerPositionNft.PublicKey())
	if err != nil {
		return nil, err
	}

	partnerPositionNftAccount, err := dbc.DerivePositionNftAccount(partnerPositionNft.PublicKey())
	if err != nil {
		return nil, err
	}

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

// MigrationDammV2 migrates the Dynamic Bonding Curve pool to DAMM V2.
// This function is blocking and will wait for on-chain confirmation before returning.
//
// The migration process consists of three steps:
// 1. CreateLocker.
// 2. MigrationDammV2CreateMetadata.
// 3. MigrationDammV2.
//
// partnerPositionNft is the partner's position in dammv2, which will be used later to claim the position fee.
//
// creatorPositionNft is the creator's position in dammv2, which will be used later to claim the position fee.
//
// Example:
//
// sig, partnerPositionNft, creatorPositionNft, _ := meteoraDBC.MigrationDammV2(
//
//	ctx,
//	wsClient,
//	payer,
//	baseMint,
//
// )
func (m *DBC) MigrationDammV2(
	ctx context.Context,
	wsClient *ws.Client,
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
		wsClient,
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
