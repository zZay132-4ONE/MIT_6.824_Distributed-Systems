// Server

package kv_storage

import (
	"log"
	"net"
	"net/rpc"
	"sync"
)

type KV struct {
	mu   sync.Mutex
	data map[string]string
}

// server sets up the RPC server.
func server() {
	// Declare and register an RPC handler (an object with methods)
	kv := &KV{data: map[string]string{}}
	rpcs := rpc.NewServer()
	rpcs.Register(kv)
	// Listen from clients on a specific port and accept TCP connections
	l, e := net.Listen("tcp", ":1234")
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go func() {
		for {
			conn, err := l.Accept()
			if err == nil {
				go rpcs.ServeConn(conn)
			} else {
				break
			}
		}
		l.Close()
	}()
}

// Get returns the corresponding value of the provided key.
func (kv *KV) Get(args *GetArgs, reply *GetReply) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	reply.Value = kv.data[args.Key]
	return nil
}

// Put stores a key/value pair.
func (kv *KV) Put(args *PutArgs, reply *PutReply) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	kv.data[args.Key] = args.Value
	return nil
}
