package main

import (
	"bufio"
	"distributed-db/shared"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const validToken = "secret-token"

var (
	dbHandler       *shared.DBHandler
	masterIP        string
	connectedSlaves = make(map[string]string)
	slavesMutex     sync.Mutex
	logFile         *os.File
)

func setupLogging() error {
	var err error
	logFile, err = os.OpenFile("master_log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}

	log.SetOutput(logFile)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	return nil
}

func logEvent(eventType, message string, data interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	logEntry := fmt.Sprintf("[%s] [%s] %s", timestamp, eventType, message)
	if data != nil {
		dataJSON, _ := json.Marshal(data)
		logEntry += fmt.Sprintf(" Data: %s", string(dataJSON))
	}
	log.Println(logEntry)
}

func isMasterQuery(query string) bool {
	query = strings.ToUpper(strings.TrimSpace(query))
	return strings.HasPrefix(query, "CREATE") || strings.HasPrefix(query, "DROP")
}

func StartWebServer(db *shared.DBHandler) {
	if err := setupLogging(); err != nil {
		log.Fatalf("Failed to setup logging: %v", err)
	}
	defer logFile.Close()

	logEvent("SYSTEM", "Starting web server", nil)

	mux := http.NewServeMux()

	webDir := "./web"
	if _, err := os.Stat(webDir); os.IsNotExist(err) {
		logEvent("ERROR", "Web directory not found", map[string]string{"path": webDir})
		log.Fatalf("Web directory not found at %s: %v", webDir, err)
	}
	logEvent("SYSTEM", "Found web directory", map[string]string{"path": webDir})

	fs := http.FileServer(http.Dir(webDir))

	loggedFs := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logEvent("HTTP", fmt.Sprintf("Received request: %s %s", r.Method, r.URL.Path), nil)

		filePath := webDir + r.URL.Path
		if r.URL.Path == "/" {
			filePath = webDir + "/index.html"
		}

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			logEvent("ERROR", "File not found", map[string]string{"path": filePath})
		} else {
			logEvent("FILE", "Serving file", map[string]string{"path": filePath})
		}

		fs.ServeHTTP(w, r)
	})

	corsMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}

	mux.Handle("/", loggedFs)

	mux.HandleFunc("/api/query", func(w http.ResponseWriter, r *http.Request) {
		logEvent("API", "Received query request", nil)
		handleQueryRequest(w, r, db)
	})

	mux.HandleFunc("/query", func(w http.ResponseWriter, r *http.Request) {
		handleQueryRequest(w, r, db)
	})

	mux.HandleFunc("/api/database/create", func(w http.ResponseWriter, r *http.Request) {
		logEvent("API", "Received database creation request", nil)
		handleCreateDatabase(w, r, db)
	})
	mux.HandleFunc("/api/table/create", func(w http.ResponseWriter, r *http.Request) {
		logEvent("API", "Received table creation request", nil)
		handleCreateTable(w, r, db)
	})
	mux.HandleFunc("/api/replicate", func(w http.ResponseWriter, r *http.Request) {
		logEvent("API", "Received replication request", nil)
		handleReplication(w, r, db)
	})

	mux.HandleFunc("/connect", handleConnect)

	mux.HandleFunc("/api/slaves", func(w http.ResponseWriter, r *http.Request) {
		slavesMutex.Lock()
		defer slavesMutex.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(connectedSlaves)
	})

	mux.HandleFunc("/api/logs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		file, err := os.Open("master_log.txt")
		if err != nil {
			http.Error(w, "Failed to read logs", http.StatusInternalServerError)
			return
		}
		defer file.Close()

		var logs []map[string]interface{}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "[") {
				parts := strings.SplitN(line, "] ", 3)
				if len(parts) >= 3 {
					timestamp := strings.TrimPrefix(parts[0], "[")
					typeAndMessage := strings.SplitN(parts[1], "] ", 2)
					if len(typeAndMessage) >= 2 {
						logType := strings.TrimPrefix(typeAndMessage[0], "[")
						message := typeAndMessage[1]

						logEntry := map[string]interface{}{
							"timestamp": timestamp,
							"type":      logType,
							"message":   message,
						}

						if strings.Contains(message, "Data:") {
							dataParts := strings.SplitN(message, "Data:", 2)
							if len(dataParts) == 2 {
								logEntry["message"] = strings.TrimSpace(dataParts[0])
								var data interface{}
								if err := json.Unmarshal([]byte(dataParts[1]), &data); err == nil {
									logEntry["data"] = data
								}
							}
						}

						logs = append(logs, logEntry)
					}
				}
			}
		}

		for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
			logs[i], logs[j] = logs[j], logs[i]
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(logs)
	})

	go func() {
		listener, err := net.Listen("tcp", ":8083")
		if err != nil {
			log.Fatalf("Failed to start TCP server: %v", err)
		}
		defer listener.Close()

		log.Printf("TCP server listening on :8083")

		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("Error accepting connection: %v", err)
				continue
			}
			go handleSlave(conn, db, nil)
		}
	}()

	log.Printf("Starting HTTP server on :8082")
	server := &http.Server{
		Addr:    ":8082",
		Handler: corsMiddleware(mux),
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}

func handleQueryRequest(w http.ResponseWriter, r *http.Request, db *shared.DBHandler) {
	if r.Method != http.MethodPost {
		logEvent("ERROR", "Invalid method for query request", map[string]string{"method": r.Method})
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req shared.DBRequest
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logEvent("ERROR", "Failed to read request body", map[string]string{"error": err.Error()})
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	err = json.Unmarshal(body, &req)
	if err != nil {
		logEvent("ERROR", "Failed to parse JSON request", map[string]string{"error": err.Error()})
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	logEvent("QUERY", "Starting query execution", map[string]string{
		"query": req.Query,
		"from":  req.FromSlave,
		"type":  "local",
	})

	req.Token = "secret-token"
	req.FromSlave = "master"
	req.IsSelect = strings.HasPrefix(strings.ToUpper(strings.TrimSpace(req.Query)), "SELECT")

	resp := HandleLocalQuery(req, db)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)

	logEvent("QUERY", "Query completed", map[string]string{
		"status":  resp.Status,
		"message": resp.Message,
	})
}

func cleanupInactiveSlaves() {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for range ticker.C {
			slavesMutex.Lock()
			now := time.Now()
			for ip, lastSeenStr := range connectedSlaves {
				lastSeen, err := time.Parse(time.RFC3339, lastSeenStr)
				if err != nil {
					log.Printf("Error parsing last seen time for slave %s: %v", ip, err)
					delete(connectedSlaves, ip)
					continue
				}
				if now.Sub(lastSeen) > 2*time.Minute {
					log.Printf("Removing inactive slave: %s (last seen: %s)", ip, lastSeenStr)
					delete(connectedSlaves, ip)
				}
			}
			slavesMutex.Unlock()
		}
	}()
}

func handleSlave(conn net.Conn, db *shared.DBHandler, logger *os.File) {
	slaveAddr := conn.RemoteAddr().String()
	slaveIP := strings.Split(slaveAddr, ":")[0]

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
		tcpConn.SetNoDelay(true)
	}

	logEvent("SLAVE", "New slave connection", map[string]string{
		"address": slaveAddr,
		"ip":      slaveIP,
	})

	slavesMutex.Lock()
	connectedSlaves[slaveIP] = time.Now().Format(time.RFC3339)
	slavesMutex.Unlock()

	defer func() {
		conn.Close()
		slavesMutex.Lock()
		delete(connectedSlaves, slaveIP)
		slavesMutex.Unlock()
		logEvent("SLAVE", "Slave disconnected", map[string]string{
			"address": slaveAddr,
			"ip":      slaveIP,
		})
	}()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		logEvent("SLAVE", "Received request from slave", map[string]string{
			"address": slaveAddr,
			"ip":      slaveIP,
			"request": line,
		})

		if strings.HasPrefix(line, "GET") || strings.HasPrefix(line, "POST") || strings.HasPrefix(line, "PUT") || strings.HasPrefix(line, "DELETE") {
			log.Printf("Received HTTP request on TCP port, ignoring: %s", line)
			continue
		}

		var req shared.DBRequest
		err := json.Unmarshal([]byte(line), &req)
		if err != nil {
			log.Printf("Invalid request format: %v\nRequest content: %s", err, line)
			resp := shared.DBResponse{
				Status:  "error",
				Message: fmt.Sprintf("Invalid request format: %v", err),
			}
			respData, _ := json.Marshal(resp)
			conn.Write(append(respData, '\n'))
			continue
		}

		if req.Token != validToken {
			log.Printf("Invalid token from %s", req.FromSlave)
			resp := shared.DBResponse{
				Status:  "error",
				Message: "Invalid token",
			}
			respData, _ := json.Marshal(resp)
			conn.Write(append(respData, '\n'))
			continue
		}

		if req.FromSlave != "master" && req.FromSlave != "" {
			slavesMutex.Lock()
			connectedSlaves[req.FromSlave] = time.Now().Format(time.RFC3339)
			slavesMutex.Unlock()
			logEvent("SLAVE", "Updated slave connection", map[string]string{
				"slave": req.FromSlave,
				"ip":    slaveIP,
				"time":  time.Now().Format(time.RFC3339),
			})
		}

		if isMasterQuery(req.Query) && req.FromSlave != "master" {
			log.Printf("Rejected master-only query from %s", req.FromSlave)
			resp := shared.DBResponse{
				Status:  "error",
				Message: "Only master can create/drop databases/tables",
			}
			respData, _ := json.Marshal(resp)
			conn.Write(append(respData, '\n'))
			continue
		}

		log.Printf("[%s] Executing query: %s", req.FromSlave, req.Query)
		if logger != nil {
			logger.WriteString(fmt.Sprintf("[%s] %s\n", req.FromSlave, req.Query))
		}

		resp := shared.DBResponse{}
		if req.IsSelect {
			rows, err := db.QueryRows(req.Query)
			if err != nil {
				resp.Status = "error"
				resp.Message = err.Error()
			} else {
				cols, _ := rows.Columns()
				resp.Header = cols
				for rows.Next() {
					colsVals := make([]interface{}, len(cols))
					colsPtrs := make([]interface{}, len(cols))
					for i := range colsVals {
						colsPtrs[i] = &colsVals[i]
					}
					rows.Scan(colsPtrs...)

					strRow := make([]interface{}, len(cols))
					for i, val := range colsVals {
						if b, ok := val.([]byte); ok {
							strRow[i] = string(b)
						} else {
							strRow[i] = val
						}
					}
					resp.Rows = append(resp.Rows, strRow)
				}
				resp.Status = "ok"
				resp.Message = "Select executed successfully"
			}
		} else {
			affected, err := db.ExecuteQuery(req.Query)
			if err != nil {
				resp.Status = "error"
				resp.Message = err.Error()
			} else {
				resp.Status = "ok"
				resp.Message = fmt.Sprintf("Query executed successfully. Rows affected: %d", affected)
			}
		}

		respData, err := json.Marshal(resp)
		if err != nil {
			log.Printf("Error marshaling response: %v", err)
			continue
		}
		respData = append(respData, '\n')
		if _, err := conn.Write(respData); err != nil {
			log.Printf("Error sending response: %v", err)
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading from connection: %v", err)
	}
}

func serveStatic(w http.ResponseWriter, r *http.Request) {
	log.Printf("Serving static file: %s", r.URL.Path)
	if r.URL.Path == "/" {
		log.Printf("Serving index.html")
		http.ServeFile(w, r, "../shared/web/index.html")
		return
	}
	log.Printf("Serving file: %s", "../shared/web"+r.URL.Path)
	http.ServeFile(w, r, "../shared/web"+r.URL.Path)
}

func handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req shared.DBRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	role := "slave"
	if req.IP == masterIP || req.IP == "127.0.0.1" {
		role = "master"
	}

	response := shared.DBResponse{
		Status:  "ok",
		Message: "Connected successfully",
		Role:    role,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req shared.DBRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Role == "slave" {
		query := strings.ToLower(req.Query)
		if !strings.HasPrefix(query, "select") {
			response := shared.DBResponse{
				Status:  "error",
				Message: "Slaves can only execute SELECT queries",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	rows, err := dbHandler.QueryRows(req.Query)
	if err != nil {
		response := shared.DBResponse{
			Status:  "error",
			Message: "Error executing query: " + err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		response := shared.DBResponse{
			Status:  "error",
			Message: "Error getting columns: " + err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range columns {
		valuePtrs[i] = &values[i]
	}

	var results [][]interface{}
	for rows.Next() {
		err := rows.Scan(valuePtrs...)
		if err != nil {
			response := shared.DBResponse{
				Status:  "error",
				Message: "Error scanning row: " + err.Error(),
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		row := make([]interface{}, len(columns))
		for i, val := range values {
			if b, ok := val.([]byte); ok {
				row[i] = string(b)
			} else {
				row[i] = val
			}
		}
		results = append(results, row)
	}

	response := shared.DBResponse{
		Status:  "ok",
		Message: "Query executed successfully",
		Header:  columns,
		Rows:    results,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func HandleLocalQuery(req shared.DBRequest, db *shared.DBHandler) shared.DBResponse {
	logEvent("QUERY", "Starting query execution", map[string]string{
		"query": req.Query,
		"from":  req.FromSlave,
		"type":  "local",
	})

	resp := shared.DBResponse{}

	if (strings.HasPrefix(strings.ToUpper(req.Query), "CREATE") || strings.HasPrefix(strings.ToUpper(req.Query), "DROP")) &&
		req.FromSlave != "master" {
		logEvent("ERROR", "Unauthorized query attempt", map[string]string{
			"query":  req.Query,
			"from":   req.FromSlave,
			"reason": "Only master can create/drop databases/tables",
		})
		resp.Status = "error"
		resp.Message = "Only master can create/drop databases/tables"
		return resp
	}

	if req.IsSelect {
		logEvent("QUERY", "Executing SELECT query", map[string]string{
			"query": req.Query,
		})
		rows, err := db.QueryRows(req.Query)
		if err != nil {
			logEvent("ERROR", "SELECT query failed", map[string]string{
				"query": req.Query,
				"error": err.Error(),
			})
			resp.Status = "error"
			resp.Message = err.Error()
		} else {
			cols, _ := rows.Columns()
			resp.Header = cols
			rowCount := 0
			for rows.Next() {
				colsVals := make([]interface{}, len(cols))
				colsPtrs := make([]interface{}, len(cols))
				for i := range colsVals {
					colsPtrs[i] = &colsVals[i]
				}
				rows.Scan(colsPtrs...)

				strRow := make([]interface{}, len(cols))
				for i, val := range colsVals {
					if b, ok := val.([]byte); ok {
						strRow[i] = string(b)
					} else {
						strRow[i] = val
					}
				}
				resp.Rows = append(resp.Rows, strRow)
				rowCount++
			}
			logEvent("QUERY", "SELECT query completed successfully", map[string]string{
				"query":         req.Query,
				"rows_returned": fmt.Sprintf("%d", rowCount),
			})
			resp.Status = "ok"
			resp.Message = "Select executed"
		}
	} else {
		logEvent("QUERY", "Executing non-SELECT query", map[string]string{
			"query": req.Query,
		})
		affected, err := db.ExecuteQuery(req.Query)
		if err != nil {
			logEvent("ERROR", "Query execution failed", map[string]string{
				"query": req.Query,
				"error": err.Error(),
			})
			resp.Status = "error"
			resp.Message = err.Error()
		} else {
			logEvent("QUERY", "Query executed successfully", map[string]string{
				"query":         req.Query,
				"rows_affected": fmt.Sprintf("%d", affected),
			})
			resp.Status = "ok"
			resp.Message = "Query executed successfully"
		}
	}
	return resp
}

func handleCreateDatabase(w http.ResponseWriter, r *http.Request, db *shared.DBHandler) {
	logEvent("DATABASE", "Received database creation request", nil)

	if r.Method != http.MethodPost {
		logEvent("ERROR", "Invalid method for database creation", map[string]string{"method": r.Method})
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DBName string `json:"db_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logEvent("ERROR", "Failed to parse database creation request", map[string]string{"error": err.Error()})
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	logEvent("DATABASE", "Attempting to create database", map[string]string{"db_name": req.DBName})

	if err := db.CreateDatabase(req.DBName); err != nil {
		logEvent("ERROR", "Database creation failed", map[string]string{
			"db_name": req.DBName,
			"error":   err.Error(),
		})
		response := shared.DBResponse{
			Status:  "error",
			Message: fmt.Sprintf("Failed to create database: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	logEvent("DATABASE", "Database created successfully", map[string]string{"db_name": req.DBName})

	response := shared.DBResponse{
		Status:  "ok",
		Message: fmt.Sprintf("Database %s created successfully", req.DBName),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleCreateTable(w http.ResponseWriter, r *http.Request, db *shared.DBHandler) {
	logEvent("TABLE", "Received table creation request", nil)

	if r.Method != http.MethodPost {
		logEvent("ERROR", "Invalid method for table creation", map[string]string{"method": r.Method})
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req shared.CreateTableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logEvent("ERROR", "Failed to parse table creation request", map[string]string{"error": err.Error()})
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	logEvent("TABLE", "Attempting to create table", map[string]string{
		"table_name": req.TableName,
		"db_name":    req.DBName,
	})

	if err := db.CreateTable(&req); err != nil {
		logEvent("ERROR", "Table creation failed", map[string]string{
			"table_name": req.TableName,
			"db_name":    req.DBName,
			"error":      err.Error(),
		})
		response := shared.DBResponse{
			Status:  "error",
			Message: fmt.Sprintf("Failed to create table: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	logEvent("TABLE", "Table created successfully", map[string]string{
		"table_name": req.TableName,
		"db_name":    req.DBName,
	})

	response := shared.DBResponse{
		Status:  "ok",
		Message: fmt.Sprintf("Table %s created successfully in database %s", req.TableName, req.DBName),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleReplication(w http.ResponseWriter, r *http.Request, db *shared.DBHandler) {
	logEvent("REPLICATION", "Received replication request", nil)

	if r.Method != http.MethodPost {
		logEvent("ERROR", "Invalid method for replication", map[string]string{"method": r.Method})
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req shared.ReplicationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logEvent("ERROR", "Failed to parse replication request", map[string]string{"error": err.Error()})
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	logEvent("REPLICATION", "Starting replication", map[string]string{
		"operation": req.Operation,
	})

	if err := db.ReplicateData(&req); err != nil {
		logEvent("ERROR", "Replication failed", map[string]string{
			"operation": req.Operation,
			"error":     err.Error(),
		})
		response := shared.ReplicationResponse{
			Status:  "error",
			Message: fmt.Sprintf("Failed to replicate data: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	logEvent("REPLICATION", "Replication completed successfully", map[string]string{
		"operation": req.Operation,
	})

	response := shared.ReplicationResponse{
		Status:  "ok",
		Message: fmt.Sprintf("Data replicated successfully for operation %s", req.Operation),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func StartServer() error {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Printf("Error getting interface addresses: %v", err)
		return err
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				masterIP = ipnet.IP.String()
				log.Printf("Found local IP: %s", masterIP)
				break
			}
		}
	}

	log.Printf("Initializing database connection...")
	config := shared.NewDBConfig("Kaido440", "5277859MoKaido!", "127.0.0.1", "3306")
	dbHandler, err = shared.NewDBHandler(config)
	if err != nil {
		log.Printf("Error initializing database: %v", err)
		return err
	}
	log.Printf("Database connection initialized successfully")

	log.Printf("Setting up routes...")
	http.HandleFunc("/", serveStatic)
	http.HandleFunc("/connect", handleConnect)
	http.HandleFunc("/query", handleQuery)
	log.Printf("Routes set up successfully")

	cleanupInactiveSlaves()

	log.Printf("Starting server on %s:8082", masterIP)
	return http.ListenAndServe(":8082", nil)
}
