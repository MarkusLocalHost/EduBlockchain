package main

import (
	wallets2 "RestEduBlockchain/internal/models"
	"fmt"
)

func main() {
	wallets, _ := wallets2.CreateWallets()
	address := wallets.AddWallet()
	wallets.SaveFile()

	fmt.Printf("New address is: %s\n", address)
}
