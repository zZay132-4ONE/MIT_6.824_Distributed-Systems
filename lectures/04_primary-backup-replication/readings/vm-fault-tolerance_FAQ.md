# VMware FT FAQ

> https://pdos.csail.mit.edu/6.824/papers/vm-ft-faq.txt

**Q: The introduction says that it is more difficult to ensure deterministic execution on physical servers than on VMs. Why is this the case?**

Ensuring determinism is easier on a VM <u>because the hypervisor emulates and controls many aspects of the hardware that might differ between primary and backup executions</u>, for example the precise timing of interrupt delivery.

**Q: What is a <u>hypervisor</u>?**

A hypervisor is part of a Virtual Machine system; it's the same as the <u>Virtual Machine Monitor (VMM)</u>. The hypervisor emulates a computer, and a guest operating system (and applications) execute inside the emulated computer. The emulation in which the guest runs is often called the virtual machine. 

In this paper, <u>the primary and backup are guests running inside virtual machines</u>, and <u>FT is part of the hypervisor implementing each virtual machine</u>.

**Q: Both GFS and VMware FT provide fault tolerance. How should we think about when one or the other is better?**

- **FT**:
  - <u>replicates computation</u>
  - can use it to transparently add fault-tolerance to any existing network server
  - provides <u>fairly strict consistency</u> and <u>is transparent to server and client</u>. You might use FT to make an existing mail server fault-tolerant, for example

- **GFS**:
  - in contrast, provides fault-tolerance <u>just for storage</u>
  - because GFS is specialized to a specific simple service (storage), <u>its replication is more efficient than FT</u> For example, GFS does not need to cause interrupts to happen at exactly the same instruction on all replicas
  - GFS is <u>usually only one piece of a larger system to implement complete fault-tolerant services</u>. For example, VMware FT itself relies on a fault-tolerant storage service shared by primary and backup (the Shared Disk in Figure 1), which you could use something like GFS to implement (though at a detailed level GFS wouldn't be quite the right thing for FT)

**Q: How do Section 3.4's <u>bounce buffers</u> help avoid races?**

The problem arises when a network packet or requested disk block arrives at the primary and needs to be copied into the primary's memory. <u>Without FT, the relevant hardware copies the data into memory while software is executing</u>. Guest instructions could read that memory during the DMA; depending on exact timing, the guest might see or not see the DMA'd data (this is the race). It would be bad if the primary and backup both did this, since due to slight timing differences one
might read just after the DMA and the other just before. In that case they would diverge.

<u>FT avoids this problem by not copying into guest memory while the primary or backup is executing</u>. FT first copies the network packet or disk block into a private "bounce buffer" that the primary cannot access. When this first copy completes, the FT hypervisor interrupts the primary so that it is not executing. FT records the point at which it interrupted the primary (as with any interrupt). Then FT copies the bounce buffer into the primary's memory, and after that allows the primary to continue executing. FT sends the data to the backup on the log channel. Similarly, the backup's FT interrupts the backup at the same instruction as the primary was interrupted, copies the data into the backup's memory while the backup is into executing, and then resumes the backup.

The effect is that <u>the network packet or disk block appears at exactly the same time in the primary and backup</u>, so that no matter when they read the memory, both see the same data.

**Q: What is "an atomic `test-and-set` operation on the shared storage"?**

The system uses a network disk server, shared by both primary and backup (the "shared disk" in Figure 1). That network disk server has a "`test-and-set` service". The test-and-set service maintains a flag that is initially set to false. If the primary or backup thinks the other server is dead, and thus that it should take over by itself, it first sends a test-and-set operation to the disk server. The server executes roughly this code:

```go
test-and-set() {
  acquire_lock()
  if flag == true:
    release_lock()
    return false
  else:
    flag = true
    release_lock()
    return true
```

The primary (or backup) only takes over ("goes live") if `test-and-set` returns true.

The higher-level view is that, if the primary and backup lose network contact with each other, we want only one of them to go live. The danger is that, if both are up and the network has failed, both may go live and develop split brain. If only one of the primary or backup can talk to the disk server, then that server alone will go live. But what if both can talk to the disk server? Then the network disk server acts as a tie-breaker; `test-and-set` <u>returns true only to the first call</u>.

**Q: How much performance is lost by following the Output Rule?**

Table 2 provides some insight. By following the output rule, the transmit rate is reduced, but not hugely.

**Q: What if the application calls a random number generator? Won't that yield different results on primary and backup and cause the executions to diverge?**

FT arranges that the primary and backup get the same number from their random number generators. <u>All the sources of randomness are controlled by the hypervisor</u>. For example, the application may use the current time, or a hardware cycle counter, or precise interrupt times as sources of randomness. In all three cases, the hypervisor intercepts the relevant instructions on both primary and backup and ensures they produce the same values.

**Q: How were the creators certain that they captured all possible forms of non-determinism?**

My guess is as follows. The authors work at a company where many people understand VM hypervisors, microprocessors, and internals of guest OSes well, and are aware of many of the pitfalls. For VM-FT specifically, the authors leverage the log and replay support from a previous project (deterministic replay), which must have already dealt with sources of non-determinism. I assume the designers of deterministic replay did extensive testing and gained experience with sources of non-determinism that the authors of VM-FT use.

**Q: What happens if the primary fails just after it sends output to the external world?**

The backup will likely repeat the output after taking over, so that it's generated twice. This duplication is not a problem for network and disk I/O. If the output is a network packet, then the receiving client's TCP software will discard the duplicate automatically. If the output event is a disk I/O, disk I/Os are idempotent (both write the same data to the same location, and there are no intervening I/Os).

**Q: Section 3.4 talks about disk I/Os that are outstanding on the primary when a failure happens; it says "Instead, we re-issue the pending I/Os during the go-live process of the backup VM." Where are the pending I/Os located/stored, and how far back does the re-issuing need to go?**

The paper is talking about disk I/Os for which there is a log entry indicating the I/O was started but no entry indicating completion. These are the I/O operations that must be re-started on the backup. When an I/O completes, the I/O device generates an I/O completion interrupt. So, if the I/O completion interrupt is missing in the log, then the backup restarts the I/O. If there is an I/O completion interrupt in the log, then there is no need to restart the I/O.

**Q: How is the backup FT able to deliver an interrupt at a particular point in the backup instruction stream (i.e. at the same instruction at which the interrupt originally occured on the primary)?**

Many CPUs support a feature (the "performance counters") that lets the FT VMM tell the CPU a number of instructions, and the CPU will interrupt to the FT VMM after that number of instructions.

**Q: How secure is this system?**

The authors <u>assume that the primary and backup hypervisors follow the protocol and are not malicious</u> (e.g., an attacker didn't
compromise the hypervisors). The system cannot handle compromised hypervisors. On the other hand, the hypervisor can probably defend itself against malicious or buggy guest operating systems and applications; FT will dutifully replicate the malice or bugs.

**Q: Is it reasonable to address only the fail-stop failures? What are other type of failures?**

It is reasonable, since many real-world failures are essentially fail-stop, for example many network and power failures. Doing better than this requires coping with computers that appear to be operating correctly but actually compute incorrect results; in the worst case, perhaps the failure is the result of a malicious attacker. This larger class of non-fail-stop failures is often called "Byzantine". There are ways to deal with Byzantine failures, which we'll touch on at the end of the course, but most of 6.824 is about fail-stop failures.

**Q: Given that it has low overhead (Table 1) and is easy to slip under existing software, is FT used a lot?**

I don't know; but I have a feeling it's not used as much as the authors hoped. <u>In the world of big websites built on open-source</u>
<u>software, people mostly use **application-specific** replication schemes</u>. 

- One thing to keep in mind is that FT supports only uniprocessor VM guests, so <u>the server cannot support CPU parallelism</u>. 
- Another point is that the "logging bandwidth" column in Table 1 suggests that the paper's benchmarks are not very I/O intensive, since a single disk or network interface has the potential to generate much more traffic than the values in that column.
- A final thought is that, given that the service is storing its critical state in the separate fault-tolerant shared disk, there's not much fundamental need for an FT-style hot backup; if the server fails, a fresh server can in principle be started up on some other machine and load its state from the shared disk. Databases already know how to do this for crash recovery.

**Q: The paper says FT doesn't cope with multi-processor guests. Why not?**

In general, the results of software running on multiple processors depends on exactly <u>how the instruction streams on the processors were interleaved</u>. For FT to ensure that the backup stays in sync with the primary, it would have to cause the interleaving to be the same on both computers. This turns out to be hard: you can't easily find out what the interleaving is on the primary, and you can't easily control it on either machine.

A more recent VMware product replicates multi-processor guests, probably using a different technique (replicating memory snapshots rather than operations?).

**Q: Suppose server S1 is acting as primary, and S2 as backup. The network link between them stops working. S2 sees that it can no longer talk to S1 and "goes live", taking over as sole server after successfully acquiring the test-and-set lock. Will S1 be aware that S2 has gone live? Or could S1 continue operating, thus producing "split brain"?**

S1 may indeed not realize what is happening, and clients may continue to send requests to S1 for a while. However, the next time S1 wants to produce output (to a client or the shared disk), the Output Rule requires S1 to wait for S2 to acknowledge all outstanding log entries. S2 won't acknowledge; even if S1 and S2 can communicate, S2 knows that since it is no longer the backup, it should not respond to S1's log entries. So S1 will not be able to generate any output after S2 has decided to go live. So the fact that S1 may think it is still the primary does not actually produce split brain.

This pattern, in which <u>a primary is forced to stop if can't get its backups to acknowledge that it is still the primary</u>, arises in many replication systems as part of the story for preventing split brain.
