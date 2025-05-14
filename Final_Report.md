# Distributed Database System: Final Report

## 1. System Architecture Overview

This project implements a distributed database system in Go, consisting of a Master Node and multiple Slave Nodes. The architecture follows a master-slave pattern, where the master handles all write operations and broadcasts updates to the slaves, which provide read-only access and listen for updates from the master.

```
+-------------------+
|    Master Node    |
|-------------------|
| - DB Write Access |
| - Broadcast to    |
|   Slaves          |
+-------------------+
         |
         v
+-------------------+    +-------------------+
|    Slave Node     |    |    Slave Node     |
|-------------------|    |-------------------|
| - Read-only DB    |    | - Read-only DB    |
| - Listen to MQ    |    | - Listen to MQ    |
+-------------------+    +-------------------+
```

## 2. Component Descriptions

### Master Node
- Handles all write operations (CREATE, DROP, INSERT, UPDATE).
- Maintains a list of connected slave nodes.
- Broadcasts data changes to all slaves to ensure consistency.
- Provides an HTTP API and a web interface for management and monitoring.
- Logs all events and queries for auditing and debugging.

### Slave Node
- Provides read-only access to the database.
- Maintains a persistent TCP connection to the master node.
- Listens for data updates and replication commands from the master.
- Sends periodic heartbeats to the master to indicate liveness.
- Exposes a web interface for executing SELECT queries and viewing data.

### Shared Components
- Common data structures and protocol definitions for communication.
- Database handler utilities for MySQL operations.

## 3. Design Choices

- **Language:** Go was chosen for its concurrency support, performance, and strong networking libraries.
- **Database:** MySQL is used for its reliability and familiarity.
- **Communication:** TCP sockets are used for persistent master-slave communication; HTTP is used for web interfaces and APIs.
- **Security:** Token-based authentication is implemented for all requests.
- **Scalability:** The architecture allows adding more slave nodes easily for horizontal scaling of read operations.
- **Logging:** Both master and slave nodes maintain detailed logs for monitoring and troubleshooting.

## 4. Challenges and Solutions

- **Consistency:** Ensuring all slaves receive and apply updates from the master. Solution: The master broadcasts all write operations to connected slaves, and slaves acknowledge receipt.
- **Fault Tolerance:** Detecting and handling disconnected or failed slaves. Solution: Heartbeat mechanism and periodic cleanup of inactive slaves.
- **Concurrency:** Managing concurrent connections and updates. Solution: Mutexes are used to protect shared state (e.g., the list of connected slaves).
- **Security:** Preventing unauthorized access. Solution: All requests require a valid token; only the master can perform write operations.
- **User Experience:** Providing a simple web interface for both master and slave nodes to facilitate management and query execution.

## 5. How to Run and Test

1. Install Go and MySQL.
2. Clone the repository and run `go mod download` to fetch dependencies.
3. Start the master node: `go run master/main.go`
4. Start one or more slave nodes: `go run slave/main.go`
5. Access the master web interface at `http://localhost:8083` and the slave interface at `http://localhost:8084`.
6. Test replication by performing write operations on the master and verifying data consistency on the slaves.

## 6. Conclusion

This project demonstrates a simple, extensible distributed database system with clear separation of responsibilities, robust communication, and a user-friendly interface. The design can be extended to support more advanced features such as automatic failover, sharding, or stronger consistency guarantees as needed. 