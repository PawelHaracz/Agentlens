// Command a2a-agent is a minimal mock A2A agent server for demo purposes.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

type AgentCard struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Protocol    string   `json:"protocol"`
	Endpoint    string   `json:"endpoint"`
	Skills      []Skill  `json:"skills"`
	Tags        []string `json:"tags"`
}

type Skill struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	InputModes  []string `json:"input_modes"`
	OutputModes []string `json:"output_modes"`
}

func main() {
	port := getEnv("PORT", "9001")
	name := getEnv("AGENT_NAME", "demo-a2a-agent")
	host := getEnv("HOST", fmt.Sprintf("http://localhost:%s", port))

	card := AgentCard{
		Name:        name,
		Description: "A demo A2A agent for AgentLens testing",
		Version:     "1.0.0",
		Protocol:    "a2a",
		Endpoint:    host,
		Tags:        []string{"demo", "a2a"},
		Skills: []Skill{
			{
				Name:        "echo",
				Description: "Echoes back the input message",
				InputModes:  []string{"text"},
				OutputModes: []string{"text"},
			},
			{
				Name:        "summarize",
				Description: "Summarizes the provided text",
				InputModes:  []string{"text"},
				OutputModes: []string{"text"},
			},
		},
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(card)
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"status":"ok"}`)
	})

	addr := ":" + port
	log.Printf("A2A agent %q listening on %s", name, addr)
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
