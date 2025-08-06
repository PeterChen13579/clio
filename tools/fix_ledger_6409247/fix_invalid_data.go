package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gocql/gocql"
)

var (
	clusterHosts   = kingpin.Arg("hosts", "Scylla nodes IP addresses, comma separated (i.e. 192.168.1.1,192.168.1.2)").Required().String()
	keyspace       = kingpin.Flag("keyspace", "Keyspace to use").Short('k').Default("clio_fh").String()
	userName       = kingpin.Flag("username", "Username to use when connecting to the cluster").String()
	password       = kingpin.Flag("password", "Password to use when connecting to the cluster").String()
	clusterTimeout = kingpin.Flag("timeout", "Maximum duration for query execution in millisecond").Short('t').Default("90000").Int()
)

// The specific data that needs to be added to the DB.
const (
	targetLedgerSequence    = 6409247
	targetTxnHashString     = "DB53018AAFBD1E4AE9C69C82AAB6535E9A9A969F1133E4F79DAE58319C9CFD63"
	targetDate              = 452387070
	targetMetadataString    = "201c00000001f8e5110061250061be185590c8c28e62bb62b457ef5df5bda49730a5b39b49d8dcc9deef9c6722634e279d5628eb3885f52be0d9aa0ceec4a1323f25b825f92c2c5dd65b963c1bfcaf4b6c47e662400000043657d6aae1e7220000000024000000482d0000000d62400000043cb5c74c81148797a79e6019eac76398cb302ab02262400281ffe1e1e5110061250061cc1e5558b1463a9bf2986e0528e738e42c7772cbf7081913041dbf56570c97dd0e6794562cf524584fc1dda14abf3af2134f54f29ba2ad9206bd9436e4426a3afbd298dce6240011ae406240001100525f9272e1e72200000000240011ae412d0000000062400011004c01a0d68114921a8d4667fec23671825d2fdde08b719dc77c25e1e1f1031000"
	targetTransactionString = "1200002200000000240011ae405011360bdce9a57b49f786f0d9bc9eed4d14000000000000000000000000000000006140000000065df0a26840000000000000fa732103d7895a6ae8f78a1b136903abccdc7eceb1c7c5b2a37dbe47b6c268b9cb461571744730450220185d3c4579a083c3aa25824b8c62a8e5a9d07f43420e89afc3e2ef71592a4862022100ef0e11f275d29296daac59685dfa00ae89dfb1b6c334c9617c87ccc4c2688ab58114921a8d4667fec23671825d2fdde08b719dc77c2583148797a79e6019eac76398cb302ab02262400281ff"
)

func insertLedgerWithTransaction(session *gocql.Session, ledgerSeq uint64, txHashHex string) error {
	txHashBytes, err := hex.DecodeString(txHashHex)
	if err != nil {
		return fmt.Errorf("failed to decode transaction hash '%s': %w", txHashHex, err)
	}

	log.Printf("Attempting to insert new row into 'ledger_transactions' table for ledger %d...", ledgerSeq)

	query := `INSERT INTO ledger_transactions (ledger_sequence, hash) VALUES (?, ?)`
	if err := session.Query(query, ledgerSeq, txHashBytes).Exec(); err != nil {
		return fmt.Errorf("failed to execute insert for 'ledgers' table on ledger %d: %w", ledgerSeq, err)
	}
	log.Printf("Successfully inserted row into 'ledger_transactions' table for ledger %d.", ledgerSeq)
	return nil
}

func insertTransactionRow(session *gocql.Session, ledgerSeq uint64, date int32, hash, metadata, transaction string) error {
	hashBytes, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("failed to decode hash: %w", err)
	}
	metadataBytes, err := hex.DecodeString(metadata)
	if err != nil {
		return fmt.Errorf("failed to decode metadata: %w", err)
	}
	transactionBytes, err := hex.DecodeString(transaction)
	if err != nil {
		return fmt.Errorf("failed to decode transaction: %w", err)
	}

	log.Printf("Attempting to insert row into 'transactions' table for ledger %d...", ledgerSeq)

	query := `INSERT INTO transactions (hash, date, ledger_sequence, metadata, transaction) VALUES (?, ?, ?, ?, ?)`

	if err := session.Query(query, hashBytes, date, ledgerSeq, metadataBytes, transactionBytes).Exec(); err != nil {
		return fmt.Errorf("failed to execute insert for 'transactions' table on ledger %d: %w", ledgerSeq, err)
	}

	log.Printf("Successfully inserted row into 'transactions' table for ledger %d.", ledgerSeq)
	return nil
}

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	kingpin.Parse()

	hosts := strings.Split(*clusterHosts, ",")
	cluster := gocql.NewCluster(hosts...)
	cluster.Timeout = time.Duration(*clusterTimeout * 1000 * 1000)
	cluster.NumConns = 1
	cluster.Keyspace = *keyspace

	if *userName != "" {
		cluster.Authenticator = gocql.PasswordAuthenticator{
			Username: *userName,
			Password: *password,
		}
	}

	session, err := cluster.CreateSession()
	if err != nil {
		log.Fatalf("Could not connect to database: %v", err)
	}
	defer session.Close()

	log.Println("Database session created successfully.")

	// Insert the new ledger row with its first transaction ---
	if err := insertLedgerWithTransaction(session, targetLedgerSequence, targetTxnHashString); err != nil {
		log.Fatalf("Failed to insert ledger row: %v", err)
	}

	// Insert the full transaction row into the transactions table ---
	if err := insertTransactionRow(session, targetLedgerSequence, targetDate, targetTxnHashString, targetMetadataString, targetTransactionString); err != nil {
		log.Fatalf("Failed to insert transaction row: %v", err)
	}

	log.Println("All operations completed successfully.")
}
