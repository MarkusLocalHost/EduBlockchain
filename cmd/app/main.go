package main

import (
	"RestEduBlockchain/internal/models"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"
)

// Blockchain is a series of validated Blocks
var Blockchain *models.BlockChain
var tempBlocks []*models.Block

// candidateBlocks handles incoming blocks for validation
var candidateBlocks = make(chan *models.Block)

// announcements broadcasts winning validator to all nodes
var announcements = make(chan string)

var mutex = &sync.Mutex{}

// validators keeps track of open validators and balances
var validators = make(map[string]int)

func main() {
	wallets, _ := models.CreateWallets()
	address := wallets.AddWallet()
	wallets.SaveFile()
	Blockchain = models.InitBlockChain(address)
	defer Blockchain.Database.Close()

	// wallet
	w := models.MakeWallet()
	w.Address()

	// start TCP and serve TCP server
	server, err := net.Listen("tcp", ":9000")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("TCP Server Listening on port :9000")
	defer server.Close()

	go func() {
		for candidate := range candidateBlocks {
			mutex.Lock()
			tempBlocks = append(tempBlocks, candidate)
			mutex.Unlock()
		}
	}()

	go func() {
		for {
			pickWinner()
		}
	}()

	for {
		conn, err := server.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleConn(conn)
	}
}

// pickWinner creates a lottery pool of validators and chooses the validator who gets to forge a block to the blockchain
// by random selecting from the pool, weighted by amount of tokens staked
func pickWinner() {
	time.Sleep(30 * time.Second)
	mutex.Lock()
	temp := tempBlocks
	mutex.Unlock()

	var lotteryPool []string
	if len(temp) > 0 {

		// slightly modified traditional proof of stake algorithm
		// from all validators who submitted a block, weight them by the number of staked tokens
		// in traditional proof of stake, validators can participate without submitting a block to be forged
	OUTER:
		for _, block := range temp {
			// if already in lottery pool, skip
			for _, node := range lotteryPool {
				if block.Validator == node {
					continue OUTER
				}
			}

			// lock list of validators to prevent data race
			mutex.Lock()
			setValidators := validators
			mutex.Unlock()

			k, ok := setValidators[block.Validator]
			if ok {
				for i := 0; i < k; i++ {
					lotteryPool = append(lotteryPool, block.Validator)
				}
			}
		}

		// randomly pick winner from lottery pool
		s := rand.NewSource(time.Now().Unix())
		r := rand.New(s)
		lotteryWinner := lotteryPool[r.Intn(len(lotteryPool))]

		// add block of winner to blockchain and let all the other nodes know
		for _, block := range temp {
			if block.Validator == lotteryWinner {
				mutex.Lock()

				Blockchain.AddBlock(block.Transactions, block.Validator)

				mutex.Unlock()
				for _ = range validators {
					announcements <- "\nwinning validator: " + lotteryWinner + "\n"
				}
				break
			}
		}
	}

	mutex.Lock()
	tempBlocks = []*models.Block{}
	mutex.Unlock()
}

func handleConn(conn net.Conn) {
	defer conn.Close()

	go func() {
		for {
			msg := <-announcements
			io.WriteString(conn, msg)
		}
	}()
	// validator address
	var addressFrom = "1BwGQM3WQSWtESCSvYu1g6ts5eQXmFukkV"
	var addressTo string

	// allow user to allocate number of tokens to stake
	// the greater the number of tokens, the greater chance to forging a new block
	io.WriteString(conn, "Enter token balance: ")
	scanBalance := bufio.NewScanner(conn)
	for scanBalance.Scan() {
		balance, err := strconv.Atoi(scanBalance.Text())
		if err != nil {
			log.Printf("%v not a number: %v", scanBalance.Text(), err)
			return
		}

		validators[addressFrom] = balance
		fmt.Println(validators)
		break
	}

	// address that receive token
	io.WriteString(conn, "Enter address to send token: ")
	scanAddressTo := bufio.NewScanner(conn)
	for scanAddressTo.Scan() {
		addressTo = scanAddressTo.Text()

		fmt.Println(addressTo)
		break
	}

	// allow user to allocate number of tokens to stake
	// the greater the number of tokens, the greater chance to forging a new block
	io.WriteString(conn, "Enter amount of tokens: ")
	scanBPM := bufio.NewScanner(conn)
	go func() {
		// take in BPM from stdin and add it to blockchain after conducting necessary validation
		for scanBPM.Scan() {
			tokenAmount, err := strconv.Atoi(scanBPM.Text())
			// if malicious party tries to mutate the chain with a bad input, delete them as a validator and they lose their staked tokens
			if err != nil {
				log.Printf("%v not a number: %v", scanBPM.Text(), err)
				delete(validators, addressFrom)
				conn.Close()
			}

			mutex.Lock()
			oldLastIndex := Blockchain.LastIndex
			oldLastHash := Blockchain.LastHash
			mutex.Unlock()

			tx := models.NewTransaction(addressFrom, addressTo, tokenAmount, Blockchain)
			fmt.Println(tx.String())

			// create newBlock for consideration to be forged
			newBlock, err := models.GenerateBlock(oldLastIndex, oldLastHash, []*models.Transaction{tx}, addressFrom)
			if err != nil {
				log.Println(err)
				continue
			}
			if IsBlockValid(newBlock, oldLastIndex, oldLastHash) {
				candidateBlocks <- newBlock
			}
		}
	}()

	// simulate receiving broadcast
	for {
		time.Sleep(time.Minute)
		mutex.Lock()
		output, err := json.Marshal(Blockchain)
		mutex.Unlock()
		if err != nil {
			log.Fatal(err)
		}
		io.WriteString(conn, string(output)+"\n")
	}

}

// IsBlockValid makes sure block is valid by checking index
// and comparing the hash of the previous block
func IsBlockValid(newBlock *models.Block, lastIndex uint64, lastHash []byte) bool {
	if lastIndex+1 != newBlock.Index {
		return false
	}

	if string(lastHash) != string(newBlock.PrevHash) {
		return false
	}

	if string(newBlock.CalculateBlockHash()) != string(newBlock.Hash) {
		return false
	}
	return true
}
