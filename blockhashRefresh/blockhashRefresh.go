package blockhashrefresh

import (
	"context"
	"log"
	"time"

	"arbTracker/types"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func StartBlockhashRefresher(ctx context.Context, client *rpc.Client) {
	go func() {
		ticker := time.NewTicker(150 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				resp, err := client.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
				if err != nil {
					log.Printf("Blockhash fetch error: %v", err)
					continue
				}

				types.BlockhashMutex.Lock()
				types.LatestBlockhash = resp.Value.Blockhash
				types.BlockhashMutex.Unlock()
			}
		}
	}()
}

func GetCachedBlockhash() solana.Hash {
	types.BlockhashMutex.RLock()
	defer types.BlockhashMutex.RUnlock()
	return types.LatestBlockhash
}
