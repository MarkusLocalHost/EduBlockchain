package models

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"log"
	"time"
)

// Block represents each 'item' in the blockchain
type Block struct {
	Index        uint64
	Timestamp    string
	Transactions []*Transaction
	Hash         []byte
	PrevHash     []byte
	Validator    string
	Height       int
}

// GenerateBlock creates a new block using previous block's hash
func GenerateBlock(oldIndex uint64, oldHash []byte, tx []*Transaction, address string, height int) (*Block, error) {
	t := time.Now()

	newBlock := &Block{
		Index:        oldIndex + 1,
		Timestamp:    t.String(),
		Transactions: tx,
		Hash:         []byte{},
		PrevHash:     oldHash,
		Validator:    address,
		Height:       height,
	}
	newBlock.Hash = newBlock.CalculateBlockHash()

	return newBlock, nil
}

func (b *Block) Serialize() []byte {
	var res bytes.Buffer
	encoder := gob.NewEncoder(&res)

	err := encoder.Encode(b)
	if err != nil {
		log.Panic(err)
	}

	return res.Bytes()
}

func Deserialize(data []byte) *Block {
	var block Block

	decoder := gob.NewDecoder(bytes.NewReader(data))

	err := decoder.Decode(&block)
	if err != nil {
		log.Panic(err)
	}

	return &block
}

// CalculateHash is a simple SHA256 hashing function
func CalculateHash(s string) []byte {
	h := sha256.New()
	h.Write([]byte(s))
	hashed := h.Sum(nil)
	return hashed
}

func (b *Block) HashTransactions() []byte {
	var txHashes [][]byte
	var txHash [32]byte

	for _, tx := range b.Transactions {
		txHashes = append(txHashes, tx.ID)
	}
	txHash = sha256.Sum256(bytes.Join(txHashes, []byte{}))

	return txHash[:]
}

// CalculateBlockHash returns the hash of all block information
func (b *Block) CalculateBlockHash() []byte {
	record := string(rune(b.Index)) + b.Timestamp

	data := bytes.Join(
		[][]byte{
			CalculateHash(record),
			b.HashTransactions(),
			b.PrevHash,
		},
		[]byte{})

	return data
}
