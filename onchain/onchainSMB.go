package onchainSMB

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"arbTracker/alt"
	blockhashrefresh "arbTracker/blockhashRefresh"
	"arbTracker/configLoad"
	"arbTracker/globals"
	"arbTracker/types"

	"github.com/gagliardetto/solana-go"                                        // Updated import
	compute_budget "github.com/gagliardetto/solana-go/programs/compute-budget" // Updated import
	"github.com/gagliardetto/solana-go/rpc"
	// Updated import
)

// Struct to match the private key JSON format
type KeyFile struct {
	PrivateKey string `json:"private_key"`
}

type TokenPool struct {
	XMint          solana.PublicKey
	WalletXAccount solana.PublicKey //E1SsaCT4eaKATYKKDb9RCWAf9HsdTN5w3uhvvNgHyqbd
	RaydiumPools   []types.RaydiumPool
	RaydiumCPPools []types.RaydiumCPPool
	PumpPools      []types.PumpPool
	DLMMPairs      []types.DLMMTriple
	RayCpmm        []types.RaydiumCPPool
	AmmMeteora     []types.AMMMeteora
}

// Instruction Builder
func GenerateOnchainSwapInstruction(
	wallet, solMint, walletSolAccount solana.PublicKey,
	tokenProgram, systemProgram, associatedTokenProgram solana.PublicKey,
	tokenPools []TokenPool,
	programId solana.PublicKey,
	minimumProfit, maxBinToProcess uint64,
) *solana.GenericInstruction {

	feeCollector := solana.MustPublicKeyFromBase58("6AGB9kqgSp2mQXwYpdrV4QVV8urvCaDS35U1wsLssy6H")
	// xmint := solana.MustPublicKeyFromBase58("DPTP4fUfWuwVTgCmttWBu6Sy5B9TeCTBjc2YKgpDpump")

	accounts := []*solana.AccountMeta{
		solana.NewAccountMeta(wallet, true, true),
		solana.NewAccountMeta(solMint, false, false),
		solana.NewAccountMeta(feeCollector, true, false),
		solana.NewAccountMeta(walletSolAccount, true, false),
		solana.NewAccountMeta(tokenProgram, false, false),
		solana.NewAccountMeta(systemProgram, false, false),
		solana.NewAccountMeta(associatedTokenProgram, false, false), ///////
		// solana.NewAccountMeta(solana.MustPublicKeyFromBase58("LBUZKhRxPF3XUpBCjp4YzTKgLccjZhTSDM9YuVaPwxo"), false, false),
	}

	for _, pool := range tokenPools {
		accounts = append(accounts,
			solana.NewAccountMeta(pool.XMint, false, false),
			solana.NewAccountMeta(pool.WalletXAccount, true, false),
		)

		for _, cp := range pool.RayCpmm {
			accounts = append(accounts,
				solana.NewAccountMeta(cp.RayProgramId, false, false),
				solana.NewAccountMeta(cp.RayEventAuthority, false, false),
				solana.NewAccountMeta(cp.Pool, true, false),
				solana.NewAccountMeta(cp.AmmConfig, false, false),
				solana.NewAccountMeta(cp.XVault, true, false),
				solana.NewAccountMeta(cp.SOLVault, true, false),
				solana.NewAccountMeta(cp.Observation, true, false),
			)
		}

		// for _, cp := range pool.RaydiumCPPools {
		// 	accounts = append(accounts,
		// 		solana.NewAccountMeta(cp.ProgramId, false, false),
		// 		solana.NewAccountMeta(cp.Authority, false, false),
		// 		solana.NewAccountMeta(cp.Pool, false, false),
		// 		solana.NewAccountMeta(cp.AMMConfig, false, false),
		// 		solana.NewAccountMeta(cp.XVault, false, false),
		// 		solana.NewAccountMeta(cp.SOLVault, false, false),
		// 		solana.NewAccountMeta(cp.Observation, false, false),
		// 	)
		// }

		for _, p := range pool.PumpPools {
			accounts = append(accounts,
				solana.NewAccountMeta(p.ProgramId, false, false),
				solana.NewAccountMeta(p.GlobalConfig, false, false),
				solana.NewAccountMeta(p.Authority, false, false),
				solana.NewAccountMeta(p.FeeWallet, false, false),
				solana.NewAccountMeta(p.Pool, false, false),
				solana.NewAccountMeta(p.XAccount, true, false),
				solana.NewAccountMeta(p.SOLAccount, true, false),
				solana.NewAccountMeta(p.FeeTokenWallet, true, false),
				solana.NewAccountMeta(p.CreatorFeeAta, true, false),
				solana.NewAccountMeta(p.CreatorVA, true, false),
			)
		}

		for _, d := range pool.DLMMPairs {
			accounts = append(accounts,
				solana.NewAccountMeta(d.DlmmProgramId, false, false),
				solana.NewAccountMeta(d.DlmmEventAuthority, true, false),
				solana.NewAccountMeta(d.Pair, true, false),
				solana.NewAccountMeta(d.XVault, true, false),
				solana.NewAccountMeta(d.SOLVault, true, false),
				solana.NewAccountMeta(d.Oracle, true, false),
			)
			for i := range d.BinArrays {
				accounts = append(accounts, &d.BinArrays[i])
			}
		}

		for _, p := range pool.AmmMeteora {
			accounts = append(accounts,
				solana.NewAccountMeta(p.AMMProgramId, false, false),
				solana.NewAccountMeta(p.AMMVault, false, false),
				solana.NewAccountMeta(p.Pool, true, false),
				solana.NewAccountMeta(p.XVault, true, false),
				solana.NewAccountMeta(p.SOLVault, true, false),
				solana.NewAccountMeta(p.XTokenVault, true, false),
				solana.NewAccountMeta(p.SOLTokenVault, true, false),
				solana.NewAccountMeta(p.XLpMint, true, false),
				solana.NewAccountMeta(p.SOLLpMint, true, false),
				solana.NewAccountMeta(p.XPoolLp, true, false),
				solana.NewAccountMeta(p.SOLPoolLp, true, false),
				solana.NewAccountMeta(p.AdminTokenFeeX, true, false),
				solana.NewAccountMeta(p.AdminTokenFeeSOL, true, false),
			)
		}
	}
	// Prepare data
	data := []byte{15}
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, minimumProfit)
	data = append(data, buf...)
	binary.LittleEndian.PutUint64(buf, maxBinToProcess)
	data = append(data, buf...)

	// Directly return the GenericInstruction type
	instruction := solana.NewInstruction(
		programId, // Program ID
		accounts,  // Accounts
		data,      // Data
	)

	return instruction // Return the *solana.GenericInstruction

}

func SendTx(config configLoad.Config, tradeDetails types.TradeConfig, multiplier float64) { //solana.Signature {
	start := time.Now()
	rand.Seed(time.Now().UnixNano())
	walletPubkey := globals.PrivateKey.PublicKey()

	// wallet := solana.MustPublicKeyFromBase58(walletPubkey)
	mint := solana.MustPublicKeyFromBase58(tradeDetails.Mint)

	// t1 := time.Now()
	ata, _, err := solana.FindAssociatedTokenAddress(walletPubkey, mint)
	if err != nil {
		log.Fatalf("failed to derive ATA: %v", err)
	}

	// Dummy example public keys (replace with actual ones)
	solMint := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	walletSolAccount := solana.MustPublicKeyFromBase58("6MF8zKwWjrg5Rbt5We8y9ypu7aQzGbc2JHrAZSJiBNCF")
	tokenProgram := solana.TokenProgramID
	systemProgram := solana.SystemProgramID
	associatedTokenProgram := solana.MustPublicKeyFromBase58("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL")
	programId := solana.MustPublicKeyFromBase58("MEViEnscUm6tsQRoGd9h6nLQaQspKj7DB2M5FwM3Xvz") // your on-chain arb program

	// Example tokenPools slice (empty here — fill with actual pools)
	var tokenPools []TokenPool

	pool := TokenPool{
		XMint:          mint,
		WalletXAccount: ata,                    //solana.MustPublicKeyFromBase58("E1SsaCT4eaKATYKKDb9RCWAf9HsdTN5w3uhvvNgHyqbd"),
		RaydiumPools:   tradeDetails.Raydium,   // Not used in this tx
		RayCpmm:        tradeDetails.RaydiumCP, // Not used in this tx
		PumpPools:      tradeDetails.Pump,
		DLMMPairs:      tradeDetails.DLMM,
		AmmMeteora:     tradeDetails.MeteoraAmm,
	}

	tokenPools = append(tokenPools, pool)

	// Call your function
	instruction := GenerateOnchainSwapInstruction(
		walletPubkey,
		solMint,
		walletSolAccount,
		tokenProgram,
		systemProgram,
		associatedTokenProgram,
		tokenPools,
		programId,
		1000, // minimumProfit
		50,   // maxBinToProcess
	)

	// t2 := time.Now()
	// fmt.Println(int(float64(tradeDetails.CUPrice) * float64(multiplier)))
	// fmt.Println(globals.Min(int(float64(tradeDetails.CUPrice)*float64(multiplier)), config.MaxCUPrice))
	priceIx := compute_budget.NewSetComputeUnitPriceInstruction(uint64(globals.Min(int(float64(tradeDetails.CUPrice+uint64(rand.Intn(int(float64(tradeDetails.CUPrice)*0.3))))*float64(multiplier)), config.MaxCUPrice))).Build() // 5000 µlamports = 0.000005 SOL per CU
	limitIx := compute_budget.NewSetComputeUnitLimitInstruction(uint32(config.CuLimit)).Build()

	// t25 := time.Now()

	tx, err := solana.NewTransaction(
		[]solana.Instruction{limitIx, priceIx, instruction}, // pass your instruction here
		blockhashrefresh.GetCachedBlockhash(),
		solana.TransactionPayer(walletPubkey),
		solana.TransactionAddressTables(map[solana.PublicKey]solana.PublicKeySlice{
			solana.MustPublicKeyFromBase58(config.AltAddress):                              alt.AltMap[solana.MustPublicKeyFromBase58(config.AltAddress)].Addresses, // include all, or just filtered subset
			solana.MustPublicKeyFromBase58("4sKLJ1Qoudh8PJyqBeuKocYdsZvxTcRShUt9aKqwhgvC"): alt.AltMap[solana.MustPublicKeyFromBase58("4sKLJ1Qoudh8PJyqBeuKocYdsZvxTcRShUt9aKqwhgvC")].Addresses,
		}),
	)
	if err != nil {
		log.Fatalf("failed to create transaction: %v", err)
	}
	tx.Message.SetVersion(solana.MessageVersionV0)

	// t3 := time.Now()
	// Sign the transaction
	_, err = tx.Sign(
		func(key solana.PublicKey) *solana.PrivateKey {
			if key.Equals(walletPubkey) {
				return globals.PrivateKey
			}
			return nil
		},
	)
	if err != nil {
		log.Fatalf("failed to sign transaction: %v", err)
	}

	BlastTx(tx, start, config)
}

func BlastTx(tx *solana.Transaction, start time.Time, config configLoad.Config) {
	var wg sync.WaitGroup

	// Send it
	for _, x := range globals.SendClient {
		wg.Add(1)
		go func(c *rpc.Client) {

			txSig, err := x.SendTransactionWithOpts(
				context.TODO(),
				tx,
				rpc.TransactionOpts{
					SkipPreflight:       true,
					PreflightCommitment: rpc.CommitmentFinalized,
				},
			)
			if err != nil {
				log.Fatalf("failed to send transaction: %v", err)
			}
			finish := time.Now()
			if config.ShowTx {
				fmt.Println(time.Now().Format("2006-01-02 15:04:05.999"), " Transaction sent:", txSig, "Total tx submission time: ", finish.Sub(start))
			}
		}(x)
	}
	// return txSig
	wg.Wait()

}
