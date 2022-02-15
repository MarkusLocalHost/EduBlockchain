package models

import (
	"github.com/dgraph-io/badger/v2"
	"log"
)

type BlockChainIterator struct {
	CurrentHash []byte
	Database    *badger.DB
}

func (iter *BlockChainIterator) Next() *Block {
	var block *Block
	var encodedBlock []byte

	err := iter.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get(iter.CurrentHash)
		if err != nil {
			log.Fatal(err)
		}

		err = item.Value(func(val []byte) error {
			encodedBlock = append([]byte{}, val...)

			return nil
		})

		block = Deserialize(encodedBlock)

		return err
	})
	if err != nil {
		log.Fatal(err)
	}

	iter.CurrentHash = block.PrevHash

	return block
}
