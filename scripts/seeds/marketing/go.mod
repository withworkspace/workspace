// Keeps the ws repo's own `go vet ./...` and `go test ./...` out of this
// folder: the Go under it is the seed's repos' (repos/api has a go.mod of
// its own), and what crew and teammates change in them.
module example.com/marketing-seed

go 1.22
