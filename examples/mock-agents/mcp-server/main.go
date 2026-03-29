// Command mcp-server is a minimal mock MCP server for demo purposes.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

type MCPServerCard struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Protocol    string   `json:"protocol"`
	Endpoint    string   `json:"endpoint"`
	Tools       []Tool   `json:"tools"`
	Tags        []string `json:"tags"`
}

type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func main() {
	port := getEnv("PORT", "9002")
	name := getEnv("AGENT_NAME", "demo-mcp-server")
	host := getEnv("HOST", fmt.Sprintf("http://localhost:%s", port))

	card := MCPServerCard{
		Name:        name,
		Description: "A demo MCP server for AgentLens testing",
		Version:     "1.0.0",
		Protocol:    "mcp",
		Endpoint:    host,
		Tags:        []string{"demo", "mcp"},
		Tools: []Tool{
			{Name: "read_file", Description: "Read a file from the filesystem"},
			{Name: "write_file", Description: "Write content to a file"},
			{Name: "list_directory", Description: "List files in a directory"},
		},
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/.well-known/mcp/server.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(card)
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"status":"ok"}`)
	})

	addr := ":" + port
	log.Printf("MCP server %q listening on %s", name, addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
