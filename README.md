# Distributed Database System (README)

This project is a distributed database system written in Go, consisting of a Master Node and multiple Slave Nodes. The system is designed for scalability and reliability, with a clear separation of responsibilities between the master and slave nodes.

## Project Structure

```
.
├── shared/           # Shared components
│   ├── db.go        # Database functions
│   ├── types.go     # Type definitions
│   └── protocol.go  # Communication protocol
├── master/          # Master node
│   ├── main.go      # Entry point
│   ├── server.go    # Master logic
│   └── web/         # Web interface
├── slave/           # Slave nodes
│   ├── main.go      # Entry point
│   ├── client.go    # Master connection logic
│   ├── server.go    # Slave logic
│   └── web/         # Web interface
├── go.mod           # Dependency management
└── go.sum           # Dependency verification
```

## System Architecture Overview

The system follows a Master-Slave architecture to ensure scalability, reliability, and data consistency. The Master node is responsible for all write operations and coordination, while Slave nodes handle read operations and replicate data from the Master. Communication between nodes is handled via TCP and HTTP protocols, and all nodes interact with a shared MySQL database.

**Architecture Diagram:**

```
           +-------------------+
           |    Web Clients    |
           +-------------------+
                    |
                    v
           +-------------------+
           |     Master Node   |
           |  (Write/Control)  |
           +-------------------+
            |        |        |
   TCP/HTTP |        |        | TCP/HTTP
            v        v        v
      +---------+ +---------+ +---------+
      | Slave 1 | | Slave 2 | | Slave N |
      | (Read)  | | (Read)  | | (Read)  |
      +---------+ +---------+ +---------+
            \        |        /
             \       |       /
              +--------------+
              |   MySQL DB   |
              +--------------+
```

- **Master Node:** Handles all write operations, manages slaves, and broadcasts updates.
- **Slave Nodes:** Handle read operations, replicate data from the master, and provide web interfaces for queries.
- **MySQL Database:** Central data store accessed by all nodes.
- **Communication:** Master and slaves communicate over TCP/HTTP; web clients interact via HTTP.

## Main Components

### 1. Shared Components (`shared/`)
- **db.go:** Core MySQL database functions (connect, create, drop, query, replicate).
- **types.go:** Data structures for requests, responses, and table schemas.
- **protocol.go:** Protocol for master-slave communication.

### 2. Master Node (`master/`)
- Handles all write operations (CREATE, DROP, INSERT, UPDATE).
- Manages connected slave nodes and broadcasts updates.
- Provides an HTTP API and web interface for management and monitoring.
- Logs all events and queries.

### 3. Slave Node (`slave/`)
- Provides read-only access to the database.
- Maintains a persistent TCP connection to the master.
- Listens for updates and replication commands from the master.
- Sends periodic heartbeats to the master.
- Offers a web interface for executing SELECT queries and viewing data.

## Features

- **Data Replication:** Ensures data consistency between master and slaves.
- **Separation of Responsibilities:** Master handles writes; slaves handle reads.
- **Web Interface:** User-friendly web UI for both master and slave nodes.
- **Security:** Token-based authentication for all requests.
- **Logging and Monitoring:** Detailed logs for all operations and connections.

## Requirements

- Go 1.16 or newer
- MySQL 5.7 or newer
- Modern web browser

## Installation & Running

1. Install Go and MySQL.
2. Clone the repository.
3. Run `go mod download` to fetch dependencies.
4. Start the master node: `go run master/main.go`
5. Start one or more slave nodes: `go run slave/main.go`

## Usage

1. Access the master web interface at `http://localhost:8083`
2. Access the slave web interface at `http://localhost:8084`
3. Use the web UI to execute queries and manage the database.

## Advanced Usage Examples

### Testing Replication
1. Start the master node: `go run master/main.go`
2. Start one or more slave nodes: `go run slave/main.go`
3. Execute an INSERT or UPDATE query from the master web interface.
4. Verify the new data appears when running a SELECT query from the slave web interface.

### Adding a New Slave Node
1. Ensure the master node is running.
2. Start a new slave node: `go run slave/main.go`
3. Check that the new slave appears in the master's connected slaves list.

## Troubleshooting

- If a slave cannot connect to the master, check the IP address and port.
- Ensure MySQL is running on the correct port.
- Check the log files (`master_log.txt` or `slave_log.txt`) for error messages.
- If replication does not work, ensure all write queries are sent through the master node.

## Contribution

Contributions are welcome! Please follow these steps:
1. Fork the repository
2. Create a branch for your feature
3. Make your changes
4. Submit a Pull Request

## License

This project is licensed under the MIT License.
