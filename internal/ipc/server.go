package ipc

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
)

const SocketPath = "/tmp/crewalk.sock"

type Server struct {
	events   chan Event
	answers  chan Event
	listener net.Listener
}

func NewServer() *Server {
	return &Server{
		events:  make(chan Event, 100),
		answers: make(chan Event, 10),
	}
}

func (s *Server) Events() <-chan Event {
	return s.events
}

func (s *Server) Start() error {
	os.Remove(SocketPath)
	ln, err := net.Listen("unix", SocketPath)
	if err != nil {
		return err
	}
	s.listener = ln
	go s.accept()
	return nil
}

func (s *Server) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}
	os.Remove(SocketPath)
}

func (s *Server) accept() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}
		s.events <- event
	}
}
