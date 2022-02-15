package models

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/dgraph-io/badger/v2"
	"log"
)

// BadgerDB
const dbPath = "/home/markus/IDEA/Blockchain/RestEduBlockchain/database/blocks"

type BlockChain struct {
	LastIndex uint64
	LastHash  []byte
	Database  *badger.DB
}

func InitBlockChain(address string) *BlockChain {
	var lastHash []byte
	var lastIndexBytes []byte
	var lastIndex uint64

	db, err := badger.Open(badger.DefaultOptions(dbPath))
	if err != nil {
		log.Panic(err)
	}

	err = db.Update(func(txn *badger.Txn) error {
		if _, err := txn.Get([]byte("lh")); err == badger.ErrKeyNotFound {
			fmt.Println("No existing blockchain found")

			cntx := CoinbaseTX(address, "First transaction from Genesis")
			genesisBlock, _ := GenerateBlock(1, []byte{}, []*Transaction{cntx}, "first", 0)
			fmt.Println("Genesis proved")
			fmt.Printf("Genesis send coin to address: %s\n", address)

			err = txn.Set([]byte(genesisBlock.Hash), genesisBlock.Serialize())

			err = txn.Set([]byte("lh"), []byte(genesisBlock.Hash))

			b := make([]byte, 8)
			binary.LittleEndian.PutUint64(b, genesisBlock.Index)
			err = txn.Set([]byte("li"), b)

			lastHash = []byte(genesisBlock.Hash)
			lastIndex = genesisBlock.Index

			return err
		} else {
			item, err := txn.Get([]byte("lh"))
			if err != nil {
				log.Panic(err)
			}
			err = item.Value(func(val []byte) error {
				lastHash = append([]byte{}, val...)

				return nil
			})

			item, err = txn.Get([]byte("li"))
			if err != nil {
				log.Panic(err)
			}
			err = item.Value(func(val []byte) error {
				lastIndexBytes = append([]byte{}, val...)

				return nil
			})
			lastIndex = binary.LittleEndian.Uint64(lastIndexBytes)

			return err
		}
	})
	if err != nil {
		log.Panic(err)
	}

	blockchain := BlockChain{lastIndex, lastHash, db}
	return &blockchain
}

func (chain *BlockChain) AddBlock(transactions []*Transaction, address string) {
	var lastHash []byte
	var lastIndexBytes []byte
	var lastIndex uint64

	err := chain.Database.View(func(txn *badger.Txn) error {
		// get lastHash
		item, err := txn.Get([]byte("lh"))
		if err != nil {
			log.Panic(err)
		}

		err = item.Value(func(val []byte) error {
			lastHash = append([]byte{}, val...)

			return nil
		})

		// get lastIndex
		item, err = txn.Get([]byte("li"))
		if err != nil {
			log.Panic(err)
		}

		err = item.Value(func(val []byte) error {
			lastIndexBytes = append([]byte{}, val...)

			return nil
		})
		lastIndex = binary.LittleEndian.Uint64(lastIndexBytes)

		return err
	})
	if err != nil {
		log.Panic(err)
	}

	newBlock, err := GenerateBlock(lastIndex, lastHash, transactions, address)
	if err != nil {
		log.Panic(err)
	}

	err = chain.Database.Update(func(txn *badger.Txn) error {
		err = txn.Set(newBlock.Hash, newBlock.Serialize())

		err = txn.Set([]byte("lh"), newBlock.Hash)

		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, newBlock.Index)
		err = txn.Set([]byte("li"), b)

		chain.LastIndex = newBlock.Index
		chain.LastHash = newBlock.Hash

		return err
	})
	if err != nil {
		log.Panic(err)
	}
}

func (chain *BlockChain) Iterator() *BlockChainIterator {
	iter := &BlockChainIterator{
		CurrentHash: chain.LastHash,
		Database:    chain.Database,
	}

	return iter
}

func (chain *BlockChain) FindUTXO() map[string]TxOutputs {
	UTXO := make(map[string]TxOutputs)
	spentTXOs := make(map[string][]int)

	iter := chain.Iterator()
	for {
		block := iter.Next()

		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)

		Outputs:
			for outIdx, out := range tx.Outputs {
				if spentTXOs[txID] != nil {
					for _, spentOut := range spentTXOs[txID] {
						if spentOut == outIdx {
							continue Outputs
						}
					}
				}

				outs := UTXO[txID]
				outs.Outputs = append(outs.Outputs, out)
				UTXO[txID] = outs
			}

			if tx.isCoinbase() == false {
				for _, in := range tx.Inputs {
					inTxID := hex.EncodeToString(in.ID)
					spentTXOs[inTxID] = append(spentTXOs[inTxID], in.Out)
				}
			}
		}

		if len(block.PrevHash) == 0 {
			break
		}
	}

	return UTXO
}

//func (chain *BlockChain) FindUTXO(pubKeyHash []byte) []TxOutput {
//	var UTXOs []TxOutput
//	unspentTransactions := chain.FindUnspentTransactions(pubKeyHash)
//
//	for _, tx := range unspentTransactions {
//		for _, out := range tx.Outputs {
//			if out.IsLockedWithKey(pubKeyHash) {
//				UTXOs = append(UTXOs, out)
//			}
//		}
//	}
//
//	return UTXOs
//}
//
//func (chain *BlockChain) FindSpendableOutputs(pubKeyHash []byte, amount int) (int, map[string][]int) {
//	unspentOuts := make(map[string][]int)
//	unspentTxs := chain.FindUnspentTransactions(pubKeyHash)
//	accumulated := 0
//
//Work:
//	for _, tx := range unspentTxs {
//		txID := hex.EncodeToString(tx.ID)
//
//		for outIdx, out := range tx.Outputs {
//			if out.IsLockedWithKey(pubKeyHash) && accumulated < amount {
//				accumulated += out.Value
//				unspentOuts[txID] = append(unspentOuts[txID], outIdx)
//
//				if accumulated >= amount {
//					break Work
//				}
//			}
//		}
//	}
//
//	return accumulated, unspentOuts
//}

func (bc *BlockChain) FindTransaction(ID []byte) (Transaction, error) {
	iter := bc.Iterator()

	for {
		block := iter.Next()

		for _, tx := range block.Transactions {
			if bytes.Compare(tx.ID, ID) == 0 {
				return *tx, nil
			}
		}

		if len(block.PrevHash) == 0 {
			break
		}
	}

	return Transaction{}, errors.New("transactions does not exist")
}

func (bc *BlockChain) SignTransaction(tx *Transaction, privKey ecdsa.PrivateKey) {
	prevTXs := make(map[string]Transaction)

	for _, in := range tx.Inputs {
		prevTX, err := bc.FindTransaction(in.ID)
		if err != nil {
			log.Panic(err)
		}

		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}

	tx.Sign(privKey, prevTXs)
}

func (bc *BlockChain) VerifyTransaction(tx *Transaction) bool {
	prevTXs := make(map[string]Transaction)

	for _, in := range tx.Inputs {
		prevTX, err := bc.FindTransaction(in.ID)
		if err != nil {
			log.Panic(err)
		}

		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}

	return tx.Verify(prevTXs)
}
