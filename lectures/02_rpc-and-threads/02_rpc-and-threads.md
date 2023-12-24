# 6.5840 2023 Lecture 2: Threads and RPC

[TOC]

> https://pdos.csail.mit.edu/6.824/notes/l-rpc.txt

**Implementing distributed systems**

- Core infrastructures: <u>threads</u> and <u>RPC</u> (in Go)
- Important for the labs

**Why Go?**

- good support for threads
- convenient RPC
- type-safe and memory-safe
- garbage-collected (no use after freeing problems)
- threads + GC is particularly attractive!
- not too complex
- many recent distributed systems implemented in Go
- After the tutorial, use https://golang.org/doc/effective_go.html

<br>

## Threads

- a useful structuring tool, but can be tricky
- Go calls them <u>goroutines</u>; everyone else calls them <u>threads</u>

**Thread = "thread of execution"**

- threads allow one program to do many things at once
- each thread executes serially, just like an ordinary non-threaded program
- the threads <u>share memory</u>
- each thread includes some <u>per-thread state</u>:
  - <u>program counter(PC)</u>, <u>registers</u>, <u>stack</u>, what it's waiting for

**Why threads?**

- **<u>I/O concurrency</u>**
  - Client sends requests to many servers in parallel and waits for replies.
  - Server processes multiple client requests; each request may block.
  - While waiting for the disk to read data for client X, process a request from client Y.
- <u>**Multicore performance**</u>
  - Execute code in parallel on several cores.
- **<u>Convenience</u>**
  - In background, once per second, check whether each worker is still alive.

**Is there an alternative to threads?**

- Yes: write code that explicitly interleaves activities, in a single thread.
  - Usually called <u>"event-driven"</u>.
- Keep a table of state about each activity, e.g. each client request.
- One "event" loop that:
  - checks for new input for each activity (e.g. arrival of reply from server),
  - does the next step for each activity,
  - updates state.
- Event-driven gets you I/O concurrency,
  - and eliminates thread costs (which can be substantial),
  - but <u>doesn't get multi-core speedup</u>,
  - and is painful to program.

**Threading challenges**

- **<u>sharing data safely</u>**
  - what if two threads do `n = n + 1` at the same time? or one thread reads while another increments?
  - this is a "<u>race</u>" -- and is often a bug
    1. -> use <u>locks</u> (Go's <u>sync.Mutex</u>)
    2. -> or <u>avoid sharing mutable data</u>
- **<u>coordination between threads</u>**
  - one thread is producing data, another thread is consuming it
  - how can the consumer wait (and release the CPU)?
  - how can the producer wake up the consumer?
    1. -> use <u>Go channels</u>
    2. -> or <u>sync.Cond</u>
    3. -> or <u>sync.WaitGroup</u>
- **<u>deadlock</u>**
  - cycles via locks and/or communication (e.g. <u>RPC</u> or <u>Go channels</u>)

<br>

## Threading Example - Web Crawler

### Web Crawler - Concept

**What is a web crawler?**

- <u>goal</u>: fetch all web pages, e.g. to feed to an indexer
- you give it a starting web page, it recursively follows all links
- but don't fetch a given page more than once, and don't get stuck in cycles

**Crawler challenges**

- **<u>Exploit I/O concurrency</u>**

  - <u>Network latency</u> is more limiting than network capacity
  - Fetch many pages in parallel
    - To increase URLs fetched per second

  1. => Use <u>threads</u> for concurrency

- **<u>Fetch each URL only *once*</u>**

  - avoid wasting network bandwidth
  - be nice to remote servers

  1. => Need to remember which URLs visited (record states)

- **<u>Know when finished</u>**

### Web Crawler - Three styles of Solution

> <u>*crawler.go*</u> on schedule page

#### Serial crawler

- performs <u>depth-first exploration</u> via <u>recursive Serial calls</u>
- the "fetched" map avoids repeats, breaks cycles
  - a single map, passed by reference, caller sees callee's updates
- but: <u>fetches only one page at a time</u> -- slow
  - can we just <u>put a "go" in front of the Serial() call</u>?
  - let's try it... what happened?

#### ConcurrentMutex crawler

- Creates a thread for each page fetch
  - Many concurrent fetches, higher fetch rate
- The <u>"go func" creates a goroutine</u> and starts it running
  - func... is an "anonymous function"
- The threads <u>share the "fetched" map</u>
  - So only one thread will fetch any given page
- Why the <u>Mutex (`Lock()` and `Unlock()`) in testAndSet</u>?
  1. One reason:
     - Two threads make simultaneous calls to `ConcurrentMutex()` with same URL
       - Due to two different pages containing link to same URL
     - `T1` reads fetched[url], `T2` reads fetched[url]
     - Both see that url hasn't been fetched (`fetched[url] = false`) => Both fetch, which is wrong
     - <u>The mutex causes one to wait while the other does both check and set</u>
       - So only one thread sees `fetched[url]==false`
     - We say "the lock protects the data"
       - But not Go does not enforce any relationship between locks and data!
     - The code between lock/unlock is often called a "<u>critical section</u>“
  2. Another reason:
     - Internally, map is a complex data structure (tree? expandable hash?)
     - Concurrent update/update may wreck internal invariants
     - Concurrent update/read may crash the read
  3. What if I comment out `Lock()` / `Unlock()`?
  4. `go run crawler.go`
     - Why does it work?
  5. `go run -race crawler.go`
     - Detects races even when output is correct!
  6. What if i forget to `Unlock()`?  => deadlock

- How does the ConcurrentMutex crawler decide it is done?
  - <u>`sync.WaitGroup`</u>
  - <u>`Wait()` waits for all `Add()`'s to be balanced by `Done()`'s</u>
    - i.e. waits for all child threads to finish
    - [diagram: tree of goroutines, overlaid on cyclic URL graph]
    - there's a WaitGroup per node in the tree
- How many concurrent threads might this crawler create?

#### ConcurrentChannel crawler

- **<u>Go channel</u>**
  - a channel is an object
    - `ch := make(chan int)`
  - <u>a channel lets one thread send an object to another thread</u>
  - `ch <- x`
    - the sender waits until some goroutine receives
  - `y := <- ch`
    - `for y := range ch`
    - a receiver waits until some goroutine sends
  - channels both <u>communicate</u> and <u>synchronize</u>
  - several threads can send and receive on a channel
  - <u>channels are cheap</u>
  - remember: <u>sender blocks until the receiver receives</u>!
    - "synchronous"
    - watch out for deadlock
- <u>**ConcurrentChannel coordinator()**</u>
  - `coordinator()` <u>creates a worker goroutine to fetch each page</u>
  - `worker()` sends slice of page's URLs on a channel
  - multiple workers send on the single channel
  - `coordinator()` reads URL slices from the channel
- At what line does the coordinator wait?
  - Does the coordinator use CPU time while it waits?
- Note: there is no recursion here; instead there's a work list.
- Note: <u>no need to lock the fetched map, because it isn't shared</u>!
- How does the coordinator know it is done?
  - Keeps count of workers in `n`.
  - Each worker sends exactly one item on channel.

**Why is it safe for multiple threads use the same channel?**

**Worker thread writes url slice, coordinator reads it, is that a race?**

  * worker only writes slice *before* sending
  * coordinator only reads slice *after* receiving
  * So they can't use the slice at the same time.

**Why does `ConcurrentChannel()` create a goroutine just for `ch <- ...`?**

- Let's get rid of the goroutine...

**When to use <u>sharing and locks</u>, versus <u>channels</u>?**

- Most problems can be solved in either style
- What makes the most sense depends on how the programmer thinks
  - **<u>state</u>** -- sharing and locks
  - **<u>communication</u>** -- channels
- For the 6.824 labs, I recommend:
  - <u>state</u>: sharing+locks 
  - <u>waiting / notification</u>: `sync.Cond` or channels or `time.Sleep()`

<br>

## Remote Procedure Call (RPC)

-   a key piece of distributed system machinery; all the labs use RPC
-   <u>goal</u>: easy-to-program client/server communication
-   hide details of network protocols
-   convert data (strings, arrays, maps, &c) to "wire format"
-   portability / interoperability

**RPC message diagram:**

```
 Client             Server
    request--->
           <---response
```

**Software structure:**

```
client app        handler fns
 stub fns         dispatcher
 RPC lib           RPC lib
   net  ------------ net
```

<br>

## Go RPC Example - Toy K/V Storage Server

> <u>`kv.go`</u> on the schedule page

- <u>A toy key/value storage server</u> -- `Put(key,value)`, `Get(key)->value`
- Uses <u>Go's RPC library</u>

1. Common:
   - Declare <u>`Args` and `Reply` struct</u> for each server handler.
2. Client:
   - `connect()`'s `Dial()` <u>creates a TCP connection to the server</u>
   - `get()` and `put()` are <u>client "stubs"</u>
   - `Call()` <u>asks the RPC library to perform the call</u>
     - you: specify server function name, arguments, place to put reply
     - library: marshalls args, sends request, waits, unmarshalls reply
     - return value from `Call()`: indicates whether it got a reply
     - `reply.Err`: indicating service-level failure
3. Server:
   - Go requires server to <u>declare an object with methods</u> as <u>**RPC handlers**</u>
   - Server then <u>registers that object</u> with the RPC library
   - Server <u>accepts TCP connections</u>, <u>gives them to RPC library</u>
   - The RPC library:
     1. reads each request
     2. creates a new goroutine for this request
     3. unmarshalls request
     4. looks up the named object (in table create by `Register()`)
     5. calls the object's named method (**<u>dispatch</u>**)
     6. marshalls reply
     7. writes reply on TCP connection
   - The server's `Get()` and `Put()` handlers:
     - <u>Must lock</u>, since RPC library creates a new goroutine for each request
     - read args; modify reply
     - Note: <u>state-oriented</u> implementation

**A few details:**

- **<u>Binding</u>**: how does client know what server computer to talk to?
  - For Go's RPC, <u>server name/port</u> is an argument to `Dial()`
  - Big systems have some kind of name or configuration server
- **<u>Marshalling</u>**: format data into packets
  - ​    Go's RPC library can pass strings, arrays, objects, maps, &c
  - ​    Go passes pointers by copying the pointed-to data
  - ​    Cannot pass channels or functions
  - ​    <u>Marshals only exported field</u> (i.e., ones w CAPITAL letter) 

**RPC problem: what to do about failures?**

- e.g. lost packet, broken network, slow server, crashed server

**What does a failure look like to the client RPC library?**

- Client never sees a response from the server
- Client does *not* know if the server saw the request!
  - [diagram of losses at various points]
    1. Maybe server never saw the request
    2. Maybe server executed, crashed just before sending reply
    3. Maybe server executed, but network died just before delivering reply

<br>

### Simplest failure-handling scheme: "best-effort RPC"

- `Call()` waits for response for a while
- If none arrives, re-send the request
- Do this a few times
- Then give up and return an error

**Q: is "best effort" easy for applications to cope with?**

A particularly bad situation:

- client executes and both succeeds:
  1. `Put("k", 10);`
  2. `Put("k", 20);`
- what will `Get("k")` yield?
- [diagram, timeout, re-send, original arrives late]

**Q: is "best effort" ever OK?**

- read-only operations
- operations that do nothing if repeated
- e.g. DB checks if record has already been inserted

<br>

### Better RPC behavior: "at-most-once RPC"

<u>**idea: client re-sends if no answer**</u>

- server RPC code detects duplicate requests,
- <u>returns previous reply instead of re-running handler</u>

**Q: how to detect a duplicate request?**

- client includes <u>unique ID (XID)</u> with each request

- uses same XID for re-send

-   server:

  ```go
  if seen[xid]:
      r = old[xid]
  else
      r = handler()
      old[xid] = r
      seen[xid] = true
  ```

**some at-most-once complexities**

- this will come up in lab 3
- what if two clients use the same XID?
  - big random number?
- how to avoid a huge seen[xid] table?
  - idea: <u>each client has a unique ID</u> (perhaps a big random number)
    - per-client RPC sequence numbers
    - client includes "seen all replies <= X" with every RPC
    - much like TCP sequence #s and acks
  - then server can keep `O(# clients)` states, rather than `O(# XIDs)`
-   server must eventually discard info about old RPCs or old clients
  - when is discard safe?
-   how to handle duplicate requests while original is still executing?
  - server doesn't know reply yet
  - idea: <u>"pending" flag per executing RPC</u>; wait or ignore

**What if an at-most-once server crashes and re-starts?**

- if at-most-once duplicate info in memory, server will forget
  - and accept duplicate requests after re-start
- maybe it should write the duplicate info to disk
- maybe replica server should also replicate duplicate info

**Go RPC is a simple form of "<u>at-most-once</u>"**

- open TCP connection
- write request to TCP connection
- <u>Go RPC never re-sends a request</u>
  - So server won't see duplicate requests
- <u>Go RPC code returns an error if it doesn't get a reply</u>
  - perhaps after a timeout (from TCP)
  - perhaps server didn't see request
  - perhaps server processed request but server/net failed before reply came back

**What about "<u>exactly once</u>"?**

- <u>unbounded retries</u>, plus <u>duplicate detection</u>, plus <u>fault-tolerant service</u>
- Lab 3
