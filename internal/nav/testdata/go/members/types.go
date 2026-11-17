package members

// Base is embedded.
type Base struct{ ID string }

// Hello is Base's, and so Server's.
func (b Base) Hello() string { return "hi " + b.ID }

// Store gets things.
type Store interface {
	Get(key string) string
}

// Server serves.
type Server struct {
	Base
	store Store
	name  string
}

// NewServer makes one.
func NewServer(name string) *Server { return &Server{name: name} }

// Start starts it.
func (s *Server) Start() error { return nil }

// Store is the server's store.
func (s *Server) Store() Store { return s.store }
