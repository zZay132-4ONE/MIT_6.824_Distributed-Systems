// Common RPC request/reply definitions

package kv_storage

// PutArgs represents arguments used for calling "Put()"
type PutArgs struct {
	Key   string
	Value string
}

// PutReply represents the reply of "Put()"
type PutReply struct {
}

// GetArgs represents arguments used for calling "Get()"
type GetArgs struct {
	Key string
}

// GetReply represents the reply of "Get()"
type GetReply struct {
	Value string
}
