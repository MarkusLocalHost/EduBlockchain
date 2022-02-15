package utils

import "os"

const dbFile = "/home/markus/IDEA/Blockchain/RestEduBlockchain/tmp/blocks/MANIFEST"

func DBExists() bool {
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		return false
	}

	return true
}
