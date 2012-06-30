package main

import (
	"femebe"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
)

// Automatically chooses between unix sockets and tcp sockets for
// listening
func autoListen(place string) (net.Listener, error) {
	if strings.Contains(place, "/") {
		return net.Listen("unix", place)
	}

	return net.Listen("tcp", place)
}

// Automatically chooses between unix sockets and tcp sockets for
// dialing.
func autoDial(place string) (net.Conn, error) {
	if strings.Contains(place, "/") {
		return net.Dial("unix", place)
	}

	return net.Dial("tcp", place)
}

type handlerFunc func(
	client femebe.MessageStream,
	server femebe.MessageStream,
	errch chan error)

type proxyBehavior struct {
	toFrontend handlerFunc
	toServer   handlerFunc
}

func (pbh *proxyBehavior) start(
	client femebe.MessageStream,
	server femebe.MessageStream) (errch chan error) {

	errch = make(chan error)

	go pbh.toFrontend(client, server, errch)
	go pbh.toServer(client, server, errch)
	return errch
}

var simpleProxy = proxyBehavior{
	toFrontend: func(client femebe.MessageStream,
		server femebe.MessageStream, errch chan error) {
		for {
			msg, err := server.Next()
			if err != nil {
				errch <- err
				return
			}

			err = client.Send(msg)
			if err != nil {
				errch <- err
				return
			}
		}
	},
	toServer: func(client femebe.MessageStream,
		server femebe.MessageStream, errch chan error) {
		for {
			msg, err := client.Next()
			if err != nil {
				errch <- err
				return
			}

			err = server.Send(msg)
			if err != nil {
				errch <- err
				return
			}
		}
	},
}

// Virtual hosting connection handler
func proxyHandler(clientConn net.Conn, rt *RoutingTable) {
	var err error

	// Log disconnections
	defer func() {
		if err != nil && err != io.EOF {
			fmt.Printf("Session exits with error: %v\n", err)
		} else {
			fmt.Printf("Session exits cleanly\n")
		}
	}()

	defer clientConn.Close()

	c := femebe.NewMessageStream("Client", clientConn, clientConn)

	// Handle the very first message -- the startup packet --
	// specially to do switching.
	firstMessage, err := c.Next()
	startupMsg := femebe.ReadStartupMessage(firstMessage)
	dbname := startupMsg.Params["database"]
	serverAddr := rt.Route(dbname)

	// No route found, quickly exit
	if serverAddr == "" {
		fmt.Printf("No route found for database \"%v\"\n", dbname)
		return
	}

	// Route was found, so now start a trivial proxy forwarding
	// traffic.
	serverConn, err := autoDial(serverAddr)
	if err != nil {
		fmt.Printf("Could not connect to server: %v\n", err)
	}

	s := femebe.NewMessageStream("Server", serverConn, serverConn)
	if err = s.Send(firstMessage); err != nil {
		fmt.Printf("Could not relay startup packet: %v\n", err)
	}

	done := simpleProxy.start(c, s)
	err = <-done
}


type QueryValue interface {
	Value() interface{}
}

type QueryStr string

func (str QueryStr) Value() interface{} {
	return str
}

func adminHandler(clientConn net.Conn, rt *RoutingTable) {
	commandCh := make(chan string)

	var simpleProxy = proxyBehavior{
		toFrontend: func(client femebe.MessageStream,
			server femebe.MessageStream, errch chan error) {
			for {
				command := <- commandCh
				
				if command == "dump routing table" {
					data := make([][]QueryValue, 2)
					for key, value := range rt.table {
						data = append(data, []QueryValue{&QueryStr{key}, &QueryStr{value}})
					}
					SendDataSet(client, []string{"key","value"}, data)
				} else {
					fmt.Printf("Ignoring unknown command %v", command)
				}

			}
		},
		toServer: func(client femebe.MessageStream,
			server femebe.MessageStream, errch chan error) {
			for {
				msg, err := client.Next()
				if femebe.IsQuery(msg) {
					q := femebe.ReadQuery(msg)
					commandCh <- q.Query
				}
			}
		},
	}

	var err error

}


func SendDataSet(stream femebe.MessageStream, colnames []string, rows [][]QueryValue) {
	rowLen :=  len(colnames)
	fieldDescs := make([]femebe.FieldDescription, len(colnames))
	for i, name := range colnames {
		fieldDescs[i] = femebe.NewField(name, femebe.STRING)
	}
	rowDesc := femebe.NewRowDescription(fieldDescs)
	stream.Send(rowDesc)
	for _, row := range rows {
		if len(row) != rowLen {
			panic("oh snap!")
		}
		rowData := make([]interface{}, rowLen)
		for i, item := range row {
			rowData[i] = item.Value()
		}
		stream.Send(femebe.NewDataRow(rowData))
	}
	stream.Send(femebe.NewCommandComplete(fmt.Sprintf("SELECT %v", rowLen)))
}

type Acceptor func(ln net.Conn)

func AcceptorLoop(ln net.Listener, a Acceptor, done chan bool) {
	defer func() { done <- true }()

	for {
		conn, err := ln.Accept()

		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		go a(conn)
	}
}

type RoutingTable struct {
	table map[string]string
	l     sync.Mutex
}

func NewRoutingTable() (rt *RoutingTable) {
	return &RoutingTable{table: make(map[string]string)}
}

func (r *RoutingTable) SetRoute(dbname, addr string) {
	// Only necessary because hash tables are allowed to race and
	// subsequently uglifully crash in non-sandboxed Go programs.
	r.l.Lock()
	defer r.l.Unlock()

	r.table[dbname] = addr
}

func (r *RoutingTable) Route(dbname string) (addr string) {
	return r.table[dbname]
}



// Startup and main client acceptance loop
func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: simpleproxy LISTENADDR\n")
		os.Exit(1)
	}

	listen := func(addr string) net.Listener {
		ln, err := autoListen(addr)
		if err != nil {
			fmt.Printf("Could not listen on address: %v\n", err)
			os.Exit(1)
		}
		return ln		
	}
	proxyLn := listen(os.Args[1])
	if proxyLn != nil {
		defer proxyLn.Close()
	}

	adminLn := listen("/tmp/.s.PGSQL.45432")
	if adminLn != nil {
		defer adminLn.Close()
	}

	rt := NewRoutingTable()
	rt.SetRoute("fdr", "/var/run/postgresql/.s.PGSQL.5432")

	done := make(chan bool)
	go AcceptorLoop(proxyLn,
		func(conn net.Conn) {
			proxyHandler(conn, rt)
		},
		done)

	go AcceptorLoop(proxyLn,
		func(conn net.Conn) {
			adminHandler(conn, rt)
		},
		done)

	_ = <-done
	_ = <-done
	fmt.Println("simpleproxy quits successfully")
	return
}
