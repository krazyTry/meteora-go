package dammV2

import (
	"context"
	"fmt"
	"math/big"

	"github.com/gagliardetto/solana-go"
	computebudget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"
	solanago "github.com/krazyTry/meteora-go/solana"
	"github.com/krazyTry/meteora-go/u128"
	"github.com/shopspring/decimal"
)

// CreateCustomizablePoolInstruction generates the instruction required for CreateCustomizablePool
// The function includes the creation of the ATA account.
//
// Example:
//
// baseFeeParam, _ := cp_amm.GetBaseFeeParams(maxBaseFeeBps, minBaseFeeBps, feeSchedulerMode, numberOfPeriod, totalDuration)
//
// var dynamicFeeParam *cp_amm.DynamicFeeParameters
//
//	if useDynamicFee {
//		dynamicFeeParam, _ = cp_amm.GetDynamicFeeParams(minBaseFeeBps, cp_amm.MAX_PRICE_CHANGE_BPS_DEFAULT)
//	}
//
//	poolFees := cp_amm.PoolFeeParameters{
//		BaseFee:    *baseFeeParam,
//		DynamicFee: dynamicFeeParam,
//		Padding:    [3]uint8{},
//	}
//
// positionNft := solana.NewWallet()
//
// instructions, cpammPool, err := CreateCustomizablePoolInstruction(
//
//	ctx,
//	m.rpcClient,
//	payer.PublicKey(), // payer account
//	m.poolCreator.PublicKey(), // pool creator account
//	positionNft.PublicKey(),// position of the creator account
//	initialPrice, // 1 base token = 1 quote token
//	baseMint, // baseMintToken
//	quoteMint, // quoteMintToken
//	baseAmount, // baseMintAmount
//	quoteAmount,// quoteMintAmount
//	hasAlphaVault,
//	activationType, // 0.ActivationTypeSlot or 1.ActivationTypeTimestamp
//	collectFeeMode, // 0.CollectFeeModeBothToken 1.CollectFeeModeTokenA 2.CollectFeeModeTokenB
//	activationPoint,
//	cp_amm.MAX_SQRT_PRICE,
//	cp_amm.MIN_SQRT_PRICE,
//	poolFees,
//	isLockLiquidity,
//
// )
func CreateCustomizablePoolInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	payer solana.PublicKey,
	poolCreator solana.PublicKey,
	positionNft solana.PublicKey,
	initialPrice float64, // 1 base token = 1 quote token
	baseMint solana.PublicKey,
	quoteMint solana.PublicKey,
	baseAmount *big.Int,
	quoteAmount *big.Int,
	hasAlphaVault bool,
	activationType cp_amm.ActivationType,
	collectFeeMode cp_amm.CollectFeeMode,
	activationPoint *uint64,
	sqrtMaxPrice *big.Int,
	sqrtMinPrice *big.Int,
	poolFees cp_amm.PoolFeeParameters,
	isLockLiquidity bool,
) ([]solana.Instruction, solana.PublicKey, error) {
	tokens, err := solanago.GetMultipleToken(ctx, rpcClient, baseMint, quoteMint)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	if tokens[0] == nil || tokens[1] == nil {
		return nil, solana.PublicKey{}, fmt.Errorf("baseMint or quoteMint error")
	}

	tokenBaseProgram := tokens[0].Owner
	tokenQuoteProgram := tokens[1].Owner

	tokenBaseDecimals := tokens[0].Decimals
	tokenQuoteDecimals := tokens[1].Decimals

	initialPoolTokenBaseAmount := cp_amm.GetInitialPoolTokenAmount(decimal.NewFromBigInt(baseAmount, 0), tokenBaseDecimals)
	initialPoolTokenQuoteAmount := cp_amm.GetInitialPoolTokenAmount(decimal.NewFromBigInt(quoteAmount, 0), tokenQuoteDecimals)

	initSqrtPrice, err := cp_amm.GetSqrtPriceFromPrice(decimal.NewFromFloat(initialPrice), tokenBaseDecimals, tokenQuoteDecimals)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	liquidityDelta := cp_amm.GetLiquidityDelta(
		initialPoolTokenBaseAmount,
		initialPoolTokenQuoteAmount,
		decimal.NewFromBigInt(sqrtMaxPrice, 0), // cp_amm.MAX_SQRT_PRICE,
		decimal.NewFromBigInt(sqrtMinPrice, 0), // cp_amm.MIN_SQRT_PRICE,
		initSqrtPrice,
	)

	position, err := cp_amm.DerivePositionAddress(positionNft)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	positionNftAccount, err := cp_amm.DerivePositionNftAccount(positionNft)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	poolAddress, err := cp_amm.DeriveCustomizablePoolAddress(baseMint, quoteMint) //m.deriveCpAmmPoolPDA(quoteMint, baseMint)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	baseVault, err := cp_amm.DeriveTokenVaultAddress(baseMint, poolAddress)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}
	quoteVault, err := cp_amm.DeriveTokenVaultAddress(quoteMint, poolAddress)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	var instructions []solana.Instruction

	baseTokenAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, payer, baseMint, payer, &instructions)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	quoteTokenAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, payer, quoteMint, payer, &instructions)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	if baseMint.Equals(solana.WrappedSol) {
		if initialPoolTokenBaseAmount.Cmp(decimal.Zero) <= 0 {
			return nil, solana.PublicKey{}, fmt.Errorf("amountIn must be greater than 0")
		}

		wrapSOLIx := system.NewTransferInstruction(
			initialPoolTokenBaseAmount.BigInt().Uint64(),
			payer,
			baseTokenAccount,
		).Build()

		// sync the WSOL account to update its balance
		syncNativeIx := token.NewSyncNativeInstruction(
			baseTokenAccount,
		).Build()

		instructions = append(instructions, wrapSOLIx, syncNativeIx)
	}

	if quoteMint.Equals(solana.WrappedSol) {
		if initialPoolTokenQuoteAmount.Cmp(decimal.Zero) <= 0 {
			return nil, solana.PublicKey{}, fmt.Errorf("amountIn must be greater than 0")
		}

		wrapSOLIx := system.NewTransferInstruction(
			initialPoolTokenQuoteAmount.BigInt().Uint64(),
			payer,
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
		return nil, solana.PublicKey{}, err
	}

	quoteTokenBadge, err := cp_amm.DeriveTokenBadgeAddress(quoteMint)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	tokenBadgeAccounts = append(tokenBadgeAccounts, solana.NewAccountMeta(baseTokenBadge, false, false))
	tokenBadgeAccounts = append(tokenBadgeAccounts, solana.NewAccountMeta(quoteTokenBadge, false, false))

	createIx, err := cp_amm.NewInitializeCustomizablePoolInstruction(
		// Params:
		&cp_amm.InitializeCustomizablePoolParameters{
			PoolFees:        poolFees,
			SqrtMinPrice:    u128.GenUint128FromString(sqrtMinPrice.String()),
			SqrtMaxPrice:    u128.GenUint128FromString(sqrtMaxPrice.String()),
			HasAlphaVault:   hasAlphaVault,
			Liquidity:       u128.GenUint128FromString(liquidityDelta.String()),
			SqrtPrice:       u128.GenUint128FromString(initSqrtPrice.String()),
			ActivationType:  activationType,
			CollectFeeMode:  collectFeeMode,
			ActivationPoint: activationPoint,
		},
		// Accounts:
		poolCreator,
		positionNft,
		positionNftAccount,
		payer,
		poolAuthority,
		poolAddress,
		position,
		baseMint,
		quoteMint,
		baseVault,
		quoteVault,
		baseTokenAccount,
		quoteTokenAccount,
		tokenBaseProgram,
		tokenQuoteProgram,
		solana.Token2022ProgramID,
		solana.SystemProgramID,
		eventAuthority,
		cp_amm.ProgramID,
		tokenBadgeAccounts,
	)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	instructions = append(instructions, createIx)

	if baseMint.Equals(solana.WrappedSol) {
		unwrapIx := token.NewCloseAccountInstruction(
			baseTokenAccount,
			payer,
			payer,
			nil,
		).Build()
		instructions = append(instructions, unwrapIx)
	}

	if quoteMint.Equals(solana.WrappedSol) {
		unwrapIx := token.NewCloseAccountInstruction(
			quoteTokenAccount,
			payer,
			payer,
			nil,
		).Build()
		instructions = append(instructions, unwrapIx)
	}

	if isLockLiquidity {
		lockIx, err := cp_amm.NewPermanentLockPositionInstruction(
			// Params:
			u128.GenUint128FromString(liquidityDelta.String()),

			// Accounts:
			poolAddress,
			position,
			positionNftAccount,
			poolCreator,
			eventAuthority,
			cp_amm.ProgramID,
		)
		if err != nil {
			return nil, solana.PublicKey{}, err
		}
		instructions = append(instructions, lockIx)
	}
	return instructions, poolAddress, nil
}

// CreateCustomizablePool Creates a customizable pool with specific fee parameters, reward settings, and activation conditions.
// The function depends on CreateCustomizablePoolInstruction.
// The function is blocking; it will wait for on-chain confirmation before returning.
// This function is an example function. The sqrtMaxPrice and sqrtMinPrice are hardcoded. If you want to modify them, you need to implement it yourself.
//
// Example:
//
// baseAmount := big.NewInt(1_000_000)
//
// quoteAmount := big.NewInt(1) // SOL
//
// sig, cpammPool, positionNft, _ := meteoraDammV2.CreateCustomizablePool(
//
//	ctx1,
//	wsClient,
//	payer, // payer account
//	1, // 1 base token = 1 quote token
//	baseMint, // baseMintToken
//	solana.WrappedSol, // quoteMintToken
//	baseAmount, // baseMintAmount
//	quoteAmount, // quoteMintAmount
//	false,
//	cp_amm.ActivationTypeTimestamp, // 0.ActivationTypeSlot or 1.ActivationTypeTimestamp
//	cp_amm.CollectFeeModeBothToken, // 0.CollectFeeModeBothToken 1.CollectFeeModeTokenA 2.CollectFeeModeTokenB
//	nil,
//	true,
//	5000, // 50%
//	25,   // 0.25%
//	cp_amm.FeeSchedulerModeExponential,
//	60,   // 60 peridos
//	3600, // 60 * 60
//	true,
//
// )
func (m *DammV2) CreateCustomizablePool(
	ctx context.Context,
	wsClient *ws.Client,
	payer *solana.Wallet,
	initialPrice float64, // 1 base token = 1 quote token
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
) (string, solana.PublicKey, *solana.Wallet, error) {

	baseFeeParam, err := cp_amm.GetBaseFeeParams(maxBaseFeeBps, minBaseFeeBps, feeSchedulerMode, numberOfPeriod, totalDuration)
	if err != nil {
		return "", solana.PublicKey{}, nil, err
	}

	var dynamicFeeParam *cp_amm.DynamicFeeParameters
	if useDynamicFee {
		dynamicFeeParam, err = cp_amm.GetDynamicFeeParams(minBaseFeeBps, cp_amm.MAX_PRICE_CHANGE_BPS_DEFAULT)
		if err != nil {
			return "", solana.PublicKey{}, nil, err
		}
	}

	poolFees := cp_amm.PoolFeeParameters{
		BaseFee:    *baseFeeParam,
		DynamicFee: dynamicFeeParam,
		Padding:    [3]uint8{},
	}

	positionNft := solana.NewWallet()

	instructions, cpammPool, err := CreateCustomizablePoolInstruction(
		ctx,
		m.rpcClient,
		payer.PublicKey(),
		m.poolCreator.PublicKey(),
		positionNft.PublicKey(),
		initialPrice,
		baseMint,
		quoteMint,
		baseAmount,
		quoteAmount,
		hasAlphaVault,
		activationType,
		collectFeeMode,
		activationPoint,
		cp_amm.MAX_SQRT_PRICE,
		cp_amm.MIN_SQRT_PRICE,
		poolFees,
		isLockLiquidity,
	)
	if err != nil {
		return "", solana.PublicKey{}, nil, err
	}
	sig, err := solanago.SendTransaction(ctx,
		m.rpcClient,
		wsClient,
		instructions,
		m.poolCreator.PublicKey(),
		func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(payer.PublicKey()):
				return &payer.PrivateKey
			case key.Equals(m.poolCreator.PublicKey()):
				return &m.poolCreator.PrivateKey
			case key.Equals(positionNft.PublicKey()):
				return &positionNft.PrivateKey
			default:
				return nil
			}
		},
	)
	if err != nil {
		return "", solana.PublicKey{}, nil, err
	}
	return sig.String(), cpammPool, positionNft, nil
}

// CreatePoolInstruction generates the instruction required for CreatePool
// The function includes the creation of the ATA account.
//
// Example:
//
// config, _ := cp_amm.DeriveConfigAddress(configIndex)
//
// configState, _ := m.GetConfig(ctx, config)
//
// positionNft := solana.NewWallet()
//
// instructions, cpammPool, _ := CreatePoolInstruction(
//
//	ctx,
//	m.rpcClient,
//	payer.PublicKey(), // payer account
//	m.poolCreator.PublicKey(), // pool creator account
//	config, // damm v2 config address
//	configState, // damm v2 config state
//	positionNft.PublicKey(), // position of the pool creator account
//	initialPrice, // 1 base token = 1 quote token
//	baseMint, // baseMintToken
//	quoteMint, // quoteMintToken
//	baseAmount, // baseMintAmount
//	quoteAmount, // quoteMintAmount
//	activationPoint,
//	isLockLiquidity,
//
// )
func CreatePoolInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	payer solana.PublicKey,
	poolCreator solana.PublicKey,
	config solana.PublicKey,
	configState *cp_amm.Config,
	positionNft solana.PublicKey,
	initialPrice float64, // 1 base token = 1 quote token
	baseMint solana.PublicKey,
	quoteMint solana.PublicKey,
	baseAmount *big.Int,
	quoteAmount *big.Int,
	activationPoint *uint64,
	isLockLiquidity bool,
) ([]solana.Instruction, solana.PublicKey, error) {

	tokens, err := solanago.GetMultipleToken(ctx, rpcClient, baseMint, quoteMint)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}
	if tokens[0] == nil || tokens[1] == nil {
		return nil, solana.PublicKey{}, fmt.Errorf("baseMint or quoteMint error")
	}

	tokenBaseProgram := tokens[0].Owner

	tokenQuoteProgram := tokens[1].Owner

	tokenBaseDecimals := tokens[0].Decimals
	tokenQuoteDecimals := tokens[1].Decimals

	initialPoolTokenBaseAmount := cp_amm.GetInitialPoolTokenAmount(decimal.NewFromBigInt(baseAmount, 0), tokenBaseDecimals)
	initialPoolTokenQuoteAmount := cp_amm.GetInitialPoolTokenAmount(decimal.NewFromBigInt(quoteAmount, 0), tokenQuoteDecimals)

	initSqrtPrice, err := cp_amm.GetSqrtPriceFromPrice(decimal.NewFromFloat(initialPrice), tokenBaseDecimals, tokenQuoteDecimals)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	liquidityDelta := cp_amm.GetLiquidityDelta(
		initialPoolTokenBaseAmount,
		initialPoolTokenQuoteAmount,
		decimal.NewFromBigInt(configState.SqrtMaxPrice.BigInt(), 0),
		decimal.NewFromBigInt(configState.SqrtMinPrice.BigInt(), 0),
		initSqrtPrice,
	)

	position, err := cp_amm.DerivePositionAddress(positionNft)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	positionNftAccount, err := cp_amm.DerivePositionNftAccount(positionNft)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	poolAddress, err := cp_amm.DeriveCpAmmPoolPDA(config, quoteMint, baseMint)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	baseVault, err := cp_amm.DeriveTokenVaultAddress(baseMint, poolAddress)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	quoteVault, err := cp_amm.DeriveTokenVaultAddress(quoteMint, poolAddress)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	var instructions []solana.Instruction

	baseTokenAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, payer, baseMint, payer, &instructions)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	quoteTokenAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, payer, quoteMint, payer, &instructions)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	if baseMint.Equals(solana.WrappedSol) {
		if initialPoolTokenBaseAmount.Cmp(decimal.Zero) <= 0 {
			return nil, solana.PublicKey{}, fmt.Errorf("amountIn must be greater than 0")
		}

		wrapSOLIx := system.NewTransferInstruction(
			initialPoolTokenBaseAmount.BigInt().Uint64(),
			payer,
			baseTokenAccount,
		).Build()

		// sync the WSOL account to update its balance
		syncNativeIx := token.NewSyncNativeInstruction(
			baseTokenAccount,
		).Build()

		instructions = append(instructions, wrapSOLIx, syncNativeIx)
	}

	if quoteMint.Equals(solana.WrappedSol) {
		if initialPoolTokenQuoteAmount.Cmp(decimal.Zero) <= 0 {
			return nil, solana.PublicKey{}, fmt.Errorf("amountIn must be greater than 0")
		}

		wrapSOLIx := system.NewTransferInstruction(
			initialPoolTokenQuoteAmount.BigInt().Uint64(),
			payer,
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
		return nil, solana.PublicKey{}, err
	}
	quoteTokenBadge, err := cp_amm.DeriveTokenBadgeAddress(quoteMint)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}
	tokenBadgeAccounts = append(tokenBadgeAccounts, solana.NewAccountMeta(baseTokenBadge, false, false))
	tokenBadgeAccounts = append(tokenBadgeAccounts, solana.NewAccountMeta(quoteTokenBadge, false, false))

	createIx, err := cp_amm.NewInitializePoolInstruction(
		// Params:
		&cp_amm.InitializePoolParameters{
			Liquidity:       u128.GenUint128FromString(liquidityDelta.String()),
			SqrtPrice:       u128.GenUint128FromString(initSqrtPrice.String()),
			ActivationPoint: activationPoint,
		},

		// Accounts:
		poolCreator,
		positionNft,
		positionNftAccount,
		payer,
		config,
		poolAuthority,
		poolAddress,
		position,
		baseMint,
		quoteMint,
		baseVault,
		quoteVault,
		baseTokenAccount,
		quoteTokenAccount,
		tokenBaseProgram,
		tokenQuoteProgram,
		solana.Token2022ProgramID,
		solana.SystemProgramID,
		eventAuthority,
		cp_amm.ProgramID,
		tokenBadgeAccounts,
	)

	if err != nil {
		return nil, solana.PublicKey{}, err
	}
	instructions = append(instructions, createIx)

	if baseMint.Equals(solana.WrappedSol) {
		unwrapIx := token.NewCloseAccountInstruction(
			baseTokenAccount,
			payer,
			payer,
			nil,
		).Build()
		instructions = append(instructions, unwrapIx)
	}

	if quoteMint.Equals(solana.WrappedSol) {
		unwrapIx := token.NewCloseAccountInstruction(
			quoteTokenAccount,
			payer,
			payer,
			nil,
		).Build()
		instructions = append(instructions, unwrapIx)
	}

	if isLockLiquidity {
		lockIx, err := cp_amm.NewPermanentLockPositionInstruction(
			// Params:
			u128.GenUint128FromString(liquidityDelta.String()),

			// Accounts:
			poolAddress,
			position,
			positionNftAccount,
			poolCreator,
			eventAuthority,
			cp_amm.ProgramID,
		)
		if err != nil {
			return nil, solana.PublicKey{}, err
		}
		instructions = append(instructions, lockIx)
	}
	return instructions, poolAddress, nil
}

// CreatePool Creates a new standard pool according to a predefined configuration.
// The function depends on CreatePoolInstruction.
// The function is blocking; it will wait for on-chain confirmation before returning.
//
// Example:
//
// baseAmount := big.NewInt(1_000_000)
//
// quoteAmount := big.NewInt(1) // SOL
//
// sig, cpammPool, positionNft, _ := meteoraDammV2.CreatePool(
//
//	ctx1,
//	wsClient,
//	payer, // payer account
//	0, // configIndex = 0 // https://docs.meteora.ag/developer-guide/guides/damm-v2/pool-fee-configs#view-all-public-config-key-addresses-json
//	1, // 1 base token = 1 quote token
//	baseMint, // baseMintToken
//	solana.WrappedSol, // quoteMintToken
//	baseAmount, // baseMintAmount
//	quoteAmount, // quoteMintAmount
//	nil,
//	true,
//
// )
func (m *DammV2) CreatePool(
	ctx context.Context,
	wsClient *ws.Client,
	payer *solana.Wallet,
	configIndex uint64, // https://docs.meteora.ag/developer-guide/guides/damm-v2/pool-fee-configs#view-all-public-config-key-addresses-json
	initialPrice float64, // 1 base token = 1 quote token
	baseMint solana.PublicKey,
	quoteMint solana.PublicKey,
	baseAmount *big.Int,
	quoteAmount *big.Int,
	activationPoint *uint64,
	isLockLiquidity bool,
) (string, solana.PublicKey, *solana.Wallet, error) {
	config, err := cp_amm.DeriveConfigAddress(configIndex)
	if err != nil {
		return "", solana.PublicKey{}, nil, err
	}
	configState, err := m.GetConfig(ctx, config)
	if err != nil {
		return "", solana.PublicKey{}, nil, err
	}

	positionNft := solana.NewWallet()
	instructions, cpammPool, err := CreatePoolInstruction(
		ctx,
		m.rpcClient,
		payer.PublicKey(),
		m.poolCreator.PublicKey(),
		config,
		configState,
		positionNft.PublicKey(),
		initialPrice,
		baseMint,
		quoteMint,
		baseAmount,
		quoteAmount,
		activationPoint,
		isLockLiquidity,
	)
	if err != nil {
		return "", solana.PublicKey{}, nil, err
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
			case key.Equals(m.poolCreator.PublicKey()):
				return &m.poolCreator.PrivateKey
			case key.Equals(positionNft.PublicKey()):
				return &positionNft.PrivateKey
			default:
				return nil
			}
		},
	)
	if err != nil {
		return "", solana.PublicKey{}, nil, err
	}
	return sig.String(), cpammPool, positionNft, nil
}

// CreateCustomizablePoolWithDynamicConfigInstruction generates the instruction required for CreateCustomizablePoolWithDynamicConfig
// The function includes the creation of the ATA account.
//
// Example:
//
// config, _ := cp_amm.DeriveConfigAddress(configIndex)
//
// configState, _ := m.GetConfig(ctx, config)
//
// baseFeeParam, _ := cp_amm.GetBaseFeeParams(maxBaseFeeBps, minBaseFeeBps, feeSchedulerMode, numberOfPeriod, totalDuration)
//
// var dynamicFeeParam *cp_amm.DynamicFeeParameters
//
//	if useDynamicFee {
//		dynamicFeeParam, _ = cp_amm.GetDynamicFeeParams(minBaseFeeBps, cp_amm.MAX_PRICE_CHANGE_BPS_DEFAULT)
//	}
//
//	poolFees := cp_amm.PoolFeeParameters{
//		BaseFee:    *baseFeeParam,
//		DynamicFee: dynamicFeeParam,
//		Padding:    [3]uint8{},
//	}
//
// positionNft := solana.NewWallet()
//
// instructions, cpammPool, _ := CreateCustomizablePoolWithDynamicConfigInstruction(
//
//	ctx,
//	m.rpcClient,
//	payer.PublicKey(), // payer account
//	m.poolCreator.PublicKey(), // pool creator account
//	config, // damm v2 config address
//	configState, // damm v2 config state
//	positionNft.PublicKey(), // position of the pool creator account
//	poolCreatorAuthority.PublicKey(), // authority of the pool creator account
//	initialPrice, // 1 base token = 1 quote token
//	baseMint, // baseMintToken
//	quoteMint, // quoteMintToken
//	baseAmount, // baseMintAmount
//	quoteAmount, // quoteMintAmount
//	hasAlphaVault,
//	activationType, // 0.ActivationTypeSlot or 1.ActivationTypeTimestamp
//	collectFeeMode, // 0.CollectFeeModeBothToken 1.CollectFeeModeTokenA 2.CollectFeeModeTokenB
//	activationPoint,
//	poolFees,
//	isLockLiquidity,
//
// )
func CreateCustomizablePoolWithDynamicConfigInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	payer solana.PublicKey,
	poolCreator solana.PublicKey,
	config solana.PublicKey,
	configState *cp_amm.Config,
	positionNft solana.PublicKey,
	poolCreatorAuthority solana.PublicKey,
	initialPrice float64, // 1 base token = 1 quote token
	baseMint solana.PublicKey,
	quoteMint solana.PublicKey,
	baseAmount *big.Int,
	quoteAmount *big.Int,
	hasAlphaVault bool,
	activationType cp_amm.ActivationType,
	collectFeeMode cp_amm.CollectFeeMode,
	activationPoint *uint64,
	poolFees cp_amm.PoolFeeParameters,
	isLockLiquidity bool,
) ([]solana.Instruction, solana.PublicKey, error) {

	tokens, err := solanago.GetMultipleToken(ctx, rpcClient, baseMint, quoteMint)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}
	if tokens[0] == nil || tokens[1] == nil {
		return nil, solana.PublicKey{}, fmt.Errorf("baseMint or quoteMint error")
	}

	tokenBaseProgram := tokens[0].Owner

	tokenQuoteProgram := tokens[1].Owner

	tokenBaseDecimals := tokens[0].Decimals
	tokenQuoteDecimals := tokens[1].Decimals

	initialPoolTokenBaseAmount := cp_amm.GetInitialPoolTokenAmount(decimal.NewFromBigInt(baseAmount, 0), tokenBaseDecimals)
	initialPoolTokenQuoteAmount := cp_amm.GetInitialPoolTokenAmount(decimal.NewFromBigInt(quoteAmount, 0), tokenQuoteDecimals)

	initSqrtPrice, err := cp_amm.GetSqrtPriceFromPrice(decimal.NewFromFloat(initialPrice), tokenBaseDecimals, tokenQuoteDecimals)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	liquidityDelta := cp_amm.GetLiquidityDelta(
		initialPoolTokenBaseAmount,
		initialPoolTokenQuoteAmount,
		decimal.NewFromBigInt(configState.SqrtMaxPrice.BigInt(), 0),
		decimal.NewFromBigInt(configState.SqrtMinPrice.BigInt(), 0),
		initSqrtPrice,
	)

	position, err := cp_amm.DerivePositionAddress(positionNft)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	positionNftAccount, err := cp_amm.DerivePositionNftAccount(positionNft)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	poolAddress, err := cp_amm.DeriveCpAmmPoolPDA(config, quoteMint, baseMint)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	baseVault, err := cp_amm.DeriveTokenVaultAddress(baseMint, poolAddress)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}
	quoteVault, err := cp_amm.DeriveTokenVaultAddress(quoteMint, poolAddress)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	var instructions []solana.Instruction

	baseTokenAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, payer, baseMint, payer, &instructions)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	quoteTokenAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, payer, quoteMint, payer, &instructions)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}

	if baseMint.Equals(solana.WrappedSol) {
		if initialPoolTokenBaseAmount.Cmp(decimal.Zero) <= 0 {
			return nil, solana.PublicKey{}, fmt.Errorf("amountIn must be greater than 0")
		}

		wrapSOLIx := system.NewTransferInstruction(
			initialPoolTokenBaseAmount.BigInt().Uint64(),
			payer,
			baseTokenAccount,
		).Build()

		// sync the WSOL account to update its balance
		syncNativeIx := token.NewSyncNativeInstruction(
			baseTokenAccount,
		).Build()

		instructions = append(instructions, wrapSOLIx, syncNativeIx)
	}

	if quoteMint.Equals(solana.WrappedSol) {
		if initialPoolTokenQuoteAmount.Cmp(decimal.Zero) <= 0 {
			return nil, solana.PublicKey{}, fmt.Errorf("amountIn must be greater than 0")
		}

		wrapSOLIx := system.NewTransferInstruction(
			initialPoolTokenQuoteAmount.BigInt().Uint64(),
			payer,
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
		return nil, solana.PublicKey{}, err
	}
	quoteTokenBadge, err := cp_amm.DeriveTokenBadgeAddress(quoteMint)
	if err != nil {
		return nil, solana.PublicKey{}, err
	}
	tokenBadgeAccounts = append(tokenBadgeAccounts, solana.NewAccountMeta(baseTokenBadge, false, false))
	tokenBadgeAccounts = append(tokenBadgeAccounts, solana.NewAccountMeta(quoteTokenBadge, false, false))

	createIx, err := cp_amm.NewInitializePoolWithDynamicConfigInstruction(
		// Params:
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

		// Accounts:
		poolCreator,
		positionNft,
		positionNftAccount,
		payer,
		poolCreatorAuthority,
		config,
		poolAuthority,
		poolAddress,
		position,
		baseMint,
		quoteMint,
		baseVault,
		quoteVault,
		baseTokenAccount,
		quoteTokenAccount,
		tokenBaseProgram,
		tokenQuoteProgram,
		solana.Token2022ProgramID,
		solana.SystemProgramID,
		eventAuthority,
		cp_amm.ProgramID,
		tokenBadgeAccounts,
	)

	if err != nil {
		return nil, solana.PublicKey{}, err
	}
	instructions = append(instructions, createIx)

	if baseMint.Equals(solana.WrappedSol) {
		unwrapIx := token.NewCloseAccountInstruction(
			baseTokenAccount,
			payer,
			payer,
			nil,
		).Build()
		instructions = append(instructions, unwrapIx)
	}

	if quoteMint.Equals(solana.WrappedSol) {
		unwrapIx := token.NewCloseAccountInstruction(
			quoteTokenAccount,
			payer,
			payer,
			nil,
		).Build()
		instructions = append(instructions, unwrapIx)
	}

	if isLockLiquidity {
		lockIx, err := cp_amm.NewPermanentLockPositionInstruction(
			// Params:
			u128.GenUint128FromString(liquidityDelta.String()),

			// Accounts:
			poolAddress,
			position,
			positionNftAccount,
			poolCreator,
			eventAuthority,
			cp_amm.ProgramID,
		)
		if err != nil {
			return nil, solana.PublicKey{}, err
		}
		instructions = append(instructions, lockIx)
	}
	return instructions, poolAddress, nil
}

// CreateCustomizablePoolWithDynamicConfig
// The function depends on CreateCustomizablePoolWithDynamicConfigInstruction.
// The function is blocking; it will wait for on-chain confirmation before returning.
// This function requires contacting meteora to get a dedicated configIndex. The configuration of this configIndex fixes the poolCreatorAuthority within it.
//
// # This function has not completed verification
//
// Example:
//
// baseAmount := big.NewInt(1_000_000)
// quoteAmount := big.NewInt(1) // SOL
// sig, cpammPool, positionNft, _ := meteoraDammV2.CreateCustomizablePoolWithDynamicConfig(
//
//	ctx1,
//	wsClient,
//	payer, // payer account
//	1, // dedicated configIndex obtained by contacting meteora
//	poolCreatorAuthority, // poolCreatorAuthority configured in configIndex
//	1, // 1 base token = 1 quote token
//	baseMint, // baseMintToken
//	solana.WrappedSol, // quoteMintToken
//	baseAmount, // baseMintAmount
//	quoteAmount, // quoteMintAmount
//	false,
//	cp_amm.ActivationTypeTimestamp, // 0.ActivationTypeSlot or 1.ActivationTypeTimestamp
//	cp_amm.CollectFeeModeBothToken, // 0.CollectFeeModeBothToken 1.CollectFeeModeTokenA 2.CollectFeeModeTokenB
//	nil,
//	true,
//	5000, // 50%
//	25,   // 0.25%
//	cp_amm.FeeSchedulerModeExponential,
//	60,   // 60 peridos
//	3600, // 60 * 60
//	true,
//
// )
func (m *DammV2) CreateCustomizablePoolWithDynamicConfig(
	ctx context.Context,
	wsClient *ws.Client,
	payer *solana.Wallet,
	configIndex uint64,
	poolCreatorAuthority *solana.Wallet,
	initialPrice float64, // 1 base token = 1 quote token
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
) (string, solana.PublicKey, *solana.Wallet, error) {
	config, err := cp_amm.DeriveConfigAddress(configIndex)
	if err != nil {
		return "", solana.PublicKey{}, nil, err
	}
	configState, err := m.GetConfig(ctx, config)
	if err != nil {
		return "", solana.PublicKey{}, nil, err
	}

	baseFeeParam, err := cp_amm.GetBaseFeeParams(maxBaseFeeBps, minBaseFeeBps, feeSchedulerMode, numberOfPeriod, totalDuration)
	if err != nil {
		return "", solana.PublicKey{}, nil, err
	}

	var dynamicFeeParam *cp_amm.DynamicFeeParameters
	if useDynamicFee {
		dynamicFeeParam, err = cp_amm.GetDynamicFeeParams(minBaseFeeBps, cp_amm.MAX_PRICE_CHANGE_BPS_DEFAULT)
		if err != nil {
			return "", solana.PublicKey{}, nil, err
		}
	}

	poolFees := cp_amm.PoolFeeParameters{
		BaseFee:    *baseFeeParam,
		DynamicFee: dynamicFeeParam,
		Padding:    [3]uint8{},
	}

	positionNft := solana.NewWallet()

	instructions, cpammPool, err := CreateCustomizablePoolWithDynamicConfigInstruction(
		ctx,
		m.rpcClient,
		payer.PublicKey(),
		m.poolCreator.PublicKey(),
		config,
		configState,
		positionNft.PublicKey(),
		poolCreatorAuthority.PublicKey(),
		initialPrice,
		baseMint,
		quoteMint,
		baseAmount,
		quoteAmount,
		hasAlphaVault,
		activationType,
		collectFeeMode,
		activationPoint,
		poolFees,
		isLockLiquidity,
	)
	if err != nil {
		return "", solana.PublicKey{}, nil, err
	}
	// {
	// 	sig, err := solanago.SendTransaction(ctx,
	// 		m.rpcClient,
	// 		m.wsClient,
	// 		instructions[:4],
	// 		payer.PublicKey(),
	// 		func(key solana.PublicKey) *solana.PrivateKey {
	// 			switch {
	// 			case key.Equals(payer.PublicKey()):
	// 				return &payer.PrivateKey
	// 			case key.Equals(poolCreatorAuthority.PublicKey()):
	// 				return &poolCreatorAuthority.PrivateKey
	// 			case key.Equals(m.poolCreator.PublicKey()):
	// 				return &m.poolCreator.PrivateKey
	// 			case key.Equals(positionNft.PublicKey()):
	// 				return &positionNft.PrivateKey
	// 			default:
	// 				return nil
	// 			}
	// 		},
	// 	)
	// 	if err != nil {
	// 		return "", solana.PublicKey{}, nil, err
	// 	}
	// 	fmt.Println("sig", sig.String())
	// }

	sig, err := solanago.SendTransaction(ctx,
		m.rpcClient,
		wsClient,
		append([]solana.Instruction{computebudget.NewSetComputeUnitLimitInstruction(500_000).Build()}, instructions[4:]...),
		payer.PublicKey(),
		func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(payer.PublicKey()):
				return &payer.PrivateKey
			case key.Equals(poolCreatorAuthority.PublicKey()):
				return &poolCreatorAuthority.PrivateKey
			case key.Equals(m.poolCreator.PublicKey()):
				return &m.poolCreator.PrivateKey
			case key.Equals(positionNft.PublicKey()):
				return &positionNft.PrivateKey
			default:
				return nil
			}
		},
	)
	if err != nil {
		return "", solana.PublicKey{}, nil, err
	}
	return sig.String(), cpammPool, positionNft, nil
}

// GetPools gets all pools
// It depends on the GetPools function.
// This function will iterate through all pools. Use it sparingly unless necessary.
//
// Example:
//
// pools, _:= meteoraDammV2.GetPools(ctx)
func (m *DammV2) GetPools(ctx context.Context) (map[solana.PublicKey]*Pool, error) {
	return GetPools(ctx, m.rpcClient)
}

// GetPools gets all pools
// This function will iterate through all pools. Use it sparingly unless necessary.
//
// Example:
//
// pools, _:= GetPools(ctx,rpcClient)
func GetPools(
	ctx context.Context,
	rpcClient *rpc.Client,
) (map[solana.PublicKey]*Pool, error) {
	opt := solanago.GenProgramAccountFilter(cp_amm.AccountKeyPool, nil)

	outs, err := rpcClient.GetProgramAccountsWithOpts(ctx, cp_amm.ProgramID, opt)
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

// GetPoolsByBaseMint gets pools by baseMint
// It depends on the GetPoolsByBaseMint function.
//
// Example:
//
// pools, _:= meteoraDammV2.GetPoolsByBaseMint(ctx,baseMint)
func (m *DammV2) GetPoolsByBaseMint(
	ctx context.Context,
	baseMint solana.PublicKey,
) ([]*Pool, error) {
	return GetPoolsByBaseMint(ctx, m.rpcClient, baseMint)
}

// GetPoolsByBaseMint gets pools by baseMint
//
// Example:
//
// pools, _:= GetPoolsByBaseMint(ctx,rpcClient,baseMint)
func GetPoolsByBaseMint(
	ctx context.Context,
	rpcClient *rpc.Client,
	baseMint solana.PublicKey,
) ([]*Pool, error) {

	opt := solanago.GenProgramAccountFilter(cp_amm.AccountKeyPool, &solanago.Filter{
		Owner:  baseMint,
		Offset: solanago.ComputeStructOffset(new(cp_amm.Pool), "TokenAMint"),
	})

	outs, err := rpcClient.GetProgramAccountsWithOpts(ctx, cp_amm.ProgramID, opt)
	if err != nil {
		if err == rpc.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}

	if len(outs) == 0 {
		return nil, nil
	}

	var pools []*Pool
	for _, out := range outs {
		obj, err := cp_amm.ParseAnyAccount(out.Account.Data.GetBinary())
		if err != nil {
			return nil, err
		}
		pool, ok := obj.(*cp_amm.Pool)
		if !ok {
			return nil, fmt.Errorf("obj.(*cp_amm.Pool) fail")
		}
		pools = append(pools, &Pool{pool, out.Pubkey})
	}

	return pools, nil
}
