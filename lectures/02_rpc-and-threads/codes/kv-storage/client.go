// Client

package kv_storage

import (
	"log"
	"net/rpc"
)

// connect creates a TCP connection to the server for the current client
func connect() *rpc.Client {
	client, err := rpc.Dial("tcp", ":1234")
	if err != nil {
		log.Fatal("dialing:", err)
	}
	return client
}

// get returns the corresponding value of the provided key
func get(key string) string {
	client := connect()
	args := GetArgs{"subject"}
	reply := GetReply{}
	err := client.Call("KV.Get", &args, &reply)
	if err != nil {
		log.Fatal("error:", err)
	}
	closeErr := client.Close()
	if closeErr != nil {
		log.Fatal("error:", closeErr)
		return ""
	}
	return reply.Value
}

// put stores a key/value pair containing the provided k/v on the server
func put(key string, val string) {
	client := connect()
	args := PutArgs{"subject", "6.824"}
	reply := PutReply{}
	err := client.Call("KV.Put", &args, &reply)
	if err != nil {
		log.Fatal("error:", err)
	}
	closeErr := client.Close()
	if closeErr != nil {
		log.Fatal("error:", closeErr)
	}
}
