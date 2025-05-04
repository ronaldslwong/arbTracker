package tradeLoop

import (
	"context"
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

				// for _, trade := range trades {
				if len(trades) > 0 {

					go func(t []types.TradeConfig) {
						onchainSMB.SendTx(config, t, CurrentMultiplier)

					}(trades)
				}
				// }
			}
		}
	}()
}
