package onchainSMB

import (
	"encoding/binary"
	"encoding/hex"

	"github.com/gagliardetto/solana-go"
)

func CreateKaminoBorrowInstruction(
	amount uint64,
	wallet solana.PublicKey,
) *solana.GenericInstruction {

	// append amount
	amt := make([]byte, 8)
	binary.LittleEndian.PutUint64(amt, amount)
	data, _ := hex.DecodeString("87e734a70734d4c1")
	data = append(data, amt...)
	data = append(data, 0x02)

	// data := append(discriminator, amountBytes...)

	accounts := []*solana.AccountMeta{
		solana.NewAccountMeta(wallet, true, true), //obligation
		solana.NewAccountMeta(solana.MustPublicKeyFromBase58("9DrvZvyWh1HuAoZxvYWMvkf2XCzryCpGgHqrMjyDWpmo"), false, false),
		solana.NewAccountMeta(solana.MustPublicKeyFromBase58("7u3HeHxYDLhnCoErrtycNokbQYbWGzLs6JSDqGAv5PfF"), false, false),
		solana.NewAccountMeta(solana.MustPublicKeyFromBase58("d4A2prbA2whesmvHaL88BH6Ewn5N4bTSU2Ze8P6Bc4Q"), true, false),
		solana.NewAccountMeta(solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112"), false, false),
		solana.NewAccountMeta(solana.MustPublicKeyFromBase58("GafNuUXj9rxGLn4y79dPu6MHSuPWeJR6UtTWuexpGh3U"), true, false),
		solana.NewAccountMeta(solana.MustPublicKeyFromBase58("6MF8zKwWjrg5Rbt5We8y9ypu7aQzGbc2JHrAZSJiBNCF"), true, false),
		solana.NewAccountMeta(solana.MustPublicKeyFromBase58("3JNof8s453bwG5UqiXBLJc77NRQXezYYEBbk3fqnoKph"), true, false),
		solana.NewAccountMeta(solana.MustPublicKeyFromBase58("KLend2g3cP87fffoy8q1mQqGKjrxjC8boSyAYavgmjD"), false, false),
		solana.NewAccountMeta(solana.MustPublicKeyFromBase58("KLend2g3cP87fffoy8q1mQqGKjrxjC8boSyAYavgmjD"), false, false),
		solana.NewAccountMeta(solana.MustPublicKeyFromBase58("Sysvar1nstructions1111111111111111111111111"), false, false),
		solana.NewAccountMeta(solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"), false, false),
	}

	return &solana.GenericInstruction{
		ProgID:        solana.MustPublicKeyFromBase58("KLend2g3cP87fffoy8q1mQqGKjrxjC8boSyAYavgmjD"),
		AccountValues: accounts,
		DataBytes:     data,
	}
}

func CreateKaminoRepayInstruction(
	amount uint64,
	wallet solana.PublicKey,
) *solana.GenericInstruction {

	amt := make([]byte, 8)
	binary.LittleEndian.PutUint64(amt, amount)
	data, _ := hex.DecodeString("b97500cb60f5b4ba")
	data = append(data, amt...)
	data = append(data, 0x02)

	accounts := []*solana.AccountMeta{
		solana.NewAccountMeta(wallet, true, true), //obligation
		solana.NewAccountMeta(solana.MustPublicKeyFromBase58("9DrvZvyWh1HuAoZxvYWMvkf2XCzryCpGgHqrMjyDWpmo"), false, false),
		solana.NewAccountMeta(solana.MustPublicKeyFromBase58("7u3HeHxYDLhnCoErrtycNokbQYbWGzLs6JSDqGAv5PfF"), false, false), //
		solana.NewAccountMeta(solana.MustPublicKeyFromBase58("d4A2prbA2whesmvHaL88BH6Ewn5N4bTSU2Ze8P6Bc4Q"), true, false),   //
		solana.NewAccountMeta(solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112"), false, false),
		solana.NewAccountMeta(solana.MustPublicKeyFromBase58("GafNuUXj9rxGLn4y79dPu6MHSuPWeJR6UtTWuexpGh3U"), true, false),
		solana.NewAccountMeta(solana.MustPublicKeyFromBase58("6MF8zKwWjrg5Rbt5We8y9ypu7aQzGbc2JHrAZSJiBNCF"), true, false),
		solana.NewAccountMeta(solana.MustPublicKeyFromBase58("3JNof8s453bwG5UqiXBLJc77NRQXezYYEBbk3fqnoKph"), true, false),
		solana.NewAccountMeta(solana.MustPublicKeyFromBase58("KLend2g3cP87fffoy8q1mQqGKjrxjC8boSyAYavgmjD"), false, false),
		solana.NewAccountMeta(solana.MustPublicKeyFromBase58("KLend2g3cP87fffoy8q1mQqGKjrxjC8boSyAYavgmjD"), false, false),
		solana.NewAccountMeta(solana.MustPublicKeyFromBase58("Sysvar1nstructions1111111111111111111111111"), false, false),
		solana.NewAccountMeta(solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"), false, false),
	}

	return &solana.GenericInstruction{
		ProgID:        solana.MustPublicKeyFromBase58("KLend2g3cP87fffoy8q1mQqGKjrxjC8boSyAYavgmjD"),
		AccountValues: accounts,
		DataBytes:     data,
	}
}
