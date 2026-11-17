package scopes

// Limit is how many.
const Limit = 3

func count(items []string) int {
	n := 0
	for i, item := range items {
		if item == "" {
			// ^ resolved :item "local"
			continue
		}
		n := n + i
		//   ^ resolved :n "local"
		//       ^ resolved :i "local"
		_ = n
	}
	if n > Limit {
		//     ^ resolved :Limit "in this file"
		return helper(n)
		//     ^ resolved other.go:helper "in this package"
	}
	return len(items)
	//     ^ builtin
	//         ^ resolved :items "local"
}

func shadow() string {
	Limit := "four"
	return Limit
	//     ^ resolved :Limit "local"
}

func later() int {
	x := 1
	{
		y := x
		//   ^ resolved :x "local"
		x := 2
		_ = y + x
		//      ^ resolved :x "local"
	}
	return nothing
	//     ^ unresolved
}

func receive(ch chan int) int {
	select {
	case v, ok := <-ch:
		if ok {
			// ^ resolved :ok "local"
			return v
			//     ^ resolved :v "local"
		}
	}
	return 0
}
