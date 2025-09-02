package dbc

import (
	"fmt"

	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"
	solanago "github.com/krazyTry/meteora-go/solana"

	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"
	"github.com/krazyTry/meteora-go/locker/locker"

	"context"

	"github.com/gagliardetto/solana-go"
	computebudget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/rpc"
	sendandconfirmtransaction "github.com/gagliardetto/solana-go/rpc/sendAndConfirmTransaction"
)

func dbcCreateLocker(m *DBC,
	dbcPool solana.PublicKey,
	config solana.PublicKey,
	baseVault solana.PublicKey,
	baseMint solana.PublicKey,
	base solana.PublicKey,
	creator solana.PublicKey,
	escrow solana.PublicKey,
	escrowToken solana.PublicKey,
	payer solana.PublicKey,
	tokenProgram solana.PublicKey,
) (solana.Instruction, error) {

	poolAuthority := m.poolAuthority
	lockerProgram := locker.ProgramID
	lockerEventAuthority := m.lockerEventAuthority
	systemProgram := solana.SystemProgramID

	return dbc.NewCreateLockerInstruction(
		dbcPool,
		config,
		poolAuthority,
		baseVault,
		baseMint,
		base,
		creator,
		escrow,
		escrowToken,
		payer,
		tokenProgram,
		lockerProgram,
		lockerEventAuthority,
		systemProgram,
	)
}

func (m *DBC) CreateLockerInstruction(
	ctx context.Context,
	payer solana.PublicKey,
	virtualPool *dbc.VirtualPool,
	config *dbc.PoolConfig,
) ([]solana.Instruction, error) {

	if config.LockedVestingConfig.AmountPerPeriod <= 0 {
		return nil, errDammV2LockerExist
	}

	if virtualPool.MigrationProgress == dbc.MigrationProgressPreBondingCurve {
		return nil, fmt.Errorf("MigrationProgress = PreBondingCurve")
	}

	quoteMint := config.QuoteMint      // solana.WrappedSol
	baseMint := virtualPool.BaseMint   // baseMint
	baseVault := virtualPool.BaseVault // dbc.DeriveTokenVaultPDA(pool, virtualPool.BaseMint)

	pool, err := dbc.DeriveDbcPoolPDA(quoteMint, baseMint, virtualPool.Config)
	if err != nil {
		return nil, err
	}

	base, err := dbc.DeriveBaseKeyForLocker(pool)
	if err != nil {
		return nil, err
	}

	escrow, err := dbc.DeriveEscrow(base)
	if err != nil {
		return nil, err
	}

	exists, err := solanago.GetAccountInfo(ctx, m.rpcClient, escrow)
	if err != nil && err != rpc.ErrNotFound {
		return nil, err
	}

	if exists != nil {
		return nil, errDammV2LockerExist
	}

	var instructions []solana.Instruction
	tokenEscrowAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, escrow, baseMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	tokenBaseProgram := dbc.GetTokenProgram(config.TokenType)

	lockerIx, err := dbcCreateLocker(m,
		pool,
		virtualPool.Config,
		baseVault,
		baseMint,
		base,
		m.poolCreator.PublicKey(),
		escrow,
		tokenEscrowAccount,
		payer,
		tokenBaseProgram,
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
	virtualPool, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}

	if virtualPool.MigrationProgress != dbc.MigrationProgressLockedVesting {
		return "", fmt.Errorf("virtualPool.MigrationProgress != dbc.MigrationProgressLockedVesting")
	}

	// poolState.quoteReserve >= poolConfig.migrationQuoteThreshold
	config, err := m.GetConfig(ctx, virtualPool.Config)
	if err != nil {
		return "", err
	}

	instructions, err := m.CreateLockerInstruction(ctx, payer.PublicKey(), virtualPool, config)
	if err != nil {
		if err == errDammV2LockerExist {
			return "*************", nil
		}
		return "", err
	}

	latestBlockhash, err := solanago.GetLatestBlockhash(ctx, m.rpcClient)
	if err != nil {
		return "", err
	}

	tx, err := solana.NewTransaction(instructions, latestBlockhash, solana.TransactionPayer(payer.PublicKey()))
	if err != nil {
		return "", err
	}

	if _, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(payer.PublicKey()):
			return &payer.PrivateKey
		default:
			return nil
		}
	}); err != nil {
		return "", err
	}

	if m.bSimulate {
		if _, err = m.rpcClient.SimulateTransactionWithOpts(
			ctx,
			tx,
			&rpc.SimulateTransactionOpts{
				SigVerify:  false,
				Commitment: rpc.CommitmentFinalized,
			}); err != nil {
			return "", err
		}
		return "-", nil
	}
	sig, err := m.rpcClient.SendTransactionWithOpts(
		ctx,
		tx,
		rpc.TransactionOpts{
			SkipPreflight:       false,
			PreflightCommitment: rpc.CommitmentFinalized,
		},
	)
	if err != nil {
		return "", err
	}

	if _, err = sendandconfirmtransaction.WaitForConfirmation(ctx, m.wsClient, sig, nil); err != nil {
		return "", err
	}
	return sig.String(), nil
}

func dbcMigrationDammV2CreateMetadata(m *DBC,
	dbcPool solana.PublicKey,
	config solana.PublicKey,
	migrationMetadata solana.PublicKey,
	payer solana.PublicKey,
) (solana.Instruction, error) {
	eventAuthority := m.eventAuthority
	systemProgram := solana.SystemProgramID
	program := dbc.ProgramID

	return dbc.NewMigrationDammV2CreateMetadataInstruction(
		dbcPool,
		config,
		migrationMetadata,
		payer,
		systemProgram,
		eventAuthority,
		program,
	)
}

func (m *DBC) MigrationDammV2CreateMetadataInstruction(
	ctx context.Context,
	payer solana.PublicKey,
	virtualPool *dbc.VirtualPool,
	config *dbc.PoolConfig,
) ([]solana.Instruction, error) {
	quoteMint := config.QuoteMint    // solana.WrappedSol
	baseMint := virtualPool.BaseMint // baseMint

	pool, err := dbc.DeriveDbcPoolPDA(quoteMint, baseMint, virtualPool.Config)
	if err != nil {
		return nil, err
	}

	migrationMetadata, err := dbc.DeriveDammV2MigrationMetadataPDA(pool)
	if err != nil {
		return nil, err
	}

	exists, err := solanago.GetAccountInfo(ctx, m.rpcClient, migrationMetadata)
	if err != nil && err != rpc.ErrNotFound {
		return nil, err
	}

	if exists != nil {
		return nil, errDammV2MetadataExist
	}

	var instructions []solana.Instruction

	migrationIx, err := dbcMigrationDammV2CreateMetadata(m,
		pool,
		virtualPool.Config,
		migrationMetadata,
		payer,
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

	virtualPool, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}

	if virtualPool.MigrationProgress != dbc.MigrationProgressLockedVesting {
		return "", fmt.Errorf("virtualPool.MigrationProgress != dbc.MigrationProgressLockedVesting")
	}

	config, err := m.GetConfig(ctx, virtualPool.Config)
	if err != nil {
		return "", err
	}

	instructions, err := m.MigrationDammV2CreateMetadataInstruction(ctx, payer.PublicKey(), virtualPool, config)
	if err != nil {
		if err == errDammV2MetadataExist {
			return "*************", nil
		}
		return "", err
	}

	latestBlockhash, err := solanago.GetLatestBlockhash(ctx, m.rpcClient)
	if err != nil {
		return "", err
	}

	tx, err := solana.NewTransaction(instructions, latestBlockhash, solana.TransactionPayer(payer.PublicKey()))
	if err != nil {
		return "", err
	}

	if _, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(payer.PublicKey()):
			return &payer.PrivateKey
		default:
			return nil
		}
	}); err != nil {
		return "", err
	}

	if m.bSimulate {
		if _, err = m.rpcClient.SimulateTransactionWithOpts(
			ctx,
			tx,
			&rpc.SimulateTransactionOpts{
				SigVerify:  false,
				Commitment: rpc.CommitmentFinalized,
			}); err != nil {
			return "", err
		}
		return "-", nil
	}
	sig, err := m.rpcClient.SendTransactionWithOpts(
		ctx,
		tx,
		rpc.TransactionOpts{
			SkipPreflight:       false,
			PreflightCommitment: rpc.CommitmentFinalized,
		},
	)
	if err != nil {
		return "", err
	}

	if _, err = sendandconfirmtransaction.WaitForConfirmation(ctx, m.wsClient, sig, nil); err != nil {
		return "", err
	}
	return sig.String(), nil
}

func dbcMigrationDammV2(m *DBC,
	dbcPool solana.PublicKey,
	migrationMetadata solana.PublicKey,
	config solana.PublicKey,
	dammPool solana.PublicKey,
	firstPositionNftMint solana.PublicKey,
	firstPositionNftAccount solana.PublicKey,
	firstPosition solana.PublicKey,
	secondPositionNftMint solana.PublicKey,
	secondPositionNftAccount solana.PublicKey,
	secondPosition solana.PublicKey,
	baseMint solana.PublicKey,
	quoteMint solana.PublicKey,
	tokenAVault solana.PublicKey,
	tokenBVault solana.PublicKey,
	baseVault solana.PublicKey,
	quoteVault solana.PublicKey,
	payer solana.PublicKey,
	tokenBaseProgram solana.PublicKey,
	tokenQuoteProgram solana.PublicKey,
	remainingAccounts []*solana.AccountMeta,
) (solana.Instruction, error) {

	dammPoolAuthority := m.dammPoolAuthority
	dammEventAuthority := m.dammEventAuthority
	poolAuthority := m.poolAuthority
	systemProgram := solana.SystemProgramID
	ammProgram := cp_amm.ProgramID
	token2022Program := solana.Token2022ProgramID

	return dbc.NewMigrationDammV2Instruction(
		dbcPool,
		migrationMetadata,
		config,
		poolAuthority,
		dammPool,
		firstPositionNftMint,
		firstPositionNftAccount,
		firstPosition,
		secondPositionNftMint,
		secondPositionNftAccount,
		secondPosition,
		dammPoolAuthority,
		ammProgram,
		baseMint,
		quoteMint,
		tokenAVault,
		tokenBVault,
		baseVault,
		quoteVault,
		payer,
		tokenBaseProgram,
		tokenQuoteProgram,
		token2022Program,
		dammEventAuthority,
		systemProgram,
		remainingAccounts,
	)
}

func (m *DBC) MigrationDammV2Instruction(
	ctx context.Context,
	payer solana.PublicKey,
	virtualPool *dbc.VirtualPool,
	config *dbc.PoolConfig,
	partnerPositionNft *solana.Wallet,
	creatorPositionNft *solana.Wallet,
) ([]solana.Instruction, error) {

	quoteMint := config.QuoteMint        // solana.WrappedSol
	baseMint := virtualPool.BaseMint     // baseMint
	baseVault := virtualPool.BaseVault   // dbc.DeriveTokenVaultPDA(pool, virtualPool.BaseMint)
	quoteVault := virtualPool.QuoteVault // dbc.DeriveTokenVaultPDA(pool, config.QuoteMint)

	pool, err := dbc.DeriveDbcPoolPDA(quoteMint, baseMint, virtualPool.Config)
	if err != nil {
		return nil, err
	}

	migrationMetadata, err := dbc.DeriveDammV2MigrationMetadataPDA(pool)
	if err != nil {
		return nil, err
	}

	dammConfig := dbc.GetDammV2Config(config.MigrationFeeOption)

	dammPool, err := dbc.DeriveDammV2PoolPDA(dammConfig, baseMint, quoteMint)
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

	tokenBaseVault, err := dbc.DeriveDammV2TokenVaultPDA(dammPool, baseMint)
	if err != nil {
		return nil, err
	}

	tokenQuoteVault, err := dbc.DeriveDammV2TokenVaultPDA(dammPool, quoteMint)
	if err != nil {
		return nil, err
	}

	tokenBaseProgram := dbc.GetTokenProgram(config.TokenType)

	tokenQuoteProgram := dbc.GetTokenProgram(config.QuoteTokenFlag)

	var instructions []solana.Instruction

	migrationIx, err := dbcMigrationDammV2(m,
		pool,
		migrationMetadata,
		virtualPool.Config,
		dammPool,
		partnerPositionNft.PublicKey(),
		partnerPositionNftAccount,
		partnerPosition,
		creatorPositionNft.PublicKey(),
		creatorPositionNftAccount,
		creatorPosition,
		baseMint,
		quoteMint,
		tokenBaseVault,
		tokenQuoteVault,
		baseVault,
		quoteVault,
		payer,
		tokenBaseProgram,
		tokenQuoteProgram,
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
	virtualPool, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", nil, nil, err
	}
	if virtualPool.MigrationProgress != dbc.MigrationProgressLockedVesting {
		return "", nil, nil, fmt.Errorf("virtualPool.MigrationProgress != dbc.MigrationProgressLockedVesting")
	}

	config, err := m.GetConfig(ctx, virtualPool.Config)
	if err != nil {
		return "", nil, nil, err
	}

	partnerPositionNft := solana.NewWallet()
	creatorPositionNft := solana.NewWallet()

	instructions, err := m.MigrationDammV2Instruction(ctx, payer.PublicKey(), virtualPool, config, partnerPositionNft, creatorPositionNft)
	if err != nil {
		return "", nil, nil, err
	}

	latestBlockhash, err := solanago.GetLatestBlockhash(ctx, m.rpcClient)
	if err != nil {
		return "", nil, nil, err
	}

	instructions = append([]solana.Instruction{computebudget.NewSetComputeUnitLimitInstruction(500_000).Build()}, instructions...)

	tx, err := solana.NewTransaction(instructions, latestBlockhash, solana.TransactionPayer(payer.PublicKey()))
	if err != nil {
		return "", nil, nil, err
	}

	if _, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
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
	}); err != nil {
		return "", nil, nil, err
	}

	if m.bSimulate {
		if _, err = m.rpcClient.SimulateTransactionWithOpts(
			ctx,
			tx,
			&rpc.SimulateTransactionOpts{
				SigVerify:  false,
				Commitment: rpc.CommitmentFinalized,
			}); err != nil {
			return "", nil, nil, err
		}
		return "-", nil, nil, nil
	}

	sig, err := m.rpcClient.SendTransactionWithOpts(
		ctx,
		tx,
		rpc.TransactionOpts{
			SkipPreflight:       false,
			PreflightCommitment: rpc.CommitmentFinalized,
		},
	)
	if err != nil {
		return "", nil, nil, err
	}

	if _, err = sendandconfirmtransaction.WaitForConfirmation(ctx, m.wsClient, sig, nil); err != nil {
		return "", nil, nil, err
	}
	return sig.String(), partnerPositionNft, creatorPositionNft, nil
}
