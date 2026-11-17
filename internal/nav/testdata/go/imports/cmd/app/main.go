package main

import (
	"fmt"
	// ^ external fmt
	kv "example.com/app/internal/store"
	// <- resolved internal/store/doc.go
	"example.com/lib"
	"github.com/acme/flags"
)

func main() {
	s := kv.New()
	//   ^ resolved internal/store/doc.go "imports example.com/app/internal/store"
	//      ^ resolved internal/store/store.go:New "imported from example.com/app/internal/store"
	fmt.Println(s.Get("k"))
	// <- external fmt
	//  ^ external fmt
	//            ^ resolved internal/store/store.go:Get "by the type written on s"
	_ = lib.Shout("hi")
	//      ^ resolved lib/lib.go:Shout "imported from example.com/lib"
	flags.Parse()
	//    ^ external github.com/acme/flags
	var st kv.Store
	//        ^ resolved internal/store/store.go:Store
	_ = st
}
