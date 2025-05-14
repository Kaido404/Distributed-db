package main

import (
	"bufio"
	"distributed-db/shared"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

var dbHandler *shared.DBHandler

func corsMiddleware(next http.Handler) http.Handler {
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

func main() {
	logFile, err := os.OpenFile("slave_log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	dbHandler, err = shared.NewDBHandler(&shared.DBConfig{
		Host:     "127.0.0.1",
		Port:     "3307",
		Username: "Kaido440",
		Password: "5277859MoKaido!",
	})
	if err != nil {
		log.Fatalf("Failed to initialize database handler: %v", err)
	}

	log.Println("Connecting to master server...")
	if err := establishMasterConnection(); err != nil {
		log.Printf("Warning: Failed to connect to master server: %v", err)
		log.Println("Will retry connection when sending queries")
	} else {
		log.Println("Successfully connected to master server")
	}

	go func() {
		log.Printf("Starting Slave GUI server...")

		webDir := "./web"
		if _, err := os.Stat(webDir); os.IsNotExist(err) {
			log.Fatalf("Web directory not found at %s: %v", webDir, err)
		}
		log.Printf("Found web directory at: %s", webDir)

		mux := http.NewServeMux()
		fs := http.FileServer(http.Dir(webDir))

		loggedFs := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("Received request: %s %s", r.Method, r.URL.Path)
			log.Printf("Serving from directory: %s", webDir)

			filePath := webDir + r.URL.Path
			if r.URL.Path == "/" {
				filePath = webDir + "/index.html"
			}

			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				log.Printf("File not found: %s", filePath)
			} else {
				log.Printf("File exists: %s", filePath)
			}

			fs.ServeHTTP(w, r)
		})

		mux.Handle("/", loggedFs)
		mux.HandleFunc("/api/query", handleQueryRequest)
		mux.HandleFunc("/connect", handleConnect)
		mux.HandleFunc("/api/replicate", handleReplicationRequest)

		log.Printf("Slave GUI running at http://localhost:8084/")

		for i := 0; i < 3; i++ {
			log.Printf("Attempt %d: Starting server on port 8084...", i+1)
			server := &http.Server{
				Addr:         ":8084",
				Handler:      corsMiddleware(mux),
				ReadTimeout:  10 * time.Second,
				WriteTimeout: 10 * time.Second,
			}

			if err := server.ListenAndServe(); err != nil {
				log.Printf("Attempt %d: Failed to start web server: %v", i+1, err)
				time.Sleep(time.Second * 2)
			} else {
				log.Printf("Server started successfully on port 8084")
				break
			}
		}
	}()

	log.Println("Slave started. Type your SQL query (or 'exit' to quit):")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		query := scanner.Text()
		if query == "exit" {
			log.Println("Exiting...")
			break
		}

		if query == "" {
			continue
		}

		response, err := sendQueryToMaster(query)
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}

		if response.Status == "error" {
			log.Printf("Error: %s", response.Message)
		} else {
			log.Printf("Success: %s", response.Message)
			if len(response.Rows) > 0 {
				for _, col := range response.Header {
					fmt.Printf("%-20s", col)
				}
				fmt.Println()
				for range response.Header {
					fmt.Printf("%-20s", "--------------------")
				}
				fmt.Println()
				for _, row := range response.Rows {
					for _, val := range row {
						fmt.Printf("%-20v", val)
					}
					fmt.Println()
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading input: %v", err)
	}
}

func handleConnect(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received connection request from %s", r.RemoteAddr)

	if r.Method != http.MethodPost {
		log.Printf("Invalid method: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]interface{}{
		"status":  "ok",
		"message": "Connected successfully",
		"role":    "slave",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Printf("Sent connection response to %s", r.RemoteAddr)
}

func handleQueryRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req shared.DBRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	query := strings.ToUpper(strings.TrimSpace(req.Query))
	if strings.HasPrefix(query, "CREATE") || strings.HasPrefix(query, "DROP") {
		http.Error(w, "CREATE and DROP operations are only allowed on the master server", http.StatusForbidden)
		return
	}

	response, err := sendQueryToMaster(req.Query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func handleReplicationRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req shared.ReplicationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := dbHandler.ReplicateData(&req); err != nil {
		http.Error(w, fmt.Sprintf("Replication failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(shared.ReplicationResponse{
		Status:  "ok",
		Message: "Data replicated successfully",
	})
}
