# GFS FAQ

**Q: Did <u>having a single master</u> turn out to be a good idea?**

That idea simplified initial deployment but was not so great in the long run. This article (GFS: Evolution on Fast Forward, https://queue.acm.org/detail.cfm?id=1594206) says that as the years went by and GFS use grew, a few things went wrong:

- <u>The number of **files**</u> grew enough that <u>it wasn't reasonable to store all files' metadata in the **RAM** of a single master</u>. 
- <u>The number of **clients**</u> grew enough that <u>a single master didn't have enough **CPU** power to serve them</u>. 
- The fact that switching from a failed master to one of its secondaries required human intervention made recovery slow. 

Apparently Google's replacement for GFS, <u>**Colossus**, splits the master over multiple servers, and has more automated master failure recovery</u>.

**Q: Why is atomic record append <u>at-least-once</u>, rather than exactly once?**

Section 3.1, Step 7, says that if a write fails at one of the secondaries, the client re-tries the write. That will cause the data
to be appended more than once at the non-failed replicas. A different design could <u>detect duplicate client requests despite arbitrary failures</u> (e.g. a primary failure between the original request and the client's retry). You'll implement such a design in Lab 3, at considerable expense in complexity and performance.

**Q: How does an application know what sections of a chunk consist of padding and duplicate records?**

- To detect padding, applications can <u>put a predictable magic number at the start of a valid record</u>, or <u>include a checksum</u> that will likely only be valid if the record is valid. 
- To detect duplicates, applications can <u>include unique IDs in records</u>. Then, if it reads a record that has the same ID as an earlier record, it knows that they are duplicates of each other. 
- GFS provides a library for applications that handles these cases. 
  - This aspect of the GFS design effectively moves complexity from GFS to applications, which is perhaps not ideal.

**Q: How can clients find their data given that atomic record append writes it at an unpredictable offset in the file?**

Append (and GFS in general) is mostly intended for applications that sequentially read entire files. Such applications will scan the file looking for valid records (see the previous question), so they don't need to know the record locations in advance. For example, the file might contain URLs encountered by a set of concurrent web crawlers. The file offset of any given URL doesn't matter much; readers just want to be able to read the entire set of URLs.

**Q: What's a checksum?**

A checksum algorithm <u>takes a sequence of bytes as input and returns a single number that's a function of that sequence</u>. For example, a simple checksum might be the sum of all the bytes in the input. 

GFS stores the checksum of each 64KB "block" in each chunk. 

- When a chunkserver writes a block of data to its disk, it first computes the checksum of the block, and saves the checksum on disk. 
- When a chunkserver reads a block from its disk, it also reads the relevant previously-saved checksum, re-computes a checksum from the data read from disk, and checks that the two checksums match. If the data was corrupted by the disk, the checksums won't match, and the chunkserver will know to return an error. 

Separately, some GFS applications store their own checksums, over application-defined records, inside GFS files, to distinguish between correct records and padding. CRC32 is an example of a checksum algorithm.

**Q: The paper mentions reference counts -- what are they?**

They are part of the implementation of <u>copy-on-write for snapshots</u>. When GFS creates a snapshot, it doesn't copy the chunks, but instead increases the reference counter of each chunk. This makes creating a snapshot inexpensive. 

If a client writes a chunk and the master notices the reference count is greater than one, the master first makes a copy so that the client can update the copy (instead of the chunk that is part of the snapshot). You can view this as <u>delaying the copy until it is absolutely necessary</u>. The hope is that not all chunks will be modified and one can avoid making some copies.

**Q: If an application uses the standard POSIX file APIs, would it need to be modified in order to use GFS?**

Yes, but GFS isn't intended for existing applications. It is designed for newly-written applications, such as MapReduce programs.

**Q: How does GFS determine the location of the nearest replica?**

The paper hints that GFS does this <u>based on the IP addresses of the servers storing the available replicas</u>. In 2003, Google must have assigned IP addresses in such a way that if two IP addresses are close to each other in IP address space, then they are also <u>close to each other in machine-room network topology</u> (perhaps plugged into the same Ethernet switch, or into Ethernet switches that are themselves directly connected).

**Q: What's a lease?**

For GFS, a lease is a period of time for which the master grants a chunkserver the ability to act as the primary for a particular chunk. The master guarantees not to assign a different primary for the duration of the lease, and the primary agrees to stop acting as primary before the lease expires (unless the primary first asks the master to extend the lease). <u>Leases are a way to avoid having the primary have to repeatedly ask the master if it is still primary</u> -- it knows it can act as primary for the next minute (or whatever the lease interval is) without talking to the master again.

**Q: Suppose S1 is the primary for a chunk, and the network between the master and S1 fails. The master will notice and designate some other server as primary, say S2. Since S1 didn't actually fail, are there now two primaries for the same chunk?**

That would be a disaster, since both primaries might apply different updates to the same chunk. Luckily GFS's lease mechanism prevents this scenario:

- The master granted S1 a 60-second lease to be primary. S1 knows to stop being primary before its lease expires. 
- The master won't grant a lease to S2 until after the lease to S1 expires. 
- So S2 won't start acting as primary until after S1 stops.

**Q: 64 megabytes sounds awkwardly large for the chunk size!**

The 64 MB chunk size is <u>the unit of book-keeping in the master</u> and <u>the granularity at which files are sharded over chunkservers</u>. Clients can issue smaller reads and writes -- they are not forced to deal in whole 64 MB chunks. 

The point of using such a big chunk size is to <u>reduce the size of the metadata tables</u> in the master, and to <u>avoid limiting clients that want to do huge transfers</u> to reduce overhead. 

On the other hand, <u>files less than 64 MB in size do not get much parallelism</u>.

**Q: Does Google still use GFS?**

Rumor has it that GFS has been replaced by something called **Colossus**, with the same overall goals, but <u>improvements in master performance and fault-tolerance</u>. 

In addition, many applications within Google have switched to <u>more database-like storage systems</u> such as **BigTable** and **Spanner**. However, much of the GFS design lives on in **HDFS**, the storage system for the Hadoop open-source MapReduce.

https://cloud.google.com/blog/products/storage-data-transfer/a-peek-behind-colossus-googles-file-system

**Q: How acceptable is it that <u>GFS trades correctness for performance and simplicity</u>?**

This a recurring theme in distributed systems. <u>Strong consistency usually requires protocols that are complex and require communication and waiting for replies</u> (as we will see in the next few lectures). 

By exploiting ways that specific application classes can tolerate relaxed consistency, one can design systems that have good performance and sufficient consistency. For example, GFS optimizes for MapReduce applications, which need high read performance for large files and are OK with having holes in files, records showing up several times, and inconsistent reads. 

On the other hand, GFS would not be good for storing account balances at a bank.

**Q: What if the master fails?**

<u>There are replica masters with a full copy of the master state</u>; the paper's design requires some outside entity (a human?) to decide to switch to one of the replicas after a master failure (Section 5.1.3). We will see later how to build replicated services that automatically switch to a backup server if the main server fails, and you'll build such a thing in Lab 2.

**Q: Why 3 replicas?**

Perhaps this was the line of reasoning: two replicas are not enough because, <u>after one fails, there may not be enough time to re-replicate before the remaining replica fails; three makes that scenario much less likely</u>. With 1000s of disks, low-probabilty events like multiple replicas failing in short order occur uncomfortably often. 

Here is a study of disk reliability from that era: https://research.google.com/archive/disk_failures.pdf. You need to factor in <u>the time it takes to make new copies of all the chunks that were stored on a failed disk</u>; and perhaps also the <u>frequency of power, server, network, and software failures</u>. The cost of disks (and associated power, air conditioning, and rent), and the value of the data being protected, are also relevant.

**Q: What is internal fragmentation? Why does lazy allocation help?**

Internal fragmentation is the space wasted when a system uses an allocation unit larger than needed for the requested allocation. If GFS allocated disk space in 64MB units, then a one-byte file would waste almost 64MB of disk. GFS avoids this problem by allocating disk space lazily. <u>Every chunk is a Linux file, and Linux file systems use block sizes of a few tens of kilobytes; so when an application creates a one-byte GFS file, the file's chunk consumes only one Linux disk block, not 64 MB</u>.

**Q: What benefit does GFS obtain from the weakness of its consistency?**

It's easier to think about the additional work GFS would have to do to achieve stronger consistency.

<u>The primary should not let secondaries apply a write unless all the secondaries will be able to do it.</u> This likely requires two rounds of communication -- one to ask all secondaries if they are alive and are able to promise to do the write if asked, and (if all answer yes); a second round to tell the secondaries to commit the write.

If the primary dies, some secondaries may have missed the last few update messages the primary sent. This will cause the remaining secondaries to have slightly differing copies of the data. <u>Before resuming operation, a new primary should ensure that all the secondaries have identical copies.</u>

Since clients re-send requests if they suspect something has gone wrong, primaries would need to filter out operations that have already been executed.

Clients cache chunk locations, and may send reads to a chunkserver that holds a stale version of a chunk. GFS would need a way to guarantee that this cannot succeed.
