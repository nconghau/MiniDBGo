package main

import (
	"fmt"
	"log"

	"github.com/your-username/mini-db-go/internal/lsm"
)

func main() {
	db, err := lsm.OpenLSM("data/tiny")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(ColorGreen + "TinyDB CLI (Mongo-like, @Index19)" + ColorReset)
	fmt.Println("Commands:")
	fmt.Println(ColorCyan + " insertOne, findOne, findMany, updateOne, deleteOne, dumpAll, dumpDB, restoreDB, compact, exit" + ColorReset)
	fmt.Println(ColorYellow + "\nExamples:" + ColorReset)
	fmt.Println(" insertOne customers {\"_id\":\"c1\",\"name\":\"Alice\",\"group\":\"vip\",\"rating\":8}")
	fmt.Println(" findOne customers {\"_id\":\"c1\"}")
	fmt.Println(" findMany customers {\"group\":\"vip\"}")
	fmt.Println(" findMany customers {\"rating\":{\"$gt\":5}}")
	fmt.Println(" updateOne customers {\"_id\":\"c1\"} {\"$set\":{\"name\":\"Alice Updated\"}}")
	fmt.Println(" deleteOne customers {\"_id\":\"c1\"}")
	fmt.Println(" dumpAll customers")
	fmt.Println(" dumpDB                      # export all collections")
	fmt.Println(" restoreDB <file.json>       # restore (will overwrite current db file)")
	fmt.Println(" compact                     # compact DB file (reclaims space)")
	fmt.Println(" exit\n")

	RunCLI(db)
}
