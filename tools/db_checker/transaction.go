package main

import (
	"internal/shamap"
	"internal/utils"
	"log"
	"slices"
	"sync"

	"github.com/gocql/gocql"
)

func TraverseTxHashFromDB(session *gocql.Session, ledgerIndex uint64, skipSha bool, skipAccountTxCheck bool, skipNFT bool, fixNFTUri bool, skipDiff bool) string {
	var hashes [][]byte
	scanner := session.Query("select hash from ledger_transactions where ledger_sequence = ?", ledgerIndex).Iter().Scanner()
	for scanner.Next() {
		var hash []byte
		err := scanner.Scan(&hash)
		if err != nil {
			log.Printf("Error: ledger_transactions reading %d", ledgerIndex)
			log.Println(err)
		}
		hashes = append(hashes, hash)
	}

	var ptrTxMap *shamap.GoSHAMap
	if !skipSha {
		txMap := shamap.MakeSHAMapTxMeta()
		ptrTxMap = &txMap
	}
	var tx []byte
	var metadata []byte
	var createdIndexes [][]byte
	var deletedIndexes [][]byte
	var updatedIndexes [][]byte
	for _, hash := range hashes {
		err := session.Query(`select transaction,metadata from transactions where hash = ?`,
			hash).Scan(&tx, &metadata)
		if err != nil {
			log.Printf("Error: Transactions reading %x %v", hash, err)
			continue
		}
		if !skipSha {
			ptrTxMap.AddTxItem(tx, uint32(len(tx)), metadata, uint32(len(metadata)))
		}
		if !skipAccountTxCheck {
			checkAccountTx(session, ledgerIndex, tx, metadata)
		}
		if !skipNFT {
			checkNFT(session, ledgerIndex, tx, metadata, fixNFTUri)
		}
		if !skipDiff {
			diff := checkDiff(session, ledgerIndex, tx, metadata)
			createdIndexes = append(createdIndexes, diff.CreatedIndexes...)
			deletedIndexes = append(deletedIndexes, diff.DeletedIndexes...)
			updatedIndexes = append(updatedIndexes, diff.UpdatedIndexes...)
		}
	}

	//diff check
	if !skipDiff {
		// find the indexes created and deleted in the same ledger , these are not diffs
		notExistIndexes := make(map[[32]byte]int)
		for _, created := range createdIndexes {
			for _, deleted := range deletedIndexes {
				if slices.Compare(created, deleted) == 0 {
					log.Printf("Index created and deleted in ledger %d: %x\n", ledgerIndex, created)
					notExistIndexes[[32]byte(created)] = 1
				}
			}
		}

		allIndexes := append(append(createdIndexes, deletedIndexes...), updatedIndexes...)
		for _, index := range allIndexes {
			if _, ok := notExistIndexes[[32]byte(index)]; !ok {
				var count int
				err := session.Query(`select count(*) from diff where seq = ? and key = ?`, ledgerIndex, index).Scan(&count)
				if err != nil {
					log.Fatalf("Error: %v diff reading ledger %d index %x", err, ledgerIndex, index)
				}
				if count == 0 {
					log.Printf("Error: diff not found for index %x in ledger %d\n", index, ledgerIndex)
				}
			}
		}
	}

	hashFromMap := ""
	if !skipSha {
		hashFromMap = ptrTxMap.GetHash()
		ptrTxMap.Free()
	}
	return hashFromMap
}

func checkDiff(session *gocql.Session, ledgerIndex uint64, tx []byte, metadata []byte) utils.Diffs {
	const MAX_ITEM = 100
	diffData := utils.GetDiffs(tx, uint32(len(tx)), metadata, uint32(len(metadata)), MAX_ITEM)
	if len(diffData.CreatedIndexes) == MAX_ITEM || len(diffData.DeletedIndexes) == MAX_ITEM || len(diffData.UpdatedIndexes) == MAX_ITEM {
		log.Printf("Error: too many diffs in tx from ledger %d", ledgerIndex)
	}
	return diffData
}

func checkNFT(session *gocql.Session, ledgerIndex uint64, tx []byte, metadata []byte, fixUri bool) {
	// It may have multiple nft tx in one transaction, eg cancel offers
	const MAX_ITEM = 100
	nftTxData, nftData := utils.GetNFT(tx, uint32(len(tx)), metadata, uint32(len(metadata)), MAX_ITEM)

	if len(nftTxData) == MAX_ITEM {
		log.Printf("Error: too many NFT tx in ledger %d\n", ledgerIndex)
	}

	//nf_token_transactions
	for _, nft := range nftTxData {
		var count int
		err := session.Query(`select count(*) from nf_token_transactions where token_id = ? and seq_idx = (?,?)`, nft.TokenId, ledgerIndex, nft.TxIdx).Scan(&count)
		if err != nil {
			log.Fatalf("Error: %v nf_token_transactions reading %x ledger %d txId %d", err, nft.TokenId, ledgerIndex, nft.TxIdx)
		}
		if count == 0 {
			log.Printf("Error: nf_token_transactions not found for nft %x ledger %d txId %d\n", nft.TokenId, ledgerIndex, nft.TxIdx)
		}
	}

	for _, nft := range nftData {
		//nf_tokens
		var count int
		err := session.Query(`select count(*) from nf_tokens where token_id = ? and sequence = ?`, nft.TokenId, ledgerIndex).Scan(&count)
		if err != nil {
			log.Fatalf("Error: %v nf_tokens reading %x ledger %d", err, nft.TokenId, ledgerIndex)
		}
		if count == 0 {
			log.Printf("Error: nf_tokens not found for nft %x ledger %d\n", nft.TokenId, ledgerIndex)
		}

		// nf_token_uris and issuer_nf_tokens_v2
		if nft.UriExists {
			err := session.Query(`select count(*) from nf_token_uris where token_id = ? and sequence = ?`, nft.TokenId, ledgerIndex).Scan(&count)
			if err != nil {
				log.Fatalf("Error: %v nf_token_uris reading %x ledger %d", err, nft.TokenId, ledgerIndex)
			}
			if count == 0 {
				log.Printf("Error: nf_token_uris not found for nft %x ledger %d\n", nft.TokenId, ledgerIndex)
				if fixUri {
					err = session.Query("insert into nf_token_uris (token_id, sequence, uri) values (?, ?, ?)", nft.TokenId, ledgerIndex, nft.Uri).Exec()
					if err != nil {
						log.Fatalf("Error: nf_token_uris insert %x uri: %x : %d, %v", nft.TokenId, nft.Uri, ledgerIndex, err)
					}
					// double confirm if the insert is successful
					session.Query(`select count(*) from nf_token_uris where token_id = ? and sequence = ?`, nft.TokenId, ledgerIndex).Scan(&count)
					if count == 1 {
						log.Printf("Success: nf_token_uris fixed %x uri: %x : %d", nft.TokenId, nft.Uri, ledgerIndex)
					} else {
						log.Printf("Failed: nf_token_uris fixed %x uri: %x : %d", nft.TokenId, nft.Uri, ledgerIndex)
					}
				}
			}

			err = session.Query(`select count(*) from issuer_nf_tokens_v2 where issuer = ? and taxon = ? and token_id = ?`,
				nft.Issuer, nft.Taxon, nft.TokenId).Scan(&count)
			if err != nil {
				log.Fatalf("Error: %v issuer_nf_tokens_v2 reading issuer %x taxon %d token_id %x", err, nft.Issuer, nft.Taxon, nft.TokenId)
			}
			if count == 0 {
				log.Printf("Error: issuer_nf_tokens_v2 not found for issuer %x taxon %d token_id %x\n", nft.Issuer, nft.Taxon, nft.TokenId)
			}
		}

	}
}

func checkAccountTx(session *gocql.Session, ledgerIndex uint64, tx []byte, metadata []byte) {
	const MAX_ACCOUNTS = 1000
	accounts, txIdx := utils.GetAffectAccountsFromTx(tx, uint32(len(tx)), metadata, uint32(len(metadata)), MAX_ACCOUNTS)
	if len(accounts) == MAX_ACCOUNTS {
		log.Printf("Error: too many accounts in ledger %d tx %d\n", ledgerIndex, txIdx)
	}
	for _, account := range accounts {
		var count int
		err := session.Query(`select count(*) from account_tx where account = ? and seq_idx = (?,?)`, account, ledgerIndex, txIdx).Scan(&count)
		if err != nil {
			log.Fatalf("Error: %v account_tx reading %x ledger %d txId %d", err, account, ledgerIndex, txIdx)
		}
		if count == 0 {
			log.Printf("Error: account_tx not found for account %x ledger %d txId %d\n", account, ledgerIndex, txIdx)
		}
	}
}

func getHashesFromLedgerHeader(session *gocql.Session, ledgerIndex uint64) (string, string) {

	var header []byte
	err := session.Query("select header from ledgers where sequence = ?", ledgerIndex).Scan(&header)

	if err != nil {
		log.Printf("Error: %v ledgers reading %d", err, ledgerIndex)
		return "", ""
	}

	txHash := utils.GetTxHashFromLedgerHeader(header, uint32(len(header)))
	stateHash := utils.GetStatesHashFromLedgerHeader(header, uint32(len(header)))
	return stateHash, txHash
}

func checkingTransactionsFromLedger(session *gocql.Session, startLedgerIndex uint64, endLedgerIndex uint64, step int, skipSHA bool, skipAccount bool, skipNFT bool, fixNFTUri bool, skipDiff bool) uint64 {
	ledgerIndex := endLedgerIndex
	var mismatch uint64 = 0

	for ledgerIndex >= startLedgerIndex {
		thisStep := min(step, int(ledgerIndex-startLedgerIndex+1))
		var wg sync.WaitGroup
		wg.Add(thisStep)

		for i := 0; i < thisStep; i++ {
			seq := ledgerIndex - uint64(i)
			go func() {
				txHashFromDB := TraverseTxHashFromDB(session, seq, skipSHA, skipAccount, skipNFT, fixNFTUri, skipDiff)

				if !skipSHA {
					_, txHash := getHashesFromLedgerHeader(session, seq)
					if txHash != txHashFromDB {
						mismatch++
						log.Printf("Error: Tx hash mismatch for ledger %d: %s != %s\n", seq, txHash, txHashFromDB)
					} else {
						log.Printf("Tx hash for ledger %d is correct: %s\n", seq, txHash)
					}
				} else {
					log.Printf("Finish checking tx existence for ledger %d\n", seq)
				}
				wg.Done()
			}()
		}
		wg.Wait()
		ledgerIndex -= uint64(thisStep)
	}

	return mismatch
}
