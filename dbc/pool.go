package dbc

import (
	"context"
	"fmt"
	"math/big"

	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"
	solanago "github.com/krazyTry/meteora-go/solana"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

func dbcInitializeVirtualPoolWithSplToken(
	m *DBC,
	config,
	poolCreator,
	dbcPool,
	baseMint,
	baseVault,
	quoteMint,
	quoteVault,
	tokenBaseProgram solana.PublicKey,
	tokenQuoteProgram solana.PublicKey,
	payer solana.PublicKey,
	params *dbc.InitializePoolParameters,
) (solana.Instruction, error) {

	poolAuthority := m.poolAuthority
	eventAuthority := m.eventAuthority

	program := dbc.ProgramID
	systemProgram := solana.SystemProgramID
	mintMetadata, err := dbc.DeriveMintMetadataPDA(baseMint)
	if err != nil {
		return nil, err
	}
	metadataProgram := solana.TokenMetadataProgramID

	// tokenQuoteProgram := solana.TokenProgramID
	// tokenProgram := solana.TokenProgramID

	if params == nil {
		return nil, fmt.Errorf("params is nil")
	}

	return dbc.NewInitializeVirtualPoolWithSplTokenInstruction(
		*params,
		config,
		poolAuthority,
		poolCreator,
		baseMint,
		quoteMint,
		dbcPool,
		baseVault,
		quoteVault,
		mintMetadata,
		metadataProgram,
		payer,
		tokenQuoteProgram,
		tokenBaseProgram,
		systemProgram,
		eventAuthority,
		program,
	)
}

func dbcInitializeVirtualPoolWithToken2022(
	m *DBC,
	config,
	poolCreator,
	dbcPool,
	baseMint,
	baseVault,
	quoteMint,
	quoteVault,
	tokenBaseProgram solana.PublicKey,
	tokenQuoteProgram solana.PublicKey,
	payer solana.PublicKey,
	params *dbc.InitializePoolParameters,
) (solana.Instruction, error) {

	poolAuthority := m.poolAuthority
	eventAuthority := m.eventAuthority

	program := dbc.ProgramID
	systemProgram := solana.SystemProgramID

	// tokenQuoteProgram := solana.TokenProgramID
	// tokenProgram := solana.Token2022ProgramID

	if params == nil {
		return nil, fmt.Errorf("params is nil")
	}

	return dbc.NewInitializeVirtualPoolWithToken2022Instruction(
		*params,
		config,
		poolAuthority,
		poolCreator,
		baseMint,
		quoteMint,
		dbcPool,
		baseVault,
		quoteVault,
		payer,
		tokenQuoteProgram,
		tokenBaseProgram,
		systemProgram,
		eventAuthority,
		program,
	)
}

func (m *DBC) CreatePoolInstruction(
	ctx context.Context,
	config *dbc.PoolConfig,
	mintWallet solana.PublicKey,
	payer solana.PublicKey,
	creator solana.PublicKey,
	name string,
	symbol string,
	uri string,
) ([]solana.Instruction, error) {

	baseMint := mintWallet
	quoteMint := solana.WrappedSol

	tokenBaseProgram := dbc.GetTokenProgram(config.TokenType)
	tokenQuoteProgram := solana.TokenProgramID

	pool, err := dbc.DeriveDbcPoolPDA(quoteMint, baseMint, m.config.PublicKey())
	if err != nil {
		return nil, err
	}
	baseVault, err := dbc.DeriveTokenVaultPDA(pool, baseMint)
	if err != nil {
		return nil, err
	}
	quoteVault, err := dbc.DeriveTokenVaultPDA(pool, quoteMint)
	if err != nil {
		return nil, err
	}

	params := &dbc.InitializePoolParameters{
		Name:   name,
		Symbol: symbol,
		Uri:    uri,
	}

	var createPoolIx solana.Instruction

	switch config.TokenType {
	case 0:
		if createPoolIx, err = dbcInitializeVirtualPoolWithSplToken(m,
			m.config.PublicKey(),
			creator,
			pool,
			baseMint,
			baseVault,
			quoteMint,
			quoteVault,
			tokenBaseProgram,
			tokenQuoteProgram,
			payer,
			params,
		); err != nil {
			return nil, err
		}
	case 1:
		if createPoolIx, err = dbcInitializeVirtualPoolWithToken2022(m,
			m.config.PublicKey(),
			creator,
			pool,
			baseMint,
			baseVault,
			quoteMint,
			quoteVault,
			tokenBaseProgram,
			tokenQuoteProgram,
			payer,
			params,
		); err != nil {
			return nil, err
		}
	}
	return []solana.Instruction{createPoolIx}, nil
}

func (m *DBC) CreatePool(
	ctx context.Context,
	mintWallet *solana.Wallet,
	payer *solana.Wallet,
	name string,
	symbol string,
	uri string,
) (string, error) {
	config, err := m.GetConfig(ctx, m.config.PublicKey())
	if err != nil {
		return "", err
	}

	instructions, err := m.CreatePoolInstruction(ctx,
		config,
		mintWallet.PublicKey(),
		payer.PublicKey(),
		m.poolCreator.PublicKey(),
		name,
		symbol,
		uri,
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
			case key.Equals(m.poolCreator.PublicKey()):
				return &m.poolCreator.PrivateKey
			case key.Equals(mintWallet.PublicKey()):
				return &mintWallet.PrivateKey
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

func (m *DBC) CreatePoolWithFirstBuInstruction(
	ctx context.Context,
	config *dbc.PoolConfig,
	mintWallet solana.PublicKey,
	payer solana.PublicKey,
	creator solana.PublicKey,
	name string,
	symbol string,
	uri string,
	buyer solana.PublicKey,
	amountIn *big.Int,
	slippageBps uint64, //  250 = 2.5%
) ([]solana.Instruction, error) {

	baseMint := mintWallet
	quoteMint := solana.WrappedSol

	tokenBaseProgram := dbc.GetTokenProgram(config.TokenType)
	tokenQuoteProgram := solana.TokenProgramID

	minOut, err := dbc.GetSwapAmountFromQuote(config, amountIn, slippageBps)
	if err != nil {
		return nil, err
	}

	pool, err := dbc.DeriveDbcPoolPDA(quoteMint, baseMint, m.config.PublicKey())
	if err != nil {
		return nil, err
	}

	baseVault, err := dbc.DeriveTokenVaultPDA(pool, baseMint)
	if err != nil {
		return nil, err
	}

	quoteVault, err := dbc.DeriveTokenVaultPDA(pool, quoteMint)
	if err != nil {
		return nil, err
	}

	params := &dbc.InitializePoolParameters{
		Name:   name,
		Symbol: symbol,
		Uri:    uri,
	}

	var instructions []solana.Instruction

	var createPoolIx solana.Instruction
	switch config.TokenType {
	case dbc.TokenTypeSPL:

		if createPoolIx, err = dbcInitializeVirtualPoolWithSplToken(m,
			m.config.PublicKey(),
			creator,
			pool,
			baseMint,
			baseVault,
			quoteMint,
			quoteVault,
			tokenBaseProgram,
			tokenQuoteProgram,
			payer,
			params,
		); err != nil {
			return nil, err
		}

	case dbc.TokenTypeToken2022:

		if createPoolIx, err = dbcInitializeVirtualPoolWithToken2022(m,
			m.config.PublicKey(),
			creator,
			pool,
			baseMint,
			baseVault,
			quoteMint,
			quoteVault,
			tokenBaseProgram,
			tokenQuoteProgram,
			payer,
			params,
		); err != nil {
			return nil, err
		}

	}
	instructions = append(instructions, createPoolIx)

	userInputTokenAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, buyer, quoteMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	userOutputTokenAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, buyer, baseMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	// wrap SOL by transferring lamports into the WSOL ATA
	wrapSOLIx := system.NewTransferInstruction(
		amountIn.Uint64(),
		buyer,
		userInputTokenAccount,
	).Build()

	instructions = append(instructions, wrapSOLIx)

	// sync the WSOL account to update its balance
	syncNativeIx := token.NewSyncNativeInstruction(
		userInputTokenAccount,
	).Build()

	instructions = append(instructions, syncNativeIx)

	currentPoint, err := solanago.CurrenPoint(ctx, m.rpcClient, uint8(config.ActivationType))
	if err != nil {
		return nil, err
	}

	isRateLimiterApplied := dbc.CheckRateLimiterApplied(
		config.PoolFees.BaseFee.BaseFeeMode,
		false,
		currentPoint,
		new(big.Int).SetUint64(0),
		new(big.Int).SetUint64(config.PoolFees.BaseFee.SecondFactor),
	)

	var remainingAccounts []*solana.AccountMeta
	if isRateLimiterApplied {
		remainingAccounts = []*solana.AccountMeta{
			solana.NewAccountMeta(solana.SysVarInstructionsPubkey, false, false),
		}
	}

	swapIx, err := dbcSwap(m,
		m.config.PublicKey(),
		pool,
		baseMint,
		quoteMint,
		baseVault,
		quoteVault,
		payer,
		solana.PublicKey{},
		userInputTokenAccount,
		userOutputTokenAccount,
		tokenBaseProgram,
		tokenQuoteProgram,
		amountIn.Uint64(),
		minOut.Uint64(),
		remainingAccounts,
	)

	if err != nil {
		return nil, err
	}

	instructions = append(instructions, swapIx)

	// close the WSOL account after swap to recover rent
	closeWSOLIx := token.NewCloseAccountInstruction(
		userInputTokenAccount,
		buyer,
		buyer,
		[]solana.PublicKey{},
	).Build()

	instructions = append(instructions, closeWSOLIx)

	return instructions, nil
}

func (m *DBC) CreatePoolWithFirstBuy(
	ctx context.Context,
	mintWallet *solana.Wallet,
	payerAndBuyer *solana.Wallet,
	name string,
	symbol string,
	uri string,
	// buyer *solana.Wallet,
	amountIn *big.Int,
	slippageBps uint64, // 250 = 2.5%
) (string, error) {

	t := new(big.Int).Sub(amountIn, new(big.Int).SetUint64(2039280)) // minimum rent-exempt balance for WSOL account
	// amountIn = uint64(1e6)
	if t.Cmp(big.NewInt(1e6)) < 0 {
		return "", fmt.Errorf("amountIn must be greater than 1e6 + 2039280")
	}

	payer := payerAndBuyer
	buyer := payerAndBuyer

	config, err := m.GetConfig(ctx, m.config.PublicKey())
	if err != nil {
		return "", err
	}

	instructions, err := m.CreatePoolWithFirstBuInstruction(ctx,
		config,
		mintWallet.PublicKey(),
		payer.PublicKey(),
		m.poolCreator.PublicKey(),
		name,
		symbol,
		uri,
		buyer.PublicKey(),
		amountIn,
		slippageBps,
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
			case key.Equals(payerAndBuyer.PublicKey()):
				return &payerAndBuyer.PrivateKey
			case key.Equals(m.poolCreator.PublicKey()):
				return &m.poolCreator.PrivateKey
			case key.Equals(mintWallet.PublicKey()):
				return &mintWallet.PrivateKey
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

func (m *DBC) GetPoolsByConfig(ctx context.Context) ([]*dbc.VirtualPool, error) {
	opt := solanago.GenProgramAccountFilter(
		dbc.AccountKeyPoolConfig,
		&solanago.Filter{
			Owner:  m.config.PublicKey(),
			Offset: 72,
		},
	)

	outs, err := m.rpcClient.GetProgramAccountsWithOpts(ctx, dbc.ProgramID, opt)
	if err != nil {
		if err == rpc.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}

	var list []*dbc.VirtualPool
	for _, out := range outs {
		obj, err := dbc.ParseAnyAccount(out.Account.Data.GetBinary())
		if err != nil {
			return nil, err
		}
		cfg, ok := obj.(*dbc.VirtualPool)
		if !ok {
			return nil, fmt.Errorf("obj.(*dbc.PoolConfig) fail")
		}
		list = append(list, cfg)
	}

	return list, nil
}

func (m *DBC) GetPoolsByCreator(ctx context.Context) ([]*dbc.VirtualPool, error) {
	opt := solanago.GenProgramAccountFilter(
		dbc.AccountKeyVirtualPool,
		&solanago.Filter{
			Owner:  m.poolCreator.PublicKey(),
			Offset: 104,
		},
	)
	outs, err := m.rpcClient.GetProgramAccountsWithOpts(ctx, dbc.ProgramID, opt)
	if err != nil {
		if err == rpc.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}

	var list []*dbc.VirtualPool
	for _, out := range outs {
		obj, err := dbc.ParseAnyAccount(out.Account.Data.GetBinary())
		if err != nil {
			return nil, err
		}
		cfg, ok := obj.(*dbc.VirtualPool)
		if !ok {
			return nil, fmt.Errorf("obj.(*dbc.PoolConfig) fail")
		}
		list = append(list, cfg)
	}

	return list, nil
}

func (m *DBC) GetPoolByBaseMint(ctx context.Context, baseMint solana.PublicKey) (*dbc.VirtualPool, error) {
	opt := solanago.GenProgramAccountFilter(
		dbc.AccountKeyVirtualPool,
		&solanago.Filter{
			Owner:  baseMint,
			Offset: 136,
		},
	)

	outs, err := m.rpcClient.GetProgramAccountsWithOpts(ctx, dbc.ProgramID, opt)
	if err != nil {
		if err == rpc.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}

	if len(outs) == 0 {
		return nil, nil
	}

	out := outs[0]
	obj, err := dbc.ParseAnyAccount(out.Account.Data.GetBinary())
	if err != nil {
		return nil, err
	}

	cfg, ok := obj.(*dbc.VirtualPool)
	if !ok {
		return nil, fmt.Errorf("obj.(*dbc.PoolConfig) fail")
	}

	return cfg, nil
}
