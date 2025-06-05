// This go script is to fix the invalid ledger data on ledgers 625681, 625753
// This script will go into the database and delete the corrupted transactions on these two ledgers
// More details can be found here:  https://github.com/XRPLF/clio/issues/2137

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gocql/gocql"
)

var (
	hosts    = flag.String("hosts", "", "Your database IP addresses, comma separated. (i.e 192.168.1.1,192.168.1.2,192.168.1.3)")
	username = flag.String("username", "", "Username for accessing DB")
	password = flag.String("password", "", "Password for accessing DB")
	keyspace = flag.String("keyspace", "", "The keyspace of your DB")
)

func main() {
	flag.Parse()

	if *username == "" || *password == "" {
		fmt.Fprintln(os.Stderr, "Error: --username and --password are required")
		os.Exit(1)
	}

	hostArr := strings.Split(*hosts, ",")
	cluster := gocql.NewCluster(hostArr...)

	cluster.Authenticator = gocql.PasswordAuthenticator{
		Username: *username,
		Password: *password,
	}

	if *keyspace != "" {
		cluster.Keyspace = *keyspace
	}

	DeleteCorruptedData(cluster)
}

func DeleteCorruptedData(cluster *gocql.ClusterConfig) {
	session, err := cluster.CreateSession()
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}
	defer session.Close()

	type DeletionTarget struct {
		table     string
		condition string
	}

	targets := []DeletionTarget{
		{"ledger_transactions", "ledger_sequence = 625753"},
		{"ledger_transactions", "ledger_sequence = 625681"},
		{"transactions", "hash = 0xaf173c404c8babcadfedc5f35cb2975dd452232219c87da3a13a12df3601bef3"},
		{"transactions", "hash = 0xada05495e452b6fc9d32835d836b0c790a5aafe89cc070a25e5d35fa89fae286"},
	}

	for _, target := range targets {

		checkQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", target.table, target.condition)
		var count int

		if err := session.Query(checkQuery).Scan(&count); err != nil {
			log.Printf("Failed to check existence for %s: %v", target.condition, err)
			continue
		}

		if count > 0 {
			delQuery := fmt.Sprintf("DELETE FROM %s WHERE %s", target.table, target.condition)

			if err := session.Query(delQuery).Exec(); err != nil {
				log.Printf("Failed to delete from %s where %s: %v", target.table, target.condition, err)
			} else {
				log.Printf("Deleted from %s where %s", target.table, target.condition)
			}

		} else {
			log.Printf("No matching record found in %s where %s", target.table, target.condition)
		}
	}
}
