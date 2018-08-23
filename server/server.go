/*

This package provides a concurrent server implementation for the protocol.

The architecture is really simple, you assign for each action a handler or several.
When the client send an action, the trame sent is dispatched to the handler with a channel.
This channel allows you to respond back to the client, multiple times if you want.

The inner implementation is made of 2 concepts:
	- the Dispatch routine whose job is to handle the data flows.
	  When you send back a trame to client, it'll route the trame to the correct client via his Data Transfer routine (see below).
	  When a trame is sent from a client, it'll dispatch it to the correct handlers
	- the Data Transfer routine is routine to handle the data incoming from a client and the the data upcoming to a client.
	  A routine is launched for each connection.
	  The Dispatch routine dials with the Data Transfer routines to send the data and receive it.

How to use ?

func main() {
	s := server.New()

	s.AddHandlers(3, func(trame *protocol.Trame, s chan<- *protocol.Trame) {
		fmt.Println(string(trame.Payload))

		s <- []byte("Hello World!")
	})

	s.Listen("localhost:4000")
}

*/

package server

import (
	"bufio"
	"crypto/rand"
	"errors"
	"io"
	"math"
	"net"
	"sync"

	"github.com/yanishoss/dumby/protocol"
)

// Handler is a function that handles the trames of a specific action
type Handler = func(trame *protocol.Trame, s chan<- *protocol.Trame)

// Config handles the Server's configuration
type Config struct {
	MaxConnections uint
}

// Server contains all the elements that allow the architecture to works correctly
type Server struct {
	config      *Config
	connections mapSessionToConnection
	listener    *net.TCPListener
	handlers    mapActionToHandlers
	r           chan *protocol.Trame // r is the channel of the incoming data
	s           chan *protocol.Trame // s is the channel of the upcoming data
	mutex       *sync.RWMutex
}

type mapSessionToConnection = map[protocol.Session]chan *protocol.Trame
type mapActionToHandlers = map[protocol.Action]*[]Handler

func generateSessionID() (protocol.Session, error) {
	sessionID := protocol.Session{}
	_, err := rand.Read(sessionID[:])
	return sessionID, err
}

// New creates a Server
func New(config ...*Config) *Server {
	defaultConfig := &Config{
		MaxConnections: 10000,
	}
	connections := make(mapSessionToConnection)
	listener := new(net.TCPListener)
	handlers := make(mapActionToHandlers)
	r := make(chan *protocol.Trame)
	s := make(chan *protocol.Trame)
	mutex := new(sync.RWMutex)

	if len(config) > 0 {
		defaultConfig = config[0]
	}

	return &Server{
		defaultConfig,
		connections,
		listener,
		handlers,
		r,
		s,
		mutex,
	}
}

// AddHandlers adds the handlers to the Server and map them to the provided action
func (s *Server) AddHandlers(action protocol.Action, handlers ...Handler) {
	s.mutex.Lock()
	var actualHandlers []Handler

	if handlersInMap, exist := s.handlers[action]; !exist {
		actualHandlers = make([]Handler, 0, len(handlers))
	} else {
		actualHandlers = *handlersInMap
	}

	newHandlers := append(actualHandlers, handlers...)

	s.handlers[action] = &newHandlers
	s.mutex.Unlock()
}

// Listen launches the Server
func (s *Server) Listen(address string) error {
	listener, err := net.Listen("tcp", address)

	if err != nil {
		return err
	}

	s.listener = listener.(*net.TCPListener)

	// It is quite useless provided your server runs all the time
	// It is here just for the case you stop the server accidentally
	defer s.listener.Close()

	// Launch the Dispatch routine before accepting connections
	go s.dispatch()

	for {
		conn, err := s.listener.Accept()

		if err != nil {
			continue
		}

		// Launch the Data Transfer routines
		s.handleDataTransfer(conn.(*net.TCPConn))
	}
}

func (s *Server) noticeHandlers(trame *protocol.Trame) {
	s.mutex.RLock()

	action := trame.Action

	if handlers, exist := s.handlers[action]; exist {
		for _, handler := range *handlers {
			go handler(trame, s.s)
		}
	}

	s.mutex.RUnlock()
}

func (s *Server) dispatch() {
	for {
		select {
		case trame := <-s.r:
			go s.noticeHandlers(trame)
		case trame := <-s.s:
			go s.noticeDataRoutine(trame)
		}
	}
}

func (s *Server) noticeDataRoutine(trame *protocol.Trame) {
	// Reach the Data Transfer routine of the session
	s.mutex.RLock()
	if send, exist := s.connections[trame.Session]; exist {
		send <- trame
	}
	s.mutex.RUnlock()
}

func (s *Server) generateNewSession() (protocol.Session, error) {
	sessionID, err := generateSessionID()

	if err != nil {
		return protocol.Session{}, err
	}

	// The number of different are so huge (2^256) that it will probably never enter in an infinite loop
	// But for security purpose:
	trials := 0

	s.mutex.RLock()
	for _, exist := s.connections[sessionID]; exist; {
		if trials > int(math.Pow(2, 256)) {
			return protocol.Session{}, errors.New("All the session IDs are used")
		}

		sessionID, err = generateSessionID()

		if err != nil {
			return sessionID, err
		}

		trials++
	}
	s.mutex.RUnlock()

	return sessionID, nil
}

func (s *Server) initConnection(trame *protocol.Trame, send chan *protocol.Trame) (protocol.Session, error) {
	sessionID, err := s.generateNewSession()

	if err != nil {
		return protocol.Session{}, err
	}

	s.mutex.Lock()
	s.connections[sessionID] = send
	s.mutex.Unlock()

	trame.Session = sessionID

	send <- trame

	return sessionID, nil
}

func (s *Server) kill(session protocol.Session, conn *net.TCPConn) error {
	s.mutex.Lock()

	if _, exist := s.connections[session]; exist {
		// Clean up the garbages
		delete(s.connections, session)
	}

	s.mutex.Unlock()
	return conn.Close()
}

func (s *Server) handleIncomingData(conn *net.TCPConn, onInit func(trame *protocol.Trame) (protocol.Session, error), kill chan bool) {
	r := bufio.NewReader(conn)

	receive := s.r

	isInit := false

	buf := make([]byte, r.Size())

	var sessionID protocol.Session
	for {
		select {
		case isKilled := <-kill:
			if isKilled {
				return
			}
		default:
			size, err := r.Read(buf)

			if err == io.EOF || err == io.ErrClosedPipe || err == io.ErrUnexpectedEOF || size < protocol.HeaderSize {
				kill <- true
				return
			}

			trame := new(protocol.Trame)

			err = protocol.Parse(buf, trame)

			if err != nil {
				kill <- true
				return
			}

			// Skip until the client initializes the connection
			if trame.Action != protocol.ActionInit && !isInit {
				continue
			}

			// Skip because the connection is already initialized
			if trame.Action == protocol.ActionInit && isInit {
				continue
			}

			if trame.Action == protocol.ActionInit && !isInit {
				// Initialize the connection
				isInit = true
				sessionID, err = onInit(trame)

				if err != nil {
					kill <- true
					return
				}
			}

			if isInit && sessionID != trame.Session {
				s.kill(sessionID, conn)
				return
			}

			receive <- trame
		}
	}
}

func (s *Server) handleUpcomingData(conn *net.TCPConn, send <-chan *protocol.Trame, kill chan bool) {
	for {
		select {
		case trame := <-send:
			buf := make([]byte, protocol.HeaderSize+trame.PayloadSize)

			trame.Read(buf)

			_, err := conn.Write(buf)

			if err != nil {
				kill <- true
			}
		case isKilled := <-kill:
			if isKilled {
				return
			}
		}
	}
}

func (s *Server) handleClose(session protocol.Session, conn *net.TCPConn, kill chan bool) {
	for {
		isKilled := <-kill

		if isKilled {
			s.kill(session, conn)
			return
		}
	}
}

func (s *Server) handleDataTransfer(conn *net.TCPConn) {
	s.mutex.RLock()
	if uint(len(s.connections)) <= s.config.MaxConnections {
		s.mutex.RUnlock()
		send := make(chan *protocol.Trame)
		kill := make(chan bool)
		var session protocol.Session

		// Uses for the connection initialization
		onInit := func(trame *protocol.Trame) (protocol.Session, error) {
			s, err := s.initConnection(trame, send)
			session = s
			return s, err
		}

		go s.handleIncomingData(conn, onInit, kill)
		go s.handleUpcomingData(conn, send, kill)
		go s.handleClose(session, conn, kill)
	} else {
		s.mutex.RUnlock()
		conn.Close()
		return
	}
}
