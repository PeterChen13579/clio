package main

import (
	"fmt"
	"internal/shamap"
	"log"
	"slices"
	"sync"

	"github.com/gocql/gocql"
)

var CURSOR_START = make([]byte, 32)
var CURSOR_END = make([]byte, 32)

var statesMapMutex sync.Mutex

func init() {
	for i := 0; i < 32; i++ {
		CURSOR_START[i] = 0
		CURSOR_END[i] = 0xff
	}
}

func LoadStatesFromCursor(session *gocql.Session, stateMap *shamap.GoSHAMap, ledgerIndex uint64, from []byte, to []byte) {
	log.Printf("Start loading states from cursor %x to %x", from, to)

	startCursor := make([]byte, 32)
	copy(startCursor, from)
	log.Printf("Cursor : %x Begin\n", startCursor)
	for {
		if slices.Compare(CURSOR_START, from) != 0 && slices.Compare(CURSOR_END, from) != 0 {
			var object []byte
			err := session.Query("select object from objects where key = ? and sequence <= ? order by sequence desc limit 1",
				from, ledgerIndex).Scan(&object)
			if err != nil {
				log.Printf("Error: happen when fetch object for %x - %d", from, ledgerIndex)
				log.Fatal(err)
			}
			if len(object) == 0 {
				log.Fatalf("Error: wrong object fetched %x for seq %d", from, ledgerIndex)
			}
			// add in shamap
			statesMapMutex.Lock()
			stateMap.AddStateItem(from, object, uint32(len(object)))
			statesMapMutex.Unlock()
		}
		// find next
		next := make([]byte, 32)
		err := session.Query(`select next from successor where key = ? and seq <= ? order by seq desc limit 1`,
			from, ledgerIndex).Scan(&next)
		if err != nil {
			log.Fatalf("Error when fetch next from successor for %x %v", from, err)
		}
		// over scope
		if slices.Compare(next, to) >= 0 {
			break
		}
		from = next
	}
	log.Printf("Cursor : %x Finished\n", startCursor)
}

func getIndexesFromDiff(session *gocql.Session, ledgerIndex uint64) [][]byte {
	var ret [][]byte
	scanner := session.Query("select key from diff where seq = ?", ledgerIndex).Iter().Scanner()
	for scanner.Next() {
		var key []byte
		err := scanner.Scan(&key)
		if err != nil {
			log.Fatal(err)
		}
		ret = append(ret, key)
	}
	return ret
}

func getLedgerStatesCursor(session *gocql.Session, diff uint32, startIdx uint64) ([][]byte, error) {
	var ret [][]byte

	for i := uint64(0); i < (uint64(diff)); i++ {
		keys := getIndexesFromDiff(session, startIdx-i)
		ret = append(ret, keys...)
	}

	slices.SortFunc(ret, func(a, b []byte) int {
		for i := range a {
			if a[i] != b[i] {
				if a[i] < b[i] {
					return -1
				}
				return 1
			}
		}
		return 0
	})

	ret = slices.CompactFunc(ret, func(a, b []byte) bool {
		return slices.Compare(a, b) == 0
	})

	fmt.Printf("cursors before filter: %x \n", ret)
	// filter out the deleted objects

	for i := 0; i < len(ret); i++ {
		var object []byte
		err := session.Query("select object from objects where key = ? and sequence <= ? order by sequence desc limit 1",
			ret[i], startIdx).Scan(&object)
		if err != nil {
			log.Fatal(err)
		}
		if len(object) == 0 {
			log.Printf("Remove deleted object from cursor list %x\n", ret[i])
			ret = append(ret[:i], ret[i+1:]...)
			i--
		}
	}

	return ret, nil
}

func UpdateStatesFromDiff(session *gocql.Session, statesMap *shamap.GoSHAMap, ledgerIndex uint64) string {
	diffs := getIndexesFromDiff(session, ledgerIndex)
	for _, diff := range diffs {
		var object []byte
		err := session.Query("select object from objects where key = ? and sequence <= ? order by sequence desc limit 1",
			diff, ledgerIndex).Scan(&object)
		if err != nil {
			log.Fatal(err)
		}
		statesMapMutex.Lock()
		if len(object) == 0 { // object removed
			statesMap.DeleteKey(diff)
		} else { // object updated, API may change to UpdateStateItem
			statesMap.DeleteKey(diff)
			statesMap.AddStateItem(diff, object, uint32(len(object)))
		}
		statesMapMutex.Unlock()
	}
	return statesMap.GetHash()
}

func LoadStatesFromCursors(session *gocql.Session, statesMap *shamap.GoSHAMap, ledgerIndex uint64, cursors [][]byte) string {
	first := make([]byte, 32)
	end := make([]byte, 32)

	copy(first, CURSOR_START)
	copy(end, CURSOR_END)

	var wg sync.WaitGroup
	wg.Add(len(cursors))
	for _, cursor := range cursors {
		firstCpy := make([]byte, 32)
		cursorCpy := make([]byte, 32)
		copy(firstCpy, first)
		copy(cursorCpy, cursor)
		go func() {
			LoadStatesFromCursor(session, statesMap, ledgerIndex, firstCpy, cursorCpy)
			wg.Done()
		}()
		first = cursor
	}

	LoadStatesFromCursor(session, statesMap, ledgerIndex, first, end)
	wg.Wait()
	return statesMap.GetHash()
}

func checkingStatesFromLedger(session *gocql.Session, startLedgerIndex uint64, endLedgerIndex uint64, cursorsCount uint32) uint64 {
	ledgerIndex := startLedgerIndex
	var mismatch uint64 = 0
	for ledgerIndex <= endLedgerIndex {
		log.Printf("Checking states for ledger %d\n", ledgerIndex)

		cursor, _ := getLedgerStatesCursor(session, cursorsCount, ledgerIndex)
		statesMap := shamap.MakeSHAMapState()
		ledgerHashFromMap := LoadStatesFromCursors(session, &statesMap, ledgerIndex, cursor)
		statesMap.Free()
		ledgerHashFromHeader, _ := getHashesFromLedgerHeader(session, ledgerIndex)

		if ledgerHashFromHeader != ledgerHashFromMap {
			mismatch++
			log.Printf("Error: State hash mismatch for ledger %d: %s != %s\n", ledgerIndex, ledgerHashFromMap, ledgerHashFromHeader)
		} else {
			log.Printf("State hash for ledger %d is correct: %s\n\n", ledgerIndex, ledgerHashFromHeader)
		}
		ledgerIndex++
	}
	return mismatch
}

func checkingDiff(session *gocql.Session, startLedgerIndex uint64, endLedgerIndex uint64, cursorsCount uint32) uint64 {
	ledgerIndex := startLedgerIndex
	var mismatch uint64 = 0

	//check the state for the first ledger
	log.Printf("Checking states for ledger %d\n", ledgerIndex)
	cursor, _ := getLedgerStatesCursor(session, cursorsCount, ledgerIndex)
	//using cursor to start loading from DB
	statesMap := shamap.MakeSHAMapState()
	ledgerHashFromMap := LoadStatesFromCursors(session, &statesMap, ledgerIndex, cursor)
	ledgerHashFromHeader, _ := getHashesFromLedgerHeader(session, ledgerIndex)

	if ledgerHashFromHeader != ledgerHashFromMap {
		mismatch++
		log.Printf("Error: State hash mismatch for ledger %d: %s != %s\n", ledgerIndex, ledgerHashFromMap, ledgerHashFromHeader)
		return mismatch
	}
	log.Printf("State hash for ledger %d is correct: %s\n\n", ledgerIndex, ledgerHashFromHeader)

	//check the diff for the rest of the ledgers
	for ledgerIndex = startLedgerIndex + 1; ledgerIndex <= endLedgerIndex; ledgerIndex++ {
		log.Printf("Checking diff for ledger %d\n", ledgerIndex)

		ledgerHashFromMap := UpdateStatesFromDiff(session, &statesMap, ledgerIndex)
		ledgerHashFromHeader, _ := getHashesFromLedgerHeader(session, ledgerIndex)

		if ledgerHashFromHeader != ledgerHashFromMap {
			mismatch++
			log.Printf("Error: State hash mismatch for ledger %d: %s != %s\n", ledgerIndex, ledgerHashFromMap, ledgerHashFromHeader)
		} else {
			log.Printf("State hash for ledger %d is correct: %s\n\n", ledgerIndex, ledgerHashFromHeader)
		}
	}

	statesMap.Free()
	return mismatch
}
