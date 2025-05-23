package tradeLoop

import (
	"context"
	"fmt"
	"log"
	"time"

	"arbTracker/configLoad"
	onchainSMB "arbTracker/onchain"
	"arbTracker/types"
)

// StartTradeLoop starts a background loop that sends transactions every TradeInterval ms
func StartTradeLoop(ctx context.Context, config configLoad.Config) {
	go func() {
		ticker := time.NewTicker(time.Duration(config.LoopInterval) * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Println("Trade loop shutting down.")
				return
			case <-ticker.C:
				trades := types.GetAllConfigs() // Safely snapshot current TradeConfigs

				for _, trade := range trades {
					go func(t types.TradeConfig) {
						tradesPerInterval := pruneAndCalculateVolatility(trade.Mint, config.ObservationWindow)
						if tradesPerInterval > 0 {
							fmt.Println("Bin changes per interval", tradesPerInterval)
						}

						if tradesPerInterval > config.BinMovesInWindow {
							onchainSMB.SendTx(config, t, CurrentMultiplier)
						}

					}(trade)
				}
			}
		}
	}()
}
