package initialize

import (
	"arbTracker/alt"
	blockhashrefresh "arbTracker/blockhashRefresh"
	"arbTracker/configLoad"
	"arbTracker/encryption"
	"arbTracker/globals"
	"arbTracker/tradeLoop"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"golang.org/x/net/context"
)

var err error

func Initialize(config configLoad.Config, ctx context.Context) {
	globals.RPCClient = rpc.New(config.RPCEndpoint)
	globals.TradeInterval = config.LoopInterval

	// // Setup logger
	// Logger = log.New(os.Stdout, "[arb] ", log.LstdFlags|log.Lshortfile)

	// Start background updater for blockhash
	go blockhashrefresh.StartBlockhashRefresher(ctx, globals.RPCClient)

	//load private key
	globals.PrivateKey, _ = encryption.DecryptAndLoadKeypair("private_key.json.enc", "Metal@@2")
	// globals.PrivateKey, _ = LoadKeypairFromJSON("private_key.json")
	if err != nil {
		log.Fatalf("failed to load private key: %v", err)
	}

	//load address table
	globals.AltAddress, _ = alt.LoadAlt(ctx, globals.RPCClient)
	if err != nil {
		log.Fatalf("failed to load ALT: %v", err)
	}

	globals.AltPubKey = solana.MustPublicKeyFromBase58("4sKLJ1Qoudh8PJyqBeuKocYdsZvxTcRShUt9aKqwhgvC")

	tradeLoop.StartTradeLoop(ctx, config)

}

func LoadKeypairFromJSON(filePath string) (*solana.PrivateKey, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read keypair file: %w", err)
	}

	var bytes []byte
	if err := json.Unmarshal(data, &bytes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal keypair JSON: %w", err)
	}

	kp := solana.PrivateKey(bytes)
	if err != nil {
		return nil, fmt.Errorf("invalid private key bytes: %w", err)
	}
	return &kp, nil
}
