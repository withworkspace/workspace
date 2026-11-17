package members

func (s *Server) handle() string {
	v := s.store.Get("k")
	//     ^ resolved types.go:store "by the type written on s"
	//           ^ resolved types.go:Get "by the type written on s.store"
	return v + s.Hello() + s.ID
	//           ^ resolved types.go:Hello "by the type written on s"
	//                       ^ resolved types.go:ID "by the type written on s"
}

func run() {
	srv := NewServer("x")
	_ = srv.Start()
	//      ^ resolved types.go:Start "by the type written on srv"
	_ = NewServer("y").Start()
	//                 ^ resolved types.go:Start
	lit := Server{name: "z"}
	//            ^ resolved types.go:name "a member of Server"
	_ = lit.name
	//      ^ resolved types.go:name
	_ = srv.Store().Get("k")
	//              ^ resolved types.go:Get
	var b Base
	_ = b.Hello()
	//    ^ resolved types.go:Hello
	_ = lit.Base
	//      ^ resolved types.go:Base
}
