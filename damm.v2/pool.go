package dammV2

import (
	"context"
	"fmt"
	"math/big"
	"strconv"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	sendandconfirmtransaction "github.com/gagliardetto/solana-go/rpc/sendAndConfirmTransaction"
	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"
	solanago "github.com/krazyTry/meteora-go/solana"
	"github.com/krazyTry/meteora-go/u128"
)

func cpAmmInitializeCustomizablePool(m *DammV2,
	// Params:
	param *cp_amm.InitializeCustomizablePoolParameters,

	// Accounts:
	creator solana.PublicKey,
	positionNft solana.PublicKey,
	position solana.PublicKey,
	positionNftAccount solana.PublicKey,
	payer solana.PublicKey,
	cpammPool solana.PublicKey,
	tokenBaseMint solana.PublicKey,
	tokenQuoteMint solana.PublicKey,
	tokenBaseVault solana.PublicKey,
	tokenQuoteVault solana.PublicKey,
	payerBaseToken solana.PublicKey,
	payerQuoteToken solana.PublicKey,
	tokenBaseProgram solana.PublicKey,
	tokenQuoteProgram solana.PublicKey,
	tokenBadgeAccounts []*solana.AccountMeta,
) (solana.Instruction, error) {

	poolAuthority := m.poolAuthority
	eventAuthority := m.eventAuthority
	program := cp_amm.ProgramID
	systemProgram := solana.SystemProgramID

	return cp_amm.NewInitializeCustomizablePoolInstruction(
		// Params:
		param,
		// Accounts:
		creator,
		positionNft,
		positionNftAccount,
		payer,
		poolAuthority,
		cpammPool,
		position,
		tokenBaseMint,
		tokenQuoteMint,
		tokenBaseVault,
		tokenQuoteVault,
		payerBaseToken,
		payerQuoteToken,
		tokenBaseProgram,
		tokenQuoteProgram,
		solana.Token2022ProgramID,
		systemProgram,
		eventAuthority,
		program,
		tokenBadgeAccounts,
	)
}

func (m *DammV2) CreateCustomizablePoolInstruction(ctx context.Context,
	payer *solana.Wallet,
	initialPrice int, // 1 base token = 1 quote token
	baseMint solana.PublicKey,
	quoteMint solana.PublicKey,
	baseAmount *big.Int,
	quoteAmount *big.Int,
	hasAlphaVault bool,
	activationType cp_amm.ActivationType,
	collectFeeMode cp_amm.CollectFeeMode,
	activationPoint *uint64,
	useDynamicFee bool,
	maxBaseFeeBps int64,
	minBaseFeeBps int64,
	feeSchedulerMode cp_amm.FeeSchedulerMode,
	numberOfPeriod int,
	totalDuration int64,
	isLockLiquidity bool,
) ([]solana.Instruction, error) {
	tokens, err := solanago.GetMultipleToken(ctx, m.rpcClient, baseMint, quoteMint)
	if err != nil {
		return nil, err
	}
	if tokens[0] == nil || tokens[1] == nil {
		return nil, fmt.Errorf("baseMint or quoteMint error")
	}

	tokenBaseProgram := tokens[0].Owner

	tokenQuoteProgram := tokens[1].Owner

	tokenBaseDecimals := tokens[0].Decimals
	tokenQuoteDecimals := tokens[1].Decimals

	initialPoolTokenBaseAmount := cp_amm.GetInitialPoolTokenAmount(baseAmount, tokenBaseDecimals)
	initialPoolTokenQuoteAmount := cp_amm.GetInitialPoolTokenAmount(quoteAmount, tokenQuoteDecimals)

	initSqrtPrice, err := cp_amm.GetSqrtPriceFromPrice(strconv.Itoa(initialPrice), tokenBaseDecimals, tokenQuoteDecimals)
	if err != nil {
		return nil, err
	}

	liquidityDelta := cp_amm.GetLiquidityDelta(
		initialPoolTokenBaseAmount,
		initialPoolTokenQuoteAmount,
		cp_amm.MAX_SQRT_PRICE,
		cp_amm.MIN_SQRT_PRICE,
		initSqrtPrice,
	)

	baseFeeParam, err := cp_amm.GetBaseFeeParams(maxBaseFeeBps, minBaseFeeBps, feeSchedulerMode, numberOfPeriod, totalDuration)
	if err != nil {
		return nil, err
	}

	dynamicFeeParam, err := cp_amm.GetDynamicFeeParams(minBaseFeeBps, 0)
	if err != nil {
		return nil, err
	}

	poolFees := cp_amm.PoolFeeParameters{
		BaseFee:    *baseFeeParam,
		DynamicFee: dynamicFeeParam,
		Padding:    [3]uint8{},
	}

	positionNft := solana.NewWallet()

	position, err := cp_amm.DerivePositionAddress(positionNft.PublicKey())
	if err != nil {
		return nil, err
	}

	positionNftAccount, err := cp_amm.DerivePositionNftAccount(positionNft.PublicKey())
	if err != nil {
		return nil, err
	}

	cpammPool, err := m.deriveCpAmmPoolPDA(quoteMint, baseMint)
	if err != nil {
		return nil, err
	}

	baseVault, err := cp_amm.DeriveTokenVaultAddress(baseMint, cpammPool)
	if err != nil {
		return nil, err
	}
	quoteVault, err := cp_amm.DeriveTokenVaultAddress(quoteMint, cpammPool)
	if err != nil {
		return nil, err
	}

	var instructions []solana.Instruction

	baseTokenAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, m.poolCreator.PublicKey(), baseMint, payer.PublicKey(), &instructions)
	if err != nil {
		return nil, err
	}

	quoteTokenAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, m.poolCreator.PublicKey(), quoteMint, payer.PublicKey(), &instructions)
	if err != nil {
		return nil, err
	}

	if baseMint.Equals(solana.WrappedSol) {
		if initialPoolTokenBaseAmount.Cmp(big.NewInt(0)) <= 0 {
			return nil, fmt.Errorf("amountIn must be greater than 0")
		}

		wrapSOLIx := system.NewTransferInstruction(
			initialPoolTokenBaseAmount.Uint64(),
			m.poolCreator.PublicKey(),
			baseTokenAccount,
		).Build()

		// sync the WSOL account to update its balance
		syncNativeIx := token.NewSyncNativeInstruction(
			baseTokenAccount,
		).Build()

		instructions = append(instructions, wrapSOLIx, syncNativeIx)
	}

	if quoteMint.Equals(solana.WrappedSol) {
		if initialPoolTokenQuoteAmount.Cmp(big.NewInt(0)) <= 0 {
			return nil, fmt.Errorf("amountIn must be greater than 0")
		}

		wrapSOLIx := system.NewTransferInstruction(
			initialPoolTokenQuoteAmount.Uint64(),
			m.poolCreator.PublicKey(),
			quoteTokenAccount,
		).Build()

		// sync the WSOL account to update its balance
		syncNativeIx := token.NewSyncNativeInstruction(
			quoteTokenAccount,
		).Build()

		instructions = append(instructions, wrapSOLIx, syncNativeIx)
	}

	var tokenBadgeAccounts []*solana.AccountMeta

	baseTokenBadge, err := cp_amm.DeriveTokenBadgeAddress(baseMint)
	if err != nil {
		return nil, err
	}

	quoteTokenBadge, err := cp_amm.DeriveTokenBadgeAddress(quoteMint)
	if err != nil {
		return nil, err
	}

	tokenBadgeAccounts = append(tokenBadgeAccounts, solana.NewAccountMeta(baseTokenBadge, false, false))
	tokenBadgeAccounts = append(tokenBadgeAccounts, solana.NewAccountMeta(quoteTokenBadge, false, false))

	createIx, err := cpAmmInitializeCustomizablePool(m,
		&cp_amm.InitializeCustomizablePoolParameters{
			PoolFees:        poolFees,
			SqrtMinPrice:    u128.GenUint128FromString(cp_amm.MIN_SQRT_PRICE.String()),
			SqrtMaxPrice:    u128.GenUint128FromString(cp_amm.MAX_SQRT_PRICE.String()),
			HasAlphaVault:   hasAlphaVault,
			Liquidity:       u128.GenUint128FromString(liquidityDelta.String()),
			SqrtPrice:       u128.GenUint128FromString(initSqrtPrice.String()),
			ActivationType:  activationType,
			CollectFeeMode:  collectFeeMode,
			ActivationPoint: activationPoint,
		},
		m.poolCreator.PublicKey(),
		positionNft.PublicKey(),
		position,
		positionNftAccount,
		payer.PublicKey(),
		cpammPool,

		baseMint,
		quoteMint,
		baseVault,
		quoteVault,
		baseTokenAccount,
		quoteTokenAccount,
		tokenBaseProgram,
		tokenQuoteProgram,
		tokenBadgeAccounts,
	)
	if err != nil {
		return nil, err
	}

	instructions = append(instructions, createIx)

	if baseMint.Equals(solana.WrappedSol) {
		unwrapIx := token.NewCloseAccountInstruction(
			baseTokenAccount,
			payer.PublicKey(),
			payer.PublicKey(),
			[]solana.PublicKey{},
		).Build()
		instructions = append(instructions, unwrapIx)
	}

	if quoteMint.Equals(solana.WrappedSol) {
		unwrapIx := token.NewCloseAccountInstruction(
			quoteTokenAccount,
			payer.PublicKey(),
			payer.PublicKey(),
			[]solana.PublicKey{},
		).Build()
		instructions = append(instructions, unwrapIx)
	}

	if isLockLiquidity {
		lockIx, err := cpAmmPermanentLockPosition(m, liquidityDelta, cpammPool, position, positionNftAccount, m.poolCreator.PublicKey())
		if err != nil {
			return nil, err
		}
		instructions = append(instructions, lockIx)
	}
	return instructions, nil
}

func (m *DammV2) CreateCustomizablePool(ctx context.Context,
	payer *solana.Wallet,
	initialPrice int, // 1 base token = 1 quote token
	baseMint solana.PublicKey,
	quoteMint solana.PublicKey,
	baseAmount *big.Int,
	quoteAmount *big.Int,
	hasAlphaVault bool,
	activationType cp_amm.ActivationType,
	collectFeeMode cp_amm.CollectFeeMode,
	activationPoint *uint64,
	useDynamicFee bool,
	maxBaseFeeBps int64,
	minBaseFeeBps int64,
	feeSchedulerMode cp_amm.FeeSchedulerMode,
	numberOfPeriod int,
	totalDuration int64,
	isLockLiquidity bool,
) (string, error) {
	instructions, err := m.CreateCustomizablePoolInstruction(ctx,
		payer,
		initialPrice,
		baseMint,
		quoteMint,
		baseAmount,
		quoteAmount,
		hasAlphaVault,
		activationType,
		collectFeeMode,
		activationPoint,
		useDynamicFee,
		maxBaseFeeBps,
		minBaseFeeBps,
		feeSchedulerMode,
		numberOfPeriod,
		totalDuration,
		isLockLiquidity,
	)
	if err != nil {
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

func cpAmmInitializePool(m *DammV2,
	// Params:
	param *cp_amm.InitializePoolParameters,

	// Accounts:
	creator solana.PublicKey,
	positionNft solana.PublicKey,
	positionNftAccount solana.PublicKey,
	payer solana.PublicKey,
	config solana.PublicKey,
	cpammPool solana.PublicKey,
	position solana.PublicKey,
	tokenBaseMint solana.PublicKey,
	tokenQuoteMint solana.PublicKey,
	tokenBaseVault solana.PublicKey,
	tokenQuoteVault solana.PublicKey,
	tokenBaseAccount solana.PublicKey,
	tokenQuoteAccount solana.PublicKey,
	tokenBaseProgram solana.PublicKey,
	tokenQuoteProgram solana.PublicKey,
	tokenBadgeAccounts []*solana.AccountMeta,
) (solana.Instruction, error) {
	poolAuthority := m.poolAuthority
	eventAuthority := m.eventAuthority
	program := cp_amm.ProgramID
	systemProgram := solana.SystemProgramID
	return cp_amm.NewInitializePoolInstruction(
		// Params:
		param,

		// Accounts:
		creator,
		positionNft,
		positionNftAccount,
		payer,
		config,
		poolAuthority,
		cpammPool,
		position,
		tokenBaseMint,
		tokenQuoteMint,
		tokenBaseVault,
		tokenQuoteVault,
		tokenBaseAccount,
		tokenQuoteAccount,
		tokenBaseProgram,
		tokenQuoteProgram,
		solana.Token2022ProgramID,
		systemProgram,
		eventAuthority,
		program,
		tokenBadgeAccounts,
	)
}

func (m *DammV2) CreatePoolInstruction(ctx context.Context,
	payer *solana.Wallet,
	initialPrice int,
	baseMint solana.PublicKey,
	quoteMint solana.PublicKey,
	baseAmount *big.Int,
	quoteAmount *big.Int,
	activationPoint *uint64,
	isLockLiquidity bool,
) ([]solana.Instruction, error) {
	configurator := solana.PublicKey{}
	switch {
	case m.config != nil:
		configurator = m.config.PublicKey()
	case !m.cpAmmConfig.Equals(solana.PublicKey{}):
		configurator = m.cpAmmConfig
	}

	if configurator.Equals(solana.PublicKey{}) {
		return nil, fmt.Errorf("config or cpAmmConfig not nil")
	}

	config, err := m.GetConfig(ctx, configurator)
	if err != nil {
		return nil, err
	}

	tokens, err := solanago.GetMultipleToken(ctx, m.rpcClient, baseMint, quoteMint)
	if err != nil {
		return nil, err
	}
	if tokens[0] == nil || tokens[1] == nil {
		return nil, fmt.Errorf("baseMint or quoteMint error")
	}

	tokenBaseProgram := tokens[0].Owner

	tokenQuoteProgram := tokens[1].Owner

	tokenBaseDecimals := tokens[0].Decimals
	tokenQuoteDecimals := tokens[1].Decimals

	initialPoolTokenBaseAmount := cp_amm.GetInitialPoolTokenAmount(baseAmount, tokenBaseDecimals)
	initialPoolTokenQuoteAmount := cp_amm.GetInitialPoolTokenAmount(quoteAmount, tokenQuoteDecimals)

	initSqrtPrice, err := cp_amm.GetSqrtPriceFromPrice(strconv.Itoa(initialPrice), tokenBaseDecimals, tokenQuoteDecimals)
	if err != nil {
		return nil, err
	}

	liquidityDelta := cp_amm.GetLiquidityDelta(
		initialPoolTokenBaseAmount,
		initialPoolTokenQuoteAmount,
		config.SqrtMaxPrice.BigInt(),
		config.SqrtMinPrice.BigInt(),
		initSqrtPrice,
	)

	positionNft := solana.NewWallet()

	position, err := cp_amm.DerivePositionAddress(positionNft.PublicKey())
	if err != nil {
		return nil, err
	}

	positionNftAccount, err := cp_amm.DerivePositionNftAccount(positionNft.PublicKey())
	if err != nil {
		return nil, err
	}

	cpammPool, err := m.deriveCpAmmPoolPDA(quoteMint, baseMint)
	if err != nil {
		return nil, err
	}

	baseVault, err := cp_amm.DeriveTokenVaultAddress(baseMint, cpammPool)
	if err != nil {
		return nil, err
	}
	quoteVault, err := cp_amm.DeriveTokenVaultAddress(quoteMint, cpammPool)
	if err != nil {
		return nil, err
	}

	var instructions []solana.Instruction

	baseTokenAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, m.poolCreator.PublicKey(), baseMint, payer.PublicKey(), &instructions)
	if err != nil {
		return nil, err
	}

	quoteTokenAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, m.poolCreator.PublicKey(), quoteMint, payer.PublicKey(), &instructions)
	if err != nil {
		return nil, err
	}

	if baseMint.Equals(solana.WrappedSol) {
		if initialPoolTokenBaseAmount.Cmp(big.NewInt(0)) <= 0 {
			return nil, fmt.Errorf("amountIn must be greater than 0")
		}

		wrapSOLIx := system.NewTransferInstruction(
			initialPoolTokenBaseAmount.Uint64(),
			m.poolCreator.PublicKey(),
			baseTokenAccount,
		).Build()

		// sync the WSOL account to update its balance
		syncNativeIx := token.NewSyncNativeInstruction(
			baseTokenAccount,
		).Build()

		instructions = append(instructions, wrapSOLIx, syncNativeIx)
	}

	if quoteMint.Equals(solana.WrappedSol) {
		if initialPoolTokenQuoteAmount.Cmp(big.NewInt(0)) <= 0 {
			return nil, fmt.Errorf("amountIn must be greater than 0")
		}

		wrapSOLIx := system.NewTransferInstruction(
			initialPoolTokenQuoteAmount.Uint64(),
			m.poolCreator.PublicKey(),
			quoteTokenAccount,
		).Build()

		// sync the WSOL account to update its balance
		syncNativeIx := token.NewSyncNativeInstruction(
			quoteTokenAccount,
		).Build()

		instructions = append(instructions, wrapSOLIx, syncNativeIx)
	}

	var tokenBadgeAccounts []*solana.AccountMeta
	baseTokenBadge, err := cp_amm.DeriveTokenBadgeAddress(baseMint)
	if err != nil {
		return nil, err
	}
	quoteTokenBadge, err := cp_amm.DeriveTokenBadgeAddress(quoteMint)
	if err != nil {
		return nil, err
	}
	tokenBadgeAccounts = append(tokenBadgeAccounts, solana.NewAccountMeta(baseTokenBadge, false, false))
	tokenBadgeAccounts = append(tokenBadgeAccounts, solana.NewAccountMeta(quoteTokenBadge, false, false))

	createIx, err := cpAmmInitializePool(m,
		&cp_amm.InitializePoolParameters{},
		m.poolCreator.PublicKey(),
		positionNft.PublicKey(),
		positionNftAccount,
		payer.PublicKey(),
		configurator,
		cpammPool,
		position,
		baseMint,
		quoteMint,
		baseVault,
		quoteVault,
		baseTokenAccount,
		quoteTokenAccount,
		tokenBaseProgram,
		tokenQuoteProgram,
		tokenBadgeAccounts,
	)
	if err != nil {
		return nil, err
	}
	instructions = append(instructions, createIx)

	if baseMint.Equals(solana.WrappedSol) {
		unwrapIx := token.NewCloseAccountInstruction(
			baseTokenAccount,
			payer.PublicKey(),
			payer.PublicKey(),
			[]solana.PublicKey{},
		).Build()
		instructions = append(instructions, unwrapIx)
	}

	if quoteMint.Equals(solana.WrappedSol) {
		unwrapIx := token.NewCloseAccountInstruction(
			quoteTokenAccount,
			payer.PublicKey(),
			payer.PublicKey(),
			[]solana.PublicKey{},
		).Build()
		instructions = append(instructions, unwrapIx)
	}

	if isLockLiquidity {
		lockIx, err := cpAmmPermanentLockPosition(m, liquidityDelta, cpammPool, position, positionNftAccount, m.poolCreator.PublicKey())
		if err != nil {
			return nil, err
		}
		instructions = append(instructions, lockIx)
	}
	return instructions, nil
}

func (m *DammV2) CreatePool(ctx context.Context,
	payer *solana.Wallet,
	initialPrice int,
	baseMint solana.PublicKey,
	quoteMint solana.PublicKey,
	baseAmount *big.Int,
	quoteAmount *big.Int,
	activationPoint *uint64,
	isLockLiquidity bool,
) (string, error) {
	instructions, err := m.CreatePoolInstruction(ctx,
		payer,
		initialPrice,
		baseMint,
		quoteMint,
		baseAmount,
		quoteAmount,
		activationPoint,
		isLockLiquidity,
	)
	if err != nil {
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

func cpAmmInitializePoolWithDynamicConfig(m *DammV2,
	// Params:
	param *cp_amm.InitializeCustomizablePoolParameters,

	// Accounts:
	creator solana.PublicKey,
	positionNft solana.PublicKey,
	positionNftAccount solana.PublicKey,
	payer solana.PublicKey,
	poolCreatorAuthority solana.PublicKey,
	config solana.PublicKey,
	cpammPool solana.PublicKey,
	position solana.PublicKey,
	tokenBaseMint solana.PublicKey,
	tokenQuoteMint solana.PublicKey,
	tokenBaseVault solana.PublicKey,
	tokenQuoteVault solana.PublicKey,
	payerBaseToken solana.PublicKey,
	payerQuoteToken solana.PublicKey,
	tokenBaseProgram solana.PublicKey,
	tokenQuoteProgram solana.PublicKey,
	tokenBadgeAccounts []*solana.AccountMeta,
) (solana.Instruction, error) {
	poolAuthority := m.poolAuthority
	eventAuthority := m.eventAuthority
	program := cp_amm.ProgramID
	systemProgram := solana.SystemProgramID

	return cp_amm.NewInitializePoolWithDynamicConfigInstruction(
		// Params:
		param,

		// Accounts:
		creator,
		positionNft,
		positionNftAccount,
		payer,
		poolCreatorAuthority,
		config,
		poolAuthority,
		cpammPool,
		position,
		tokenBaseMint,
		tokenQuoteMint,
		tokenBaseVault,
		tokenQuoteVault,
		payerBaseToken,
		payerQuoteToken,
		tokenBaseProgram,
		tokenQuoteProgram,
		solana.Token2022ProgramID,
		systemProgram,
		eventAuthority,
		program,
		tokenBadgeAccounts,
	)
}

func (m *DammV2) CreateCustomizablePoolWithDynamicConfigInstruction(ctx context.Context,
	payer *solana.Wallet,
	poolCreatorAuthority *solana.Wallet,
	initialPrice int, // 1 base token = 1 quote token
	baseMint solana.PublicKey,
	quoteMint solana.PublicKey,
	baseAmount *big.Int,
	quoteAmount *big.Int,
	hasAlphaVault bool,
	activationType cp_amm.ActivationType,
	collectFeeMode cp_amm.CollectFeeMode,
	activationPoint *uint64,
	useDynamicFee bool,
	maxBaseFeeBps int64,
	minBaseFeeBps int64,
	feeSchedulerMode cp_amm.FeeSchedulerMode,
	numberOfPeriod int,
	totalDuration int64,
	isLockLiquidity bool,
) ([]solana.Instruction, error) {
	configurator := solana.PublicKey{}
	switch {
	case m.config != nil:
		configurator = m.config.PublicKey()
	case !m.cpAmmConfig.Equals(solana.PublicKey{}):
		configurator = m.cpAmmConfig
	}

	if configurator.Equals(solana.PublicKey{}) {
		return nil, fmt.Errorf("config or cpAmmConfig not nil")
	}

	config, err := m.GetConfig(ctx, configurator)
	if err != nil {
		return nil, err
	}

	tokens, err := solanago.GetMultipleToken(ctx, m.rpcClient, baseMint, quoteMint)
	if err != nil {
		return nil, err
	}
	if tokens[0] == nil || tokens[1] == nil {
		return nil, fmt.Errorf("baseMint or quoteMint error")
	}

	tokenBaseProgram := tokens[0].Owner

	tokenQuoteProgram := tokens[1].Owner

	tokenBaseDecimals := tokens[0].Decimals
	tokenQuoteDecimals := tokens[1].Decimals

	initialPoolTokenBaseAmount := cp_amm.GetInitialPoolTokenAmount(baseAmount, tokenBaseDecimals)
	initialPoolTokenQuoteAmount := cp_amm.GetInitialPoolTokenAmount(quoteAmount, tokenQuoteDecimals)

	initSqrtPrice, err := cp_amm.GetSqrtPriceFromPrice(strconv.Itoa(initialPrice), tokenBaseDecimals, tokenQuoteDecimals)
	if err != nil {
		return nil, err
	}

	liquidityDelta := cp_amm.GetLiquidityDelta(
		initialPoolTokenBaseAmount,
		initialPoolTokenQuoteAmount,
		config.SqrtMaxPrice.BigInt(),
		config.SqrtMinPrice.BigInt(),
		initSqrtPrice,
	)

	baseFeeParam, err := cp_amm.GetBaseFeeParams(maxBaseFeeBps, minBaseFeeBps, feeSchedulerMode, numberOfPeriod, totalDuration)
	if err != nil {
		return nil, err
	}

	poolFees := cp_amm.PoolFeeParameters{
		BaseFee: *baseFeeParam,
		Padding: [3]uint8{},
	}

	positionNft := solana.NewWallet()

	position, err := cp_amm.DerivePositionAddress(positionNft.PublicKey())
	if err != nil {
		return nil, err
	}

	positionNftAccount, err := cp_amm.DerivePositionNftAccount(positionNft.PublicKey())
	if err != nil {
		return nil, err
	}

	cpammPool, err := m.deriveCpAmmPoolPDA(quoteMint, baseMint)
	if err != nil {
		return nil, err
	}

	baseVault, err := cp_amm.DeriveTokenVaultAddress(baseMint, cpammPool)
	if err != nil {
		return nil, err
	}
	quoteVault, err := cp_amm.DeriveTokenVaultAddress(quoteMint, cpammPool)
	if err != nil {
		return nil, err
	}

	var instructions []solana.Instruction

	baseTokenAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, m.poolCreator.PublicKey(), baseMint, payer.PublicKey(), &instructions)
	if err != nil {
		return nil, err
	}

	quoteTokenAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, m.poolCreator.PublicKey(), quoteMint, payer.PublicKey(), &instructions)
	if err != nil {
		return nil, err
	}

	if baseMint.Equals(solana.WrappedSol) {
		if initialPoolTokenBaseAmount.Cmp(big.NewInt(0)) <= 0 {
			return nil, fmt.Errorf("amountIn must be greater than 0")
		}

		wrapSOLIx := system.NewTransferInstruction(
			initialPoolTokenBaseAmount.Uint64(),
			m.poolCreator.PublicKey(),
			baseTokenAccount,
		).Build()

		// sync the WSOL account to update its balance
		syncNativeIx := token.NewSyncNativeInstruction(
			baseTokenAccount,
		).Build()

		instructions = append(instructions, wrapSOLIx, syncNativeIx)
	}

	if quoteMint.Equals(solana.WrappedSol) {
		if initialPoolTokenQuoteAmount.Cmp(big.NewInt(0)) <= 0 {
			return nil, fmt.Errorf("amountIn must be greater than 0")
		}

		wrapSOLIx := system.NewTransferInstruction(
			initialPoolTokenQuoteAmount.Uint64(),
			m.poolCreator.PublicKey(),
			quoteTokenAccount,
		).Build()

		// sync the WSOL account to update its balance
		syncNativeIx := token.NewSyncNativeInstruction(
			quoteTokenAccount,
		).Build()

		instructions = append(instructions, wrapSOLIx, syncNativeIx)
	}

	var tokenBadgeAccounts []*solana.AccountMeta
	baseTokenBadge, err := cp_amm.DeriveTokenBadgeAddress(baseMint)
	if err != nil {
		return nil, err
	}
	quoteTokenBadge, err := cp_amm.DeriveTokenBadgeAddress(quoteMint)
	if err != nil {
		return nil, err
	}
	tokenBadgeAccounts = append(tokenBadgeAccounts, solana.NewAccountMeta(baseTokenBadge, false, false))
	tokenBadgeAccounts = append(tokenBadgeAccounts, solana.NewAccountMeta(quoteTokenBadge, false, false))

	createIx, err := cpAmmInitializePoolWithDynamicConfig(m,
		&cp_amm.InitializeCustomizablePoolParameters{
			PoolFees:        poolFees,
			SqrtMinPrice:    u128.GenUint128FromString(cp_amm.MIN_SQRT_PRICE.String()),
			SqrtMaxPrice:    u128.GenUint128FromString(cp_amm.MAX_SQRT_PRICE.String()),
			HasAlphaVault:   hasAlphaVault,
			Liquidity:       u128.GenUint128FromString(liquidityDelta.String()),
			SqrtPrice:       u128.GenUint128FromString(initSqrtPrice.String()),
			ActivationType:  activationType,
			CollectFeeMode:  collectFeeMode,
			ActivationPoint: activationPoint,
		},
		m.poolCreator.PublicKey(),
		positionNft.PublicKey(),
		positionNftAccount,
		payer.PublicKey(),
		poolCreatorAuthority.PublicKey(),
		configurator,
		cpammPool,
		position,
		baseMint,
		quoteMint,
		baseVault,
		quoteVault,
		baseTokenAccount,
		quoteTokenAccount,
		tokenBaseProgram,
		tokenQuoteProgram,
		tokenBadgeAccounts,
	)
	if err != nil {
		return nil, err
	}
	instructions = append(instructions, createIx)

	if baseMint.Equals(solana.WrappedSol) {
		unwrapIx := token.NewCloseAccountInstruction(
			baseTokenAccount,
			payer.PublicKey(),
			payer.PublicKey(),
			[]solana.PublicKey{},
		).Build()
		instructions = append(instructions, unwrapIx)
	}

	if quoteMint.Equals(solana.WrappedSol) {
		unwrapIx := token.NewCloseAccountInstruction(
			quoteTokenAccount,
			payer.PublicKey(),
			payer.PublicKey(),
			[]solana.PublicKey{},
		).Build()
		instructions = append(instructions, unwrapIx)
	}

	if isLockLiquidity {
		lockIx, err := cpAmmPermanentLockPosition(m, liquidityDelta, cpammPool, position, positionNftAccount, m.poolCreator.PublicKey())
		if err != nil {
			return nil, err
		}
		instructions = append(instructions, lockIx)
	}
	return instructions, nil
}

func (m *DammV2) CreateCustomizablePoolWithDynamicConfig(ctx context.Context,
	payer *solana.Wallet,
	poolCreatorAuthority *solana.Wallet,
	initialPrice int, // 1 base token = 1 quote token
	baseMint solana.PublicKey,
	quoteMint solana.PublicKey,
	baseAmount *big.Int,
	quoteAmount *big.Int,
	hasAlphaVault bool,
	activationType cp_amm.ActivationType,
	collectFeeMode cp_amm.CollectFeeMode,
	activationPoint *uint64,
	useDynamicFee bool,
	maxBaseFeeBps int64,
	minBaseFeeBps int64,
	feeSchedulerMode cp_amm.FeeSchedulerMode,
	numberOfPeriod int,
	totalDuration int64,
	isLockLiquidity bool,
) (string, error) {

	instructions, err := m.CreateCustomizablePoolWithDynamicConfigInstruction(ctx,
		payer,
		poolCreatorAuthority,
		initialPrice,
		baseMint,
		quoteMint,
		baseAmount,
		quoteAmount,
		hasAlphaVault,
		activationType,
		collectFeeMode,
		activationPoint,
		useDynamicFee,
		maxBaseFeeBps,
		minBaseFeeBps,
		feeSchedulerMode,
		numberOfPeriod,
		totalDuration,
		isLockLiquidity,
	)
	if err != nil {
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

func (m *DammV2) GetPools(ctx context.Context) (map[solana.PublicKey]*Pool, error) {
	opt := solanago.GenProgramAccountFilter(cp_amm.AccountKeyPool, solana.PublicKey{}, 0)

	outs, err := m.rpcClient.GetProgramAccountsWithOpts(ctx, cp_amm.ProgramID, opt)
	if err != nil {
		if err == rpc.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}

	data := make(map[solana.PublicKey]*Pool)
	for _, out := range outs {
		obj, err := cp_amm.ParseAnyAccount(out.Account.Data.GetBinary())
		if err != nil {
			return nil, err
		}
		pool, ok := obj.(*cp_amm.Pool)
		if !ok {
			return nil, fmt.Errorf("obj.(*cp_amm.Pool) fail")
		}
		data[pool.TokenAMint] = &Pool{pool, out.Pubkey}
	}

	return data, nil
}

func (m *DammV2) GetPoolByBaseMint(ctx context.Context, baseMint solana.PublicKey) (*Pool, error) {
	pools, err := m.GetPools(ctx)
	if err != nil {
		return nil, err
	}
	pool, ok := pools[baseMint]
	if !ok {
		return nil, nil
	}
	return pool, nil
}
