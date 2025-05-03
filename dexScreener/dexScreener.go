package dexScreener

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
)

type DexScreenerResponse []struct {
	DexId       string `json:"dexId"`
	PairAddress string `json:"pairAddress"`
	QuoteToken  struct {
		Address string `json:"address"`
	} `json:"quoteToken"`
	Volume struct {
		M5 float64 `json:"m5"`
	} `json:"volume"`
	Liquidity struct {
		USD   float64 `json:"usd"`
		Quote float64 `json"quote"`
	} `json:"liquidity"`
	BaseToken struct {
		Symbol string `json:"symbol"`
	} `json:"baseToken"`
}

type DexPairData struct {
	Symbol   string   `json:"symbol"`
	TokenCa  string   `json:"tokenCa"`
	Pumpswap []string `json:"pumpswap"`
	Raydium  []string `json:"raydium"`
	Meteora  []string `json:"meteora"`
	Volume   float64  `json:"volume"`
}

type ByLiquidityQuote DexScreenerResponse

func (a ByLiquidityQuote) Len() int      { return len(a) }
func (a ByLiquidityQuote) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByLiquidityQuote) Less(i, j int) bool {
	return a[i].Liquidity.Quote > a[j].Liquidity.Quote // descending
}

func (d DexScreenerResponse) Len() int {
	return len(d)
}

func (d DexScreenerResponse) Less(i, j int) bool {
	return d[i].Volume.M5 > d[j].Volume.M5 // Descending
}

func (d DexScreenerResponse) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}

func QueryDexScreener(token_ca string, poolVolFilter float64, poolLiqFilter float64, TotVolumeFilter float64) DexPairData {
	fmt.Println("Querying dexscreener for pool other market information..")
	url := "https://api.dexscreener.com/token-pairs/v1/solana/" + token_ca

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Failed to fetch transaction details: %v", err)
		return DexPairData{}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read response body: %v", err)
		return DexPairData{}
	}
	// log.Println("Response body:", string(body)) // Debugging output

	var data DexScreenerResponse

	// Use a slice to correctly handle the JSON array response
	if err := json.Unmarshal(body, &data); err != nil {
		log.Printf("Failed to parse JSON: %v", err)
		fmt.Println("Response body:", string(body)) // Debugging output
		return DexPairData{}
	}
	// sort.Sort(data)
	sort.Sort(ByLiquidityQuote(data))

	// fmt.Println(data)

	validDexIds := map[string]bool{"pumpswap": true, "raydium": true, "meteora": true}
	var meteora []string
	var raydium []string
	var pumpswap []string
	var volume []float64

	for _, entry := range data {
		// fmt.Println(entry)
		if validDexIds[entry.DexId] &&
			entry.QuoteToken.Address == "So11111111111111111111111111111111111111112" &&
			entry.Volume.M5 >= poolVolFilter &&
			entry.Liquidity.Quote >= poolLiqFilter {

			if entry.DexId == "pumpswap" {
				pumpswap = append(pumpswap, entry.PairAddress)
			}
			if entry.DexId == "raydium" {
				raydium = append(raydium, entry.PairAddress)
			}
			if entry.DexId == "meteora" {
				meteora = append(meteora, entry.PairAddress)
			}

			volume = append(volume, entry.Volume.M5)
		}
	}

	// enable total volume filter (sum all volume)
	sumVol := 0.0
	for _, num := range volume {
		sumVol += num
	}

	var returnDexPairData DexPairData
	if sumVol >= TotVolumeFilter {
		returnDexPairData = DexPairData{
			Symbol:   data[0].BaseToken.Symbol,
			TokenCa:  token_ca,
			Pumpswap: pumpswap,
			Raydium:  raydium,
			Meteora:  meteora,
			Volume:   sumVol,
		}
	} else {
		returnDexPairData = DexPairData{}
	}

	fmt.Println("Pumpfun pools: ", pumpswap)
	fmt.Println("Raydium pools: ", raydium)
	fmt.Println("Meteora pools: ", meteora)

	return returnDexPairData

}
