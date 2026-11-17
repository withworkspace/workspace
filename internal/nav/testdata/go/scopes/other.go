package scopes

func helper(n int) int {
	return n * Limit
	//         ^ resolved main.go:Limit "in this package"
}
