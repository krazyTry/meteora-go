package dbc

import (
	"context"
	"fmt"
	"math/big"

	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"
	solanago "github.com/krazyTry/meteora-go/solana"
	"github.com/shopspring/decimal"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

func CreatePoolInstruction(
	ctx context.Context,
	payer solana.PublicKey,
	poolCreator solana.PublicKey,
	config solana.PublicKey,
	configState *dbc.PoolConfig,
	baseMint solana.PublicKey,
	quoteMint solana.PublicKey,
	tokenBaseProgram solana.PublicKey,
	tokenQuoteProgram solana.PublicKey,
	name string,
	symbol string,
	uri string,
) ([]solana.Instruction, error) {

	poolAddress, err := dbc.DeriveDbcPoolPDA(quoteMint, baseMint, config)
	if err != nil {
		return nil, err
	}

	baseVault, err := dbc.DeriveTokenVaultPDA(poolAddress, baseMint)
	if err != nil {
		return nil, err
	}
	quoteVault, err := dbc.DeriveTokenVaultPDA(poolAddress, quoteMint)
	if err != nil {
		return nil, err
	}

	var createPoolIx solana.Instruction

	switch configState.TokenType {
	case 0:
		mintMetadata, err := dbc.DeriveMintMetadataPDA(baseMint)
		if err != nil {
			return nil, err
		}

		if createPoolIx, err = dbc.NewInitializeVirtualPoolWithSplTokenInstruction(
			dbc.InitializePoolParameters{
				Name:   name,
				Symbol: symbol,
				Uri:    uri,
			},
			config,
			poolAuthority,
			poolCreator,
			baseMint,
			quoteMint,
			poolAddress,
			baseVault,
			quoteVault,
			mintMetadata,
			solana.TokenMetadataProgramID,
			payer,
			tokenQuoteProgram,
			tokenBaseProgram,
			solana.SystemProgramID,
			eventAuthority,
			dbc.ProgramID,
		); err != nil {
			return nil, err
		}
	case 1:
		if createPoolIx, err = dbc.NewInitializeVirtualPoolWithToken2022Instruction(
			dbc.InitializePoolParameters{
				Name:   name,
				Symbol: symbol,
				Uri:    uri,
			},
			config,
			poolAuthority,
			poolCreator,
			baseMint,
			quoteMint,
			poolAddress,
			baseVault,
			quoteVault,
			payer,
			tokenQuoteProgram,
			tokenBaseProgram,
			solana.SystemProgramID,
			eventAuthority,
			dbc.ProgramID,
		); err != nil {
			return nil, err
		}
	}

	return []solana.Instruction{createPoolIx}, nil
}

func (m *DBC) CreatePool(
	ctx context.Context,
	payer *solana.Wallet,
	baseMint *solana.Wallet,
	name string,
	symbol string,
	uri string,
) (string, error) {

	rentExemptFee, err := solanago.GetRentExempt(ctx, m.rpcClient)
	if err != nil {
		return "", err
	}

	lamportsSOL, err := solanago.SOLBalance(ctx, m.rpcClient, payer.PublicKey())
	if err != nil {
		return "", err
	}

	if lamportsSOL < rentExemptFee+transferFee {
		return "", fmt.Errorf("buyer sol must be greater than %v", (rentExemptFee+transferFee)/1e9)
	}

	configState, err := m.GetConfig(ctx, m.config.PublicKey())
	if err != nil {
		return "", err
	}

	instructions, err := CreatePoolInstruction(
		ctx,
		payer.PublicKey(),
		m.poolCreator.PublicKey(),
		m.config.PublicKey(),
		configState,
		baseMint.PublicKey(),
		solana.WrappedSol,
		dbc.GetTokenProgram(configState.TokenType),
		solana.TokenProgramID,
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
			case key.Equals(baseMint.PublicKey()):
				return &baseMint.PrivateKey
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

func CreatePoolWithFirstBuInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	payer solana.PublicKey,
	poolCreator solana.PublicKey,
	config solana.PublicKey,
	configState *dbc.PoolConfig,
	baseMint solana.PublicKey,
	quoteMint solana.PublicKey,
	tokenBaseProgram solana.PublicKey,
	tokenQuoteProgram solana.PublicKey,
	creator solana.PublicKey,
	name string,
	symbol string,
	uri string,
	buyer solana.PublicKey,
	amountIn *big.Int,
	slippageBps uint64, //  250 = 2.5%
) ([]solana.Instruction, error) {

	minOut, err := dbc.GetSwapAmountFromQuote(configState, decimal.NewFromBigInt(amountIn, 0), slippageBps)
	if err != nil {
		return nil, err
	}

	poolAddress, err := dbc.DeriveDbcPoolPDA(quoteMint, baseMint, config)
	if err != nil {
		return nil, err
	}

	baseVault, err := dbc.DeriveTokenVaultPDA(poolAddress, baseMint)
	if err != nil {
		return nil, err
	}

	quoteVault, err := dbc.DeriveTokenVaultPDA(poolAddress, quoteMint)
	if err != nil {
		return nil, err
	}

	var instructions []solana.Instruction

	var createPoolIx solana.Instruction
	switch configState.TokenType {
	case 0:

		mintMetadata, err := dbc.DeriveMintMetadataPDA(baseMint)
		if err != nil {
			return nil, err
		}

		if createPoolIx, err = dbc.NewInitializeVirtualPoolWithSplTokenInstruction(
			dbc.InitializePoolParameters{
				Name:   name,
				Symbol: symbol,
				Uri:    uri,
			},
			config,
			poolAuthority,
			poolCreator,
			baseMint,
			quoteMint,
			poolAddress,
			baseVault,
			quoteVault,
			mintMetadata,
			solana.TokenMetadataProgramID,
			payer,
			tokenQuoteProgram,
			tokenBaseProgram,
			solana.SystemProgramID,
			eventAuthority,
			dbc.ProgramID,
		); err != nil {
			return nil, err
		}
	case 1:

		if createPoolIx, err = dbc.NewInitializeVirtualPoolWithToken2022Instruction(
			dbc.InitializePoolParameters{
				Name:   name,
				Symbol: symbol,
				Uri:    uri,
			},
			config,
			poolAuthority,
			poolCreator,
			baseMint,
			quoteMint,
			poolAddress,
			baseVault,
			quoteVault,
			payer,
			tokenQuoteProgram,
			tokenBaseProgram,
			solana.SystemProgramID,
			eventAuthority,
			dbc.ProgramID,
		); err != nil {
			return nil, err
		}
	}

	instructions = append(instructions, createPoolIx)

	userInputTokenAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, buyer, quoteMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	userOutputTokenAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, buyer, baseMint, payer, &instructions)
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

	currentPoint, err := solanago.CurrenPoint(ctx, rpcClient, uint8(configState.ActivationType))
	if err != nil {
		return nil, err
	}

	isRateLimiterApplied := dbc.CheckRateLimiterApplied(
		configState.PoolFees.BaseFee.BaseFeeMode,
		false,
		decimal.NewFromBigInt(currentPoint, 0),
		decimal.Zero,
		decimal.NewFromUint64(configState.PoolFees.BaseFee.SecondFactor),
	)

	var remainingAccounts []*solana.AccountMeta
	if isRateLimiterApplied {
		remainingAccounts = []*solana.AccountMeta{
			solana.NewAccountMeta(solana.SysVarInstructionsPubkey, false, false),
		}
	}

	params := dbc.SwapParameters{
		AmountIn:         amountIn.Uint64(),
		MinimumAmountOut: minOut.BigInt().Uint64(),
	}

	swapIx, err := dbc.NewSwapInstruction(
		params,
		poolAuthority,
		config,
		poolAddress,
		userInputTokenAccount,
		userOutputTokenAccount,
		baseVault,
		quoteVault,
		baseMint,
		quoteMint,
		payer,
		tokenBaseProgram,
		tokenQuoteProgram,
		solana.PublicKey{},
		eventAuthority,
		dbc.ProgramID,
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
	payerAndBuyer *solana.Wallet,
	baseMint *solana.Wallet,
	name string,
	symbol string,
	uri string,
	// buyer *solana.Wallet,
	amountIn *big.Int,
	slippageBps uint64, // 250 = 2.5%
) (string, error) {

	payer := payerAndBuyer
	buyer := payerAndBuyer

	rentExemptFee, err := solanago.GetRentExempt(ctx, m.rpcClient)
	if err != nil {
		return "", err
	}

	lamportsSOL, err := solanago.SOLBalance(ctx, m.rpcClient, buyer.PublicKey())
	if err != nil {
		return "", err
	}

	if lamportsSOL < rentExemptFee+transferFee {
		return "", fmt.Errorf("buyer sol must be greater than %v", (rentExemptFee+transferFee)/1e9)
	}

	if amountIn.Cmp(new(big.Int).SetUint64(lamportsSOL+1)) < 0 {
		return "", fmt.Errorf("amountIn must be greater than %v SOL", (rentExemptFee+transferFee+1)/1e9)
	}

	configState, err := m.GetConfig(ctx, m.config.PublicKey())
	if err != nil {
		return "", err
	}

	instructions, err := CreatePoolWithFirstBuInstruction(
		ctx,
		m.rpcClient,
		payer.PublicKey(),
		m.poolCreator.PublicKey(),
		m.config.PublicKey(),
		configState,
		baseMint.PublicKey(),
		solana.WrappedSol,
		dbc.GetTokenProgram(configState.TokenType),
		solana.TokenProgramID,
		m.poolCreator.PublicKey(),
		name,
		symbol,
		uri,
		buyer.PublicKey(),
		new(big.Int).Sub(amountIn, new(big.Int).SetUint64(rentExemptFee+transferFee)),
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
			case key.Equals(baseMint.PublicKey()):
				return &baseMint.PrivateKey
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
	return GetPoolsByConfig(ctx, m.rpcClient, m.config.PublicKey())
}

func GetPoolsByConfig(
	ctx context.Context,
	rpcClient *rpc.Client,
	config solana.PublicKey,
) ([]*dbc.VirtualPool, error) {

	opt := solanago.GenProgramAccountFilter(dbc.AccountKeyPoolConfig, &solanago.Filter{
		Owner:  config,
		Offset: solanago.ComputeStructOffset(new(dbc.VirtualPool), "Config"),
	})

	// opt := solanago.GenProgramAccountFilter(dbc.AccountKeyPoolConfig, &solanago.Filter{Owner: config, Offset: 72})

	outs, err := rpcClient.GetProgramAccountsWithOpts(ctx, dbc.ProgramID, opt)
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

func (m *DBC) GetPoolsByCreator(
	ctx context.Context,
) ([]*dbc.VirtualPool, error) {
	return GetPoolsByCreator(ctx, m.rpcClient, m.poolCreator.PublicKey())
}

func GetPoolsByCreator(
	ctx context.Context,
	rpcClient *rpc.Client,
	poolCreator solana.PublicKey,
) ([]*dbc.VirtualPool, error) {
	opt := solanago.GenProgramAccountFilter(dbc.AccountKeyVirtualPool, &solanago.Filter{
		Owner:  poolCreator,
		Offset: solanago.ComputeStructOffset(new(dbc.VirtualPool), "Creator"),
	})

	// opt := solanago.GenProgramAccountFilter(dbc.AccountKeyVirtualPool, &solanago.Filter{Owner: poolCreator, Offset: 104})
	outs, err := rpcClient.GetProgramAccountsWithOpts(ctx, dbc.ProgramID, opt)
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

func (m *DBC) GetPoolByBaseMint(
	ctx context.Context,
	baseMint solana.PublicKey,
) (*Pool, error) {
	return GetPoolByBaseMint(ctx, m.rpcClient, baseMint)
}

func GetPoolByBaseMint(
	ctx context.Context,
	rpcClient *rpc.Client,
	baseMint solana.PublicKey,
) (*Pool, error) {
	opt := solanago.GenProgramAccountFilter(dbc.AccountKeyVirtualPool, &solanago.Filter{
		Owner:  baseMint,
		Offset: solanago.ComputeStructOffset(new(dbc.VirtualPool), "BaseMint"),
	})
	// opt := solanago.GenProgramAccountFilter(dbc.AccountKeyVirtualPool, &solanago.Filter{Owner: baseMint, Offset: 136})

	outs, err := rpcClient.GetProgramAccountsWithOpts(ctx, dbc.ProgramID, opt)
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

	pool, ok := obj.(*dbc.VirtualPool)
	if !ok {
		return nil, fmt.Errorf("obj.(*dbc.PoolConfig) fail")
	}

	return &Pool{pool, out.Pubkey}, nil
}
