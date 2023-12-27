# 6.5840 2023 Lecture 3: GFS

> https://pdos.csail.mit.edu/6.824/notes/l-gfs.txt

[TOC]

## Google File System (GFS)

- Sanjay Ghemawat, Howard Gobioff, and Shun-Tak Leung
- SOSP 2003

**Why are we reading this paper?**

- GFS paper touches on many themes of 6.824
  - parallel performance, fault tolerance, replication, consistency
- good systems paper -- details from apps all the way to network
- successful real-world design

**GFS context**

- Many Google services needed a big fast unified storage system
  - Mapreduce, crawler, indexer, log storage/analysis
- Shared among multiple applications e.g. crawl, index, analyze
- <u>Huge capacity</u>
- <u>Huge performance</u>
- <u>Fault tolerant</u>
- But:
  1. just for internal Google use
  2. <u>aimed at batch big-data performance, not interactive</u>

**GFS overview**

- 100s/1000s of <u>clients</u> (e.g. MapReduce worker machines)
- 100s of <u>chunkservers</u>, each with its own disk
- 1 <u>coordinator</u>

**<u>Capacity</u> story?**

- big files split into <u>64 MB chunks</u>
- <u>each file's chunks striped/sharded over chunkservers</u>
  - so a file can be much larger than any one disk
- each chunk in <u>a Linux file</u>

**<u>Throughput</u> story?**

- clients talk directly to chunkservers to read/write data
- if lots of clients access <u>different chunks</u>, huge parallel throughput
- read or write

**<u>Fault tolerance</u> story?**

- each 64 MB chunk stored (replicated) on <u>three chunkservers</u>
- client writes are sent to all of a chunk's copies
- a read just needs to consult one copy

**What are the <u>steps when client C wants to read a file</u>?**

  1. Client (C) sends filename and offset to coordinator (CO) (if not cached).

  2. CO finds **chunkhandle** for that offset (<u>a filename -> array-of-chunkhandle table</u>).

  3. CO replies with chunkhandle + list of **chunkservers** (<u>a chunkhandle -> list-of-chunkservers table</u>).

  4. C caches chunkhandle + chunkserver list.

  5. C sends request to nearest chunkserver.

     (chunk handle, offset)

  6. Chunkserver reads from **chunkfile** on disk, returns to client.

**Clients only ask coordinator where to find a file's chunks**

- clients cache name -> chunkhandle info
- <u>coordinator does not handle data</u>, so (hopefully) not heavily loaded

**What about writes?**

- <u>Client knows which chunkservers hold replicas that must be updated</u>.
- How should we manage updating of replicas of a chunk?

## Replication Schemes

### Bad Replication Scheme

- (This is *not* what GFS does)
- [diagram: C, S1, S2, S3]
- Client sends update to each replica chunkserver
- Each chunkserver applies the update to its copy

**What can go wrong?**

- *Two* clients write the same data at the same time
  - i.e. "concurrent writes"
  - Chunkservers may see the updates in different orders!
  - Again, the risk is that, later, two clients may read different content

### Primary/Secondary Replication (Primary/Backup)

- <u>For each chunk, designate one server as "primary"</u>.
- Clients send write requests <u>just to the primary</u>.
  - The primary alone manages interactions with secondary servers.
  - (Some designs send reads just to primary, some also to secondaries)
- The primary chooses the order for all client writes.
  - Tells the secondaries -- with sequence numbers -- so all replicas
  - apply writes in the same order, even for concurrent client writes.
- There are still many details to fill in, and we'll see a number of variants in upcoming papers.

**What are the <u>steps when client C wants to write a file at some offset</u>?**

- paper's Figure 2

  1. Client (C) asks Coordinator(CO) about file's chunk @ offset.
  2. CO tells C the primary and secondaries.
  3. C sends data to all (just temporary...), waits for all replies (?).
  4. C asks Primary (P) to write.
  5. P checks that lease hasn't expired.
  6. P writes its own chunk file (a Linux file).
  7. <u>P tells each secondary to write (copy temporary into chunk file)</u>.
    8. <u>P waits for all secondaries to reply, or timeout</u> (secondary can reply "error" e.g. out of disk space).
  9. P tells C "ok" or "error".
  10. C retries from start if error.

## GFS Consistency

1. if primary tells client that a write succeeded and no other client is writing the same part of the file:
   - all readers will see the write.
   - "defined"
2. if successful concurrent writes to the same part of a file and they all succeed:
   - all readers will see the same content,
   - but maybe it will be a mix of the writes.
   - "consistent"
   - E.g. C1 writes "ab", C2 writes "xy", everyone might see "axyb".
3. if primary doesn't tell the client that the write succeeded:
   - different readers may see different content, or none.
   - "inconsistent"

**How can <u>inconsistent content</u> arise?**

- Primary P updated its own state.
- But secondary S1 did not update (failed? slow? network problem?).
- Client C1 reads from P; Client C2 reads from S1.
  - they will see different results!
- Such a departure from ideal behavior is an "<u>anomaly</u>".
- But note that in this case the primary would have returned an error to the writing client.

**How can <u>consistent but undefined</u> arise?**

- Clients break big writes into multiple small writes,
- e.g. at chunk boundaries, and GFS may interleave them if concurrent client writes.

**How can <u>duplicated data</u> arise?**

- Clients re-try record appends.

**Why are these anomalies OK?**

- They only intended to support a certain subset of their own applications.
  - Written with knowledge of GFS's behavior.
- Probably mostly single-writer and Record Append.
- Writers could include checksums and record IDs.
  - Readers could use them to filter out junk and duplicates.
- Later commentary by Google engineers suggests that it might have been better to make GFS more consistent.
  - http://queue.acm.org/detail.cfm?id=1594206

**What might better consistency look like?**

- There are many possible answers.
- Trade-off between <u>easy-to-use for client application programmers</u> and <u>easy-to-implement for storage system designers</u>.
- Maybe try to <u>mimic local disk file behavior</u>.
- Perhaps:
  * <u>atomic writes</u>: either all replicas are updated, or none, even if failures.
  * <u>read sees latest write</u>.
  * <u>all readers see the same content</u> (assuming no writes).
- We'll see more precision later.

## How GFS handles crashes of various entities

**A client crashes while writing?**

- Either it got as far as asking primary to write, or not.

**A secondary crashes just as the primary asks it to write?**

  1. Primary may retry a few times if secondary revives quickly with disk intact, it may execute the primary's request and all is well.

  2. Primary gives up, and returns an error to the client. Client can retry -- but why would the write work the second time around?

  3. Coordinator notices that a chunkserver is down.

     Periodically pings all chunkservers.

     Removes the failed chunkserver from all chunkhandle lists.

     Perhaps re-replicates, to maintain 3 replicas.

     Tells primary the new secondary list.

**<u>Re-replication</u> after a chunkserver failure may take a Long Time.**

- Since a chunkserver failure requires re-replication of all its chunks.
- (80 GB disk, 10 MB/s network -> an hour or two for full copy.)
- So the primary probably re-tries for a while, and the coordinator lets the system operate with a missing chunk replica,
- before declaring the chunkserver permanently dead.
- How long to wait before re-replicating?
  - Too short: wasted copying work if chunkserver comes back to life.
  - Too long: more failures might destroy all copies of data.

**What if a primary crashes?**

- Remove that chunkserver from all chunkhandle lists.
- For each chunk for which it was primary:
  - wait for lease to expire,
  - grant lease to another chunkserver holding that chunk.

**What is a lease?**

- <u>Permission to act as primary</u> for a given time (60 seconds).
- Primary promises to stop acting as primary before lease expires.
- Coordinator promises not to change primaries until after expiration.
- Separate lease per actively-written chunk.

**Why are leases helpful?**

- The coordinator must be able to designate a new primary if the present primary fails.
- But the coordinator cannot distinguish "primary has failed" from "primary is still alive but the network has a problem."
- What if the coordinator designates a new primary while old one is active?
  - two active primaries!
  - C1 writes to P1, C2 reads from P2, doesn't seen C1's write!
  - called "<u>split brain</u>" -- a disaster
- <u>Leases help prevent split brain</u>:
  - Coordinator won't designate new primary until the current one is guaranteed to have stopped acting as primary.

**What if the coordinator crashes?**

  1. Coordinator <u>writes critical state to its disk</u>.

     If it crashes and reboots with disk intact, re-reads state, resumes operations.

  2. Coordinator <u>sends each state update to a "backup coordinator"</u>, which also records it to disk.

     backup coordinator can take over if main coordinator cannot be restarted.

**What information must the coordinator save to disk to recover from crashes?**

- <u>Table mapping filename -> array of chunkhandles.</u>
- <u>Table mapping chunkhandle -> current version #.</u>
- What about the list of chunkservers for each chunk?
  - A rebooted coordinator asks all the chunkservers what they store.
- A rebooted coordinator must also wait one lease time before designating any new primaries.
- Who/what decides the coordinator is dead, and chooses a replacement?
  - Paper does not say.
  - Could the coordinator replicas ping the coordinator, and automatically take over if no response?
- Suppose the coordinator reboots, and polls chunkservers. What if a chunkserver has a chunk, but it wasn't a secondary? i.e. the current primary wasn't keeping it up to date?
  - <u>Coordinator</u> remembers <u>version number per chunk, on disk</u>.
    - Increments each time it designates a new primary for the chunk.
  - <u>Chunkserver</u> also remembers its <u>version number per chunk</u>.
- When chunkserver reports to coordinator, coordinator compares version number, only accepts if current version.
- What if a client has cached a stale (wrong) primary for a chunk?
- What if the reading client has cached a stale server list for a chunk?
- What if the primary crashes before sending append to all secondaries? Could a secondary that *didn't* see the append be chosen as the new primary? Is it a problem that the other secondary *did* see the append?

**What would it take to have no anomalies -- strict consistency?**

- i.e. all clients see the same file content.

- Too hard to give a real answer, but here are some issues:

    * All replicas should complete each write, or none -- "atomic write". 

      Perhaps tentative writes until all promise to complete it?

      Don't expose writes until all have agreed to perform them!

    * Primary should detect duplicate client write requests.

    * If primary crashes, some replicas may be missing the last few ops.
        They must sync up.

    * Clients must be prevented from reading from stale ex-secondaries.

        You'll see solutions in Labs 2 and 3!


**Are there circumstances in which GFS will break its guarantees? e.g. write succeeds, but subsequent readers don't see the data.**

- All coordinator replicas permanently lose state (permanent disk failure).
  - Read will fail.
- All chunkservers holding the chunk permanently lose disk content.
  - Read will fail.
- CPU, RAM, network, or disk yields an incorrect value.
  - checksum catches some cases, but not all
  - Read may say "success" but yield the wrong data!
  - Above errors were "<u>fail-stop</u>", but this is a <u>"byzantine" failure</u>.
- T<u>ime is not properly synchronized, so leases don't work out</u>.
  - So multiple primaries, maybe write goes to one, read to the other.
  - Again, <u>read may yield "success" but wrong data -- **byzantine failure**</u>.

## GFS Performance

- <u>large aggregate throughput for read</u>
  - 94 MB/sec total for 16 clients + 16 chunkservers
    - or 6 MB/second per client
    - is that good?
    - one disk sequential throughput was about 30 MB/s
    - one NIC was about 10 MB/s
  - Close to saturating inter-switch link's 125 MB/sec (1 Gbit/sec)
  - So: <u>multi-client scalability is good</u>
  - Table 3 reports 500 MB/sec for production GFS, which was a lot
- writes to different files lower than possible maximum
  - authors blame their network stack (but no detail)
- concurrent appends to single file
  - limited by the <u>server that stores last chunk</u>
- hard to interpret after 15 years, e.g. how fast were the disks?

**Retrospective interview with GFS engineer:**

- http://queue.acm.org/detail.cfm?id=1594206
- <u>file count</u> was the biggest problem
  - eventual numbers grew to 1000x those in Table 2 !
  - hard to fit in coordinator RAM
  - coordinator scanning of all files/chunks for GC is slow
- 1000s of clients -> too much CPU load on coordinator
- coordinator fail-over initially manual, 10s of minutes, too long.
- applications had to be designed to cope with GFS semantics
  - and limitations.
  - more painful than expected.
- <u>**BigTable** is one answer to many-small-files problem</u> and <u>**Colossus** apparently shards coordinator data over many coordinators</u>

## Summary

- case study of performance, fault-tolerance, consistency
  - specialized for MapReduce applications
- good ideas:
  - global cluster file system as universal infrastructure
  - <u>separation of **naming** (coordinator) from **storage** (chunkserver)</u>
  - **sharding** for <u>parallel throughput</u>
  - huge files/chunks to reduce overheads
  - primary to choose order for concurrent writes
  - **leases** to <u>prevent split-brain</u>
- not so great:
  - single coordinator performance
    - ran out of RAM and CPU
  - chunkservers <u>not very efficient for small files</u>
  - lack of automatic fail-over to coordinator replica
  - maybe consistency was too relaxed
