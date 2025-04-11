package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/gocql/gocql"
)

const (
	successorKeyNum        = 4000      // number of keys to query
	successorThreshHoldNum = 20        // The keys have to appear over successorThreshHoldNum times in the successor table
	maxSeq                 = 100000000 // 100 million
)

type queryInfo struct {
	Key []byte
	Seq uint64
}

func main() {
	// Set up Cassandra session
	username := flag.String("username", "", "Cassandra username")
	password := flag.String("password", "", "Cassandra password")
	clusterIP := flag.String("cluster", "", "Cassandra cluster IP")
	flag.Parse()

	if *username == "" || *password == "" || *clusterIP == "" {
		log.Fatal("You must provide --username, --password, and --cluster flags")
	}

	// Set up Cassandra session
	cluster := gocql.NewCluster(*clusterIP)
	cluster.Keyspace = "clio_fh"
	cluster.NumConns = 1
	cluster.Timeout = 9999999999
	cluster.Authenticator = gocql.PasswordAuthenticator{
		Username: *username,
		Password: *password,
	}
	cluster.Consistency = gocql.Quorum

	session, err := cluster.CreateSession()
	if err != nil {
		log.Fatal("Error connecting to Cassandra:", err)
	}
	defer session.Close()

	// Load successor data
	successorData := getSuccessorKeys(session)
	fmt.Println("Keys found:", len(successorData))
	session.SetPageSize(1)

	// Query the successor table in reverse order (current way)
	start := time.Now()
	for _, val := range successorData {
		session.Query("SELECT next FROM successor WHERE key = ? AND seq <= ? ORDER BY seq DESC LIMIT 1", val.Key, val.Seq)
	}
	fmt.Printf("Time to find all keys (reverse order): %f seconds\n", time.Since(start).Seconds())

	// Query successor data in normal order
	start2 := time.Now()
	for _, val := range successorData {
		session.Query("SELECT next FROM successor WHERE key = ? AND seq <= ? ORDER BY seq ASC LIMIT 1", val.Key, val.Seq)
	}
	fmt.Printf("Time to find all keys (normal order): %f seconds\n", time.Since(start2).Seconds())

	fmt.Println("Program completed")
}

/**
 * Parses successor_data.csv and retrieves 'successorKeyNum' number of keys appearing >= 'successorThreshHoldNum' number of times
 * @returns a map of key string, to the queryInfo (which holds blob key and int sequence)
 */
func getSuccessorKeys(session *gocql.Session) map[string]queryInfo {

	query := "SELECT key, seq FROM successor"
	iter := session.Query(query).Iter()
	defer iter.Close()

	scanner := iter.Scanner()

	keys := make(map[string]queryInfo)
	numAppear := make(map[string]int)

	for scanner.Next() {
		var key []byte
		var seq uint64
		err := scanner.Scan(&key, &seq)
		if err != nil {
			log.Fatal(err)
		}

		mapKey := string(key)
		numAppear[mapKey]++

		if numAppear[mapKey] >= successorThreshHoldNum {
			keys[mapKey] = queryInfo{Key: key, Seq: seq}
		}

		if len(keys) >= successorKeyNum {
			break
		}
	}

	if err := iter.Close(); err != nil {
		log.Fatal("Error closing iterator:", err)
	}

	fmt.Printf("Unique Keys with >= %d occurrences: %d\n", successorThreshHoldNum, len(keys))
	return keys
}
