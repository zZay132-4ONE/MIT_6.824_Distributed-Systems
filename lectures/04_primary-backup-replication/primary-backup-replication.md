# 6.5840 2023 Lecture 4: Primary/Backup Replication

> https://pdos.csail.mit.edu/6.824/notes/l-vm-ft.txt

[TOC]

## Primary/Backup Replication for Fault Tolerance

**Why read this paper?**

- A clean primary/backup design that brings out many issues that come up over and and over this semester
  - state-machine replication
  - output rule
  - fail-over/primary election
- Impressive you can do this at the level of machine instructions
  - you can take any application and replicate it VM FT
  - all designs later in semester involve work by the application designer      

**Goal: <u>high availability</u>**

- even if a machine fails, deliver service
- approach: <u>replication</u>

**What kinds of failures can replication deal with?**

- Replication is good for <u>"fail-stop" failure of a single replica</u>
  - fan stops working, CPU overheats and shuts itself down
  - someone trips over replica's power cord or network cable
  - software notices it is out of disk space and stops
- Replication <u>may not help with bugs or operator error</u>
  - Often not fail-stop
  - May be correlated (i.e. some input causes all replicas to crash)
- How about earthquake or city-wide power failure?
  - Only if replicas are physically separated

**Two main replication approaches:**

1. **<u>State transfer</u>**:
   - Primary executes the service
   - Primary sends <u>state snapshots</u> over network to a <u>storage system</u>
   - On failure:
     1. Find a spare machine (or maybe there's a dedicated backup waiting)
     2. Load software, <u>load saved state</u>, execute
2. **<u>Replicated state machine</u>**:
   - Clients send operations to primary
     - primary sequences and sends to backups
   - <u>All replicas execute all operations</u>
   - If same start state,
     - same operations,
     - same order,
     - deterministic,
     - then same end state.

**State transfer is conceptually simple**

- But state may be large, slow to transfer over network

**Replicated state machine often generates less network traffic**

- Operations are often small compared to state
- But complex to get right

**<u>VM-FT uses replicated state machine</u>, as do Labs 2/3/4**

**Big Questions:**

- What are the state and operations?
- Does primary have to wait for backup?
- How does backup decide to take over?
- Are anomalies visible at cut-over?
- How to bring a replacement backup up to speed and resume replication?

**At what level do we want replicas to be identical?**

- <u>Application state</u>, e.g. a database's tables?
  - <u>GFS works this way</u>
  - Efficient; primary only sends high-level operations to backup
  - Application must understand fault tolerance
- <u>Machine level</u>, e.g. registers and RAM content?
  - might allow us to replicate any existing application without modification!
  - requires forwarding of machine events (interrupts, network packets, &c)
  - requires "machine" modifications to send/recv event stream...

## Case study: VMware FT (2010)

**VMware FT replicates <u>machine-level state</u>**

- Transparent: can run any existing O/S and server software!
- Appears like a single server to clients

**Overview**

- [diagram: app, O/S, VM-FT underneath, disk server, network, clients]
- words:
  - **hypervisor** == **monitor** == **VMM (virtual machine monitor)**
  - **O/S + app** is the "guest" running inside a virtual machine
- <u>two physical machines: primary + backup</u>

**The basic idea:**

- Primary and backup initially <u>start with identical memory and registers</u>
  - Including identical software (O/S and app)
- <u>Most instructions execute identically</u> on primary and backup
  - e.g. an ADD instruction
- So most of the time, no work is required to cause them to remain identical!

**When does the primary have to send information to the backup?**

- Any time something happens that might cause their executions to <u>diverge</u>.
- Anything that's not a deterministic consequence of executing instructions.

**What sources of divergence must FT eliminate?**

- <u>Instructions that aren't functions of state</u>, such as reading current time.
- <u>Inputs from external world</u> -- network packets and disk reads.
  - These appear as DMA'd data plus an interrupt.
- Timing of interrupts.
- But not multi-core races, since uniprocessor only.

**Why would divergence be a disaster?**

- Because state on backup would differ from state on primary, and if primary then failed, clients would see inconsistency.
- Example: the 6.824 homework submission server
  - Enforces midnight deadline for labs, and a hardware timer goes off at midnight.
  - Let's replicate submission server with a *broken* FT:
    1. On primary, my homework packet interrupt arrives just *before* timer goes off.
       - Primary will tell me I get full credit for homework.
    2. On backup, my homework arrives just after, so backup thinks it is late.
       - Primary and backup now have divergent state.
       - For now, no-one notices, since the primary answers all requests.
    3. Then primary fails, backup takes over, and course staff see backup's state, which says I submitted late!
  - So: backup must <u>see same events</u>, <u>in same order</u>, <u>at same points</u> in instruction stream.

**The logging channel**

- Primary sends all events to backup over network
  - "logging channel", carrying log entries
  - interrupts, incoming network packets, data read from shared disk
- FT provides <u>backup's input (interrupts &c) from log entries</u>
- FT suppresses backup's network output
- if either stops being able to talk to the other over the network "goes live" and provides sole service 
- if primary goes live, it stops sending log entries to the backup

**Each log entry: <u>instruction #, type, data</u>.**

**FT's <u>handling of timer interrupts</u>**:

- **Goal**: primary and backup should <u>see interrupt at exactly the same point</u> in the instruction stream
- **Primary**:
  1. FT fields the timer interrupt
  2. FT reads instruction number from CPU
  3. FT <u>sends "timer interrupt at instruction # X" on logging channel</u>
  4. FT delivers interrupt to primary, and resumes it
  5. (relies on CPU support to direct interrupts to FT software)
- **Backup**:
  1. <u>ignores its own timer hardware</u>
  2. FT <u>sees log entry *before* backup gets to instruction # X</u>
  3. FT tells CPU to transfer control to FT at instruction # X
  4. <u>FT mimics a timer interrupt that backup guest sees</u>
  5. (relies on CPU support to jump to FT after the X'th instruction)

**FT's <u>handling of network packet arrival (input)</u>**:

- **Primary**:
  1. FT configures Network Interface Card (NIC) to write packet data into <u>FT's private "bounce buffer"</u>
  2. At some point a packet arrives, NIC does Direct Memory Access (DMA), then interrupts
  3. FT gets the interrupt, reads instruction # from CPU
  4. FT pauses the primary
  5. FT copies the bounce buffer into the primary's memory
  6. <u>FT simulates a NIC interrupt in primary</u>
  7. FT <u>sends the packet data and the instruction # to the backup</u>
- **Backup**:
  1. FT <u>gets data and instruction # from log stream</u>
  2. FT tells CPU to interrupt (to FT) at instruction # X
  3. FT copies the data to guest memory, simulates NIC interrupt in backup

**Why the bounce buffer?**

- We want the data to appear in memory at exactly the same point in execution of the primary and backup.
- So they see the same thing if they read packet memory before interrupt.
- Otherwise they may diverge.

**FT VMM emulates a local disk interface**

- but <u>actual storage is on a network server -- the "shared disk"</u>
- all files/directories are in the shared storage; no local disks
- <u>only primary talks to the shared disk</u>
  - primary forwards blocks it reads to the backup
  - backup's FT ignores backup app's writes, serves reads from primary's data
- <u>shared disk makes creating a new backup much faster</u>
  - don't have to copy primary's disk

**The backup must lag by one log entry**

- Suppose primary gets an interrupt at instruction # X
- If backup has already executed past X, it is too late!
- So <u>backup FT can't execute unless at least one log entry is waiting</u>
  - Then it executes just to the instruction # in that log entry
  - And waits for the next log entry before resuming 

**Example: non-deterministic instructions**

- Some instructions yield different results even if primary/backup have same state (e.g. reading the current time or processor serial #)
- Primary:
  1. FT sets up the CPU to interrupt if primary executes such an instruction
  2. FT executes the instruction and records the result
  3. sends result and instruction # to backup
- Backup:
  1. FT reads log entry, sets up for interrupt at instruction #
  2. FT then supplies value that the primary got, does not execute instruction

**What about output (sending network packets, writing the shared disk)?**

- Primary and backup both execute instructions for output
  - <u>Primary's FT actually does the output</u>
  - Backup's FT discards the output

**Output example: DB server**

- Clients can send "increment" request
  - DB increments stored value, replies with new value
- So:
  1. suppose the server's value starts out at 10
  2. network delivers client request to FT on primary
  3. <u>primary's FT sends on logging channel to backup</u>
  4. FTs deliver request packet to primary and backup
  5. primary executes, sets value to 11, sends "11" reply, <u>FT really sends reply</u>
  6. backup executes, sets value to 11, sends "11" reply, <u>FT discards</u>
  7. the client gets one "11" response, as expected
- But wait:
  1. suppose primary sends reply and then crashes
  2. so client gets the "11" reply AND the logging channel discards the log entry w/ client request
  3. primary is dead, so it won't re-send
  4. backup goes live, but it has value "10" in its memory!
  5. now a client sends another increment request, it will get "11" again, not "12".

**Solution: the Output Rule (Section 2.2)**

- <u>Before primary sends outpu</u>t (e.g. to a client, or shared disk), <u>must wait for backup to acknowledge all previous log entries</u>

**Again, with output rule:**

- Primary:
  1. receives client "increment" request
  2. sends client request on logging channel
  3. about to send "11" reply to client
  4. first waits for backup to acknowledge previous log entry
  5. then sends "11" reply to client
- Suppose the primary crashes at some point in this sequence, if before primary receives acknowledgement from backup:
  - maybe backup didn't see client's request, and didn't increment
  - but also primary won't have replied
- if after primary receives acknowledgement from backup:
  - then client may see "11" reply
  - but backup guaranteed to have received log entry w/ client's request
  - so backup will increment to 11

**The Output Rule is a big deal**

- Occurs in some form in most strongly consistent replication systems
  - Often called "<u>**synchronous replication**</u>" b/c <u>primary must wait</u>
- A serious <u>constraint on performance</u>
- An area for application-specific cleverness
  - Eg. maybe no need for primary to wait before replying to read-only operation
- FT has no application-level knowledge, must be conservative

**Q: What if the primary crashes just after getting acknowledgement from backup, but before the primary emits the output? Does this mean that the output won't ever be generated?**

The backup goes live either before or after the instruction that sends the reply packet to the client.

- If before, it will send the reply packet.
- If after, FT will have discarded the packet. But the backup's TCP will think it sent it, and will expect a TCP ACK packet, and will re-send if it doesn't get the ACK.

**Q: But what if the primary crashed *after* emitting the output? Will the backup emit the output a *second* time?**

It might! 

- OK for TCP, since receivers ignore duplicate sequence numbers. 
- OK for writes to shared disk, since backup will write same data to same block #.

Duplicate output at cut-over is pretty common in replication systems.

-   Clients need to keep enough state to ignore duplicates
-   Or be designed so that duplicates are harmless
-   VM FT gets duplicate detection "for free",
  - TCP state is duplicated on backup by VM FT

**Q: Does FT cope with network partition -- could it suffer from split brain? E.g. if primary and backup both think the other is down. Will they both go live?**

The shared disk server breaks the tie. <u>Disk server supports atomic test-and-set.</u> If primary or backup thinks other is dead, attempts test-and-set. If only one is alive, it will win test-and-set and go live. If both try, one will lose, and halt.

**The shared disk server needs to be reliable!**

- If disk server is down, service is down
- They have in mind an expensive fault-tolerant disk server

Q: Why don't they support multi-core?

<u>Performance</u> (table 1)

- FT/Non-FT: impressive!
  - little slow down
- Logging bandwidth
  - Directly reflects disk read rate + network input rate
  - 18 Mbit/s is the max
- The logging channel traffic numbers seem low to me
  - Applications can read a disk at a few 100 megabits/second
  - So their applications may not be very disk-intensive

**When might FT be attractive?**

- <u>Critical but low-intensity</u> services, e.g. name server.
- Services whose software is not convenient to modify.

**What about <u>replication for high-throughput services</u>?**

- People <u>use application-level replicated state machines</u> for e.g. databases.
  - The state is just the DB, not all of memory+disk.
  - The events are DB commands (put or get), not packets and interrupts.
  - Can have short-cuts for e.g. read-only operations.
- Result: <u>less logging traffic, fewer Output Rule pauses</u>.
- <u>GFS</u> use application-level replication, as do Lab 2 &c

## Summary

- Primary-backup replication
  - VM-FT: clean example
- How to cope with partition without single point of failure?
  - Next lecture
- How to get better performance?
  - Application-level replicated state machines

----

VMware KB (#1013428) talks about multi-CPU support. VM-FT may have switched from a replicated state  machine approach to the state transfer approach, but unclear whether that is true or not.

- http://www.wooditwork.com/2014/08/26/whats-new-vsphere-6-0-fault-tolerance/
- http://www-mount.ece.umn.edu/~jjyi/MoBS/2007/program/01C-Xu.pdf
- http://web.eecs.umich.edu/~nsatish/abstracts/ASPLOS-10-Respec.html

