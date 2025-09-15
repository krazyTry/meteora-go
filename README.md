# Meteora SDK library for Go

[![GoDoc](https://godoc.org/github.com/krazyTry/meteora-go?status.svg)](https://pkg.go.dev/github.com/krazyTry/meteora-go) 
[![GitHub tag (latest SemVer pre-release)](https://img.shields.io/github/v/tag/krazyTry/meteora-go?include_prereleases&label=release-tag)](https://github.com/krazyTry/meteora-go/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/krazyTry/meteora-go)](https://goreportcard.com/report/github.com/krazyTry/meteora-go)

Go SDK for interacting with the **Meteora Protocol** on Solana.  
Currently supports **Dynamic Bonding Curves (DBC)** and **Dynamic AMM (DAMM v2)**.

---

### Why a Go SDK?

Meteora already provides SDKs in **TypeScript** and **Rust**, but there are strong reasons to introduce a Go implementation:

- **TypeScript SDK**  
  - Runs on Node.js, which is single-threaded by design. Even with PM2 or clustering, performance does not match native backend languages.  
  - Browser-based usage introduces risks: creating DBC pools requires signatures from both the **pool creator** and **pool partner** accounts. Exposing these private keys client-side would allow attackers to steal partner fees.  

- **Rust SDK**  
  - Rust is primarily suited for **on-chain program development**. Custom contracts deployed by individuals often trigger **warning banners** on DEXs, reducing user trust.  
  - Resolving these warnings requires manual negotiations with each DEX, adding significant overhead. By using Meteora’s official contracts via this SDK, developers avoid these issues entirely.  

- **Go SDK**  
  - Go has been proven in production for over a decade — combining **high performance**, a **rich ecosystem**, and a **gentle learning curve**.  
  - Well-suited for backend systems, enabling **scalable, secure, and production-ready** integrations with the Meteora Protocol.  

> This SDK extends Meteora’s features (DBC, DAMM V2) into the Go ecosystem, empowering backend developers to build DeFi applications on Solana with confidence and efficiency.

---

<div align="center">
    <img src="https://user-images.githubusercontent.com/15271561/128235229-1d2d9116-23bb-464e-b2cc-8fb6355e3b55.png" margin="auto" height="175"/>
</div>

## Features

 * **Dynamic Bonding Curves (DBC) – Create and manage token launch pools with dynamic bonding curves.
 * **Dynamic AMM (DAMM V2) – Seamlessly migrate DBC pools to DAMM v2 and interact with dynamic automated market makers.
 * **Liquidity Management – Add or remove liquidity, manage positions, and claim accrued fees.
 * **Token Swaps – Execute buy/sell operations with slippage protection and accurate quotations.
 * **Multi-Token Support – Fully compatible with SPL tokens and Token-2022.
 * **Real-Time Quotes – Retrieve precise swap quotations before transaction execution.
 * **Fee Management – Calculate and collect dynamic fees automatically.

## Install

Run `go get github.com/krazyTry/meteora-go`

## Requirements 

Meteora SDK requires Go version `>=1.24.3`

## Documentation

https://pkg.go.dev/github.com/krazyTry/meteora-go


## Usage

### Dynamic Bonding Curve (DBC) Example

```go
package main

import (
	"context"
	"math/big"
	
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
	"github.com/krazyTry/meteora-go/dbc"
)

func main() {
	// Initialize RPC client
	rpcClient := rpc.New("https://api.mainnet-beta.solana.com")
	wsClient, _ := ws.Connect(context.Background(), "wss://api.mainnet-beta.solana.com")
	
	// Create DBC instance
	config := solana.NewWallet()
	poolCreator := solana.NewWallet()
	poolPartner := solana.NewWallet()
	leftoverReceiver := solana.NewWallet()
	
	meteoraDBC, _ := dbc.NewDBC(rpcClient, config, poolCreator, poolPartner, leftoverReceiver)
	
	// Get swap quote
	baseMint := solana.MustPublicKeyFromBase58("")
	amountIn := big.NewInt(1000000) // 1 token with 6 decimals
	slippageBps := uint64(250)      // 2.5% slippage
	
	result, poolState, configState, currentPoint, _ := meteoraDBC.SwapQuote(
		context.Background(),
		baseMint,
		false,        // buy (quote => base)
		amountIn,
		slippageBps,
		false,        // no referral
	)
	
	// Execute the swap
	buyer := solana.NewWallet()
	sig, _ := meteoraDBC.Buy(
		context.Background(),
		wsClient,
		buyer,
		nil, // no referral
		poolState.Address,
		poolState.VirtualPool,
		configState,
		amountIn,
		result.MinimumAmountOut,
		currentPoint,
	)
	
	fmt.Printf("Swap executed with signature: %s\n", sig)
}
```

## Related Projects

 * [gagliardetto/solana-go](https://github.com/gagliardetto/solana-go) - Core Solana Go SDK
 * [gagliardetto/anchor-go](https://github.com/gagliardetto/anchor-go) - Anchor framework bindings for Go
 * [gagliardetto/binary](https://github.com/gagliardetto/binary) - Binary encoding/decoding utilities
 * [Meteora Protocol](https://meteora.ag/) - Official Meteora Protocol website
 * [Meteora Docs](https://docs.meteora.ag/) - Protocol documentation
 * [Dynamic Bonding Curve SDK](https://github.com/MeteoraAg/dynamic-bonding-curve-sdk) - Typescript SDK for DBC
 * [DAMM v2 SDK](https://github.com/MeteoraAg/damm-v2-sdk) - Typescript SDK for DAMM v2
 * [Dynamic Bonding Curve SDK](https://github.com/MeteoraAg/dynamic-bonding-curve) - Rust SDK of DBC
 * [DAMM v2 SDK](https://github.com/MeteoraAg/damm-v2) - Rust SDK of DAMM v2

## FAQ

#### What's Dynamic Bonding Curves (DBC)?

[https://docs.meteora.ag/overview/products/dbc/what-is-dbc](https://docs.meteora.ag/overview/products/dbc/what-is-dbc)

#### What's DAMM v2?

[https://docs.meteora.ag/overview/products/damm-v2/what-is-damm-v2](https://docs.meteora.ag/overview/products/damm-v2/what-is-damm-v2)

#### How do I handle slippage in swaps?

The SDK provides built-in slippage protection through the `slippageBps` parameter in swap operations. Set this value based on your risk tolerance (e.g., 250 for 2.5% slippage). The SDK will calculate the minimum amount out and revert the transaction if slippage exceeds your limit.

#### Can I use this with Token-2022 program?

Yes! The Meteora SDK supports both standard SPL tokens and Token-2022 program tokens. The SDK automatically detects the token type and uses the appropriate program for operations.

#### How do I migrate from DBC to DAMM V2?

Migration can either be handled automatically by the Meteora platform, or manually using the SDK’s MigrateToDammV2 function. This function takes care of the key steps in transitioning a DBC pool to a full AMM pool, including liquidity transfer and pool state updates.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

**Note:** This SDK provides Go bindings for the Meteora Protocol on Solana and is **not affiliated with the official Meteora team**.