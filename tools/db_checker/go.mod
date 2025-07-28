module xrplf/clio/db_checker

go 1.21.6

require (
	github.com/alecthomas/kingpin/v2 v2.4.0
	github.com/gocql/gocql v1.6.0
	github.com/stretchr/testify v1.8.2
	internal/shamap v1.0.0
	internal/utils v1.0.0
)

replace internal/shamap => ./internal/shamap

replace internal/utils => ./internal/utils

require (
	github.com/alecthomas/units v0.0.0-20211218093645-b94a6e3cc137 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang/snappy v0.0.3 // indirect
	github.com/hailocab/go-hostpool v0.0.0-20160125115350-e80d13ce29ed // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/xhit/go-str2duration/v2 v2.1.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
