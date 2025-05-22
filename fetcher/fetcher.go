package fetcher

import (
	"arbTracker/configLoad"
	"arbTracker/dexScreener"
	"arbTracker/globals"
	"arbTracker/types"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// fetcher.go
func FetchTradeConfigForMint(hotMint types.HotMints, cuLimit uint64, poolLiqFilter float64, config configLoad.Config, ctx context.Context, ata solana.PublicKey) (*types.TradeConfig, error) {
	mktList := dexScreener.QueryDexScreener(hotMint.TokenCA, 0, poolLiqFilter, 0)

	var pump = []types.PumpPool{}
	var meteora = []types.DLMMTriple{}
	var rayCpmm = []types.RaydiumCPPool{}
	var meteoraAmm = []types.AMMMeteora{}
	// var meteoraAmm = []types.AMMMeteora{}
	// raydium, _ := FetchRaydiumPools(mktList.)
	// raydiumCP, _ := FetchRaydiumCPPools(mint)

	numDex := 0
	//pumpswap mkt vaults
	if len(mktList.Pumpswap) > 0 {
		pump = FetchPumpPools(mktList.TokenCa, mktList.Pumpswap, config)
		numDex = numDex + len(pump)
	}

	//meteora dlmm mkt vaults
	if len(mktList.Meteora) > 0 {
		meteora = FetchMeteoraDLMM(mktList.TokenCa, mktList.Meteora, config, ctx)
		numDex = numDex + len(meteora)
	}

	if len(mktList.MeteoraAmm) > 0 && numDex < 2 {
		meteoraAmm = FetchMeteoraAMM(mktList.TokenCa, mktList.MeteoraAmm, config)
		numDex = numDex + len(meteoraAmm)
	}
	if len(mktList.RaydiumCpmm) > 0 && numDex < 2 {
		rayCpmm = FetchRayCpmmPools(mktList.TokenCa, mktList.RaydiumCpmm, config)
		numDex = numDex + len(rayCpmm)
	}

	if numDex > 1 {

		// fmt.Println(meteora)

		return &types.TradeConfig{
			Mint:    mktList.TokenCa,
			CUPrice: uint64(hotMint.CuPrice),
			CULimit: cuLimit,
			Ata:     ata,
			// Raydium:   raydium,
			RaydiumCP:  rayCpmm,
			Pump:       pump,
			DLMM:       meteora,
			MeteoraAmm: meteoraAmm,
		}, nil
	} else {
		return &types.TradeConfig{}, errors.New("At least one pool needs to be available")
	}
}

func FetchPumpPools(tokenCA string, mktList []string, config configLoad.Config) []types.PumpPool {
	var returnPump []types.PumpPool

	for numMkts, x := range mktList {
		if numMkts < config.PumpMkts {
			pool, xAccount, sOLAccount, _ := getPoolTokens(x, tokenCA, config)

			data, _ := GetPumpMarketAccountData(globals.RPCClient, solana.MustPublicKeyFromBase58(pool))
			// fmt.Println(data)

			creator, _ := ExtractCreatorFromPumpMarket(data)
			feeVA, _, _ := DeriveCreatorVaultAuthority(creator)
			feeATA, _, _ := solana.FindAssociatedTokenAddress(feeVA, solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112"))
			tempPump := types.PumpPool{
				ProgramId:      solana.MustPublicKeyFromBase58("pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA"),  // Pump program ID
				GlobalConfig:   solana.MustPublicKeyFromBase58("ADyA8hdefvWN2dbGGWFotbzWxrAvLW83WG6QCVXvJKqw"), // Pump global config
				Authority:      solana.MustPublicKeyFromBase58("GS4CU59F31iL7aR2Q8zVS8DRrcRnXX1yjQ66TqNVQnaR"), // Pump authority
				FeeWallet:      solana.MustPublicKeyFromBase58("JCRGumoE9Qi5BBgULTgdgTLjSgkCMSbF62ZZfGs84JeU"), // Pump fee wallet
				Pool:           solana.MustPublicKeyFromBase58(pool),
				XAccount:       solana.MustPublicKeyFromBase58(xAccount),
				SOLAccount:     solana.MustPublicKeyFromBase58(sOLAccount),
				FeeTokenWallet: solana.MustPublicKeyFromBase58("DWpvfqzGWuVy9jVSKSShdM2733nrEsnnhsUStYbkj6Nn"),
				CreatorFeeAta:  feeATA,
				CreatorVA:      feeVA,
			}
			returnPump = append(returnPump, tempPump)
		}
	}

	return returnPump
}

func getPoolTokens(poolAddress string, token_ca string, config configLoad.Config) (string, string, string, error) {
	url := config.RPCEndpoint

	payload := strings.NewReader(fmt.Sprintf(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "getTokenAccountsByOwner",
		"params": [
			"%s",
			{"programId": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"},
			{"encoding": "jsonParsed"}
		]
	}`, poolAddress))

	resp, err := http.Post(url, "application/json", payload)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to query token accounts: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to read response: %v", err)
	}
	// fmt.Println(string(body))

	var result struct {
		Result struct {
			Value []struct {
				Pubkey  string `json:"pubkey"`
				Account struct {
					Data struct {
						Parsed struct {
							Info struct {
								Mint string `json:"mint"`
							} `json:"info"`
						} `json:"parsed"`
					} `json:"data"`
				} `json:"account"`
			} `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", "", fmt.Errorf("failed to parse response: %v", err)
	}

	var outputToken string
	var outputSol string
	for _, account := range result.Result.Value {
		if account.Account.Data.Parsed.Info.Mint == token_ca {
			outputToken = account.Pubkey
		}
		if account.Account.Data.Parsed.Info.Mint == "So11111111111111111111111111111111111111112" {
			outputSol = account.Pubkey
		}

	}
	// if len(tokens) > 0 {
	// 	tokens = append(tokens, poolAddress)
	// }
	return poolAddress, outputToken, outputSol, nil
}

func GetMarketAccountData(marketPDA solana.PublicKey) ([]byte, error) {
	info, err := globals.RPCClient.GetAccountInfoWithOpts(
		context.TODO(),
		marketPDA,
		&rpc.GetAccountInfoOpts{
			Encoding: "base64",
		},
	)
	if err != nil {
		return nil, err
	}

	rawData := info.Value.Data.GetBinary()

	return rawData, nil
}

func ExtractVaultData(data []byte, offset int) solana.PublicKey {
	if len(data) < 259 {
		return solana.PublicKey{}
	}
	return solana.PublicKeyFromBytes(data[offset : offset+32])
}

func GetPumpMarketAccountData(rpcClient *rpc.Client, marketPDA solana.PublicKey) ([]byte, error) {
	info, err := rpcClient.GetAccountInfoWithOpts(
		context.TODO(),
		marketPDA,
		&rpc.GetAccountInfoOpts{
			Encoding: "base64",
		},
	)
	if err != nil {
		return nil, err
	}

	rawData := info.Value.Data.GetBinary()

	return rawData, nil
}

func ExtractCreatorFromPumpMarket(data []byte) (solana.PublicKey, error) {
	if len(data) < 259 {
		return solana.PublicKey{}, fmt.Errorf("pool account too short")
	}
	return solana.PublicKeyFromBytes(data[211 : 211+32]), nil
}

func DeriveCreatorVaultAuthority(creator solana.PublicKey) (solana.PublicKey, uint8, error) {
	programID := solana.MustPublicKeyFromBase58("pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA")
	seeds := [][]byte{
		[]byte("creator_vault"), // fixed seed
		creator.Bytes(),         // coin creator's public key (dynamically derived)
	}
	return solana.FindProgramAddress(seeds, programID)
}
