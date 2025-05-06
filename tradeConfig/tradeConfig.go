package tradeConfig

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"arbTracker/alt"
	blockhashrefresh "arbTracker/blockhashRefresh"
	"arbTracker/configLoad"
	"arbTracker/fetcher"
	"arbTracker/globals"
	"arbTracker/tradeLoop"
	"arbTracker/types"

	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/rpc"
)

func PushToMaster(mints []types.HotMints, config configLoad.Config, ctx context.Context) {
	types.Mu.Lock()
	defer types.Mu.Unlock()

	// Build a lookup set from the incoming HotMints
	activeMints := make(map[string]bool)
	for _, mintStat := range mints {
		activeMints[mintStat.TokenCA] = true
	}

	// Update existing or append new entries
	newConfigs := make([]types.TradeConfig, 0, len(mints))
	for _, mintStat := range mints {
		updated := false

		ata, _ := GetOrCreateATA(ctx, globals.PrivateKey, solana.MustPublicKeyFromBase58(mintStat.TokenCA))

		for i := range types.TradeConfigs {
			if types.TradeConfigs[i].Mint == mintStat.TokenCA {
				// Update CU price and keep the config
				types.TradeConfigs[i].CUPrice = uint64(math.Round(mintStat.CuPrice))
				types.TradeConfigs[i].Ata = ata
				newConfigs = append(newConfigs, types.TradeConfigs[i])
				updated = true
				break
			}
		}

		if !updated {
			// Fetch and add new TradeConfig
			newConfig, err := fetcher.FetchTradeConfigForMint(mintStat, uint64(math.Round(mintStat.CuPrice)), config.PoolLiqFilter, config, ctx, ata)
			if err != nil {
				log.Printf("Failed to fetch config for mint %s: %v", mintStat.TokenCA, err)
				continue
			} else {
				// add alt
				newConfigs = append(newConfigs, *newConfig)

				// update flags for dynamic CU tracker
				if !tradeLoop.TradeActive { //no trades before, turned on now
					tradeLoop.LastTradeTime = time.Now()
					tradeLoop.TradeActive = true
				}
			}

		}
	}

	// if len(newConfigs) > 0 {
	// 	tradeLoop.SendTestTrades([]int{10000, 50000, 100000, 300000, 500000, 700000}, config, newConfigs[0])
	// }
	fmt.Println(len(newConfigs))
	if len(newConfigs) > 0 {
		var accountSub []solana.PublicKey

		for _, x := range newConfigs {
			for _, y := range x.DLMM {
				accountSub = append(accountSub, y.Pair)
			}
		}

		go fetcher.ReplaceDLMMAccountSubscriptions("main", accountSub)

		y := 3
		if y == 2 {
			altAdd, _ := solana.PublicKeyFromBase58(config.AltAddress)
			fmt.Println("Pushing new entries to ALT..", altAdd)
			alt.SendExtendALTTransaction(globals.RPCClient, altAdd, &solana.Wallet{
				PrivateKey: *globals.PrivateKey,
			}, newConfigs)

			fmt.Println("Refreshing ALT..")
			alt.InitLoadAlt(ctx, globals.RPCClient, altAdd.String())
		}
		types.TradeConfigs = newConfigs
		fmt.Println("uploaded trade list - spamming trades")
	}

	// Replace the global slice with the cleaned-up one
	// fmt.Println(types.TradeConfigs)
}

func GetOrCreateATA(
	ctx context.Context,
	payer *solana.PrivateKey, // wallet paying for the ATA creation and also the ATA owner
	mint solana.PublicKey,
) (solana.PublicKey, error) {
	// Derive the associated token address
	ata, _, err := solana.FindAssociatedTokenAddress(payer.PublicKey(), mint)
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("failed to derive ATA: %w", err)
	}

	// Check if ATA already exists
	accountInfo, err := globals.RPCClient.GetAccountInfo(ctx, ata)
	if err == nil && accountInfo != nil && accountInfo.Value != nil {
		// ATA already exists
		return ata, nil
	}

	// Build the create ATA instruction
	createATAIx := associatedtokenaccount.NewCreateInstruction(
		payer.PublicKey(), // funding address (payer)
		payer.PublicKey(), // wallet address (owner)
		mint,              // token mint
	).Build()

	// Build and sign transaction
	tx, err := solana.NewTransaction(
		[]solana.Instruction{createATAIx},
		blockhashrefresh.GetCachedBlockhash(),
		solana.TransactionPayer(payer.PublicKey()),
	)
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("failed to create transaction: %w", err)
	}

	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(payer.PublicKey()) {
			return payer
		}
		return nil
	})
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Send transaction
	sig, err := globals.RPCClient.SendTransactionWithOpts(ctx, tx, rpc.TransactionOpts{
		SkipPreflight:       true,
		PreflightCommitment: rpc.CommitmentFinalized,
	})
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("failed to send transaction: %w", err)
	}

	fmt.Printf("Created ATA %s with tx %s\n", ata.String(), sig.String())
	return ata, nil
}
