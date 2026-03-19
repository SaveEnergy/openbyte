package client

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

func listServers() {
	configFile, err := loadConfigFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "openbyte client: warning: %v\n", err)
	}

	fmt.Println("Configured Servers:")
	fmt.Println()

	if configFile == nil || len(configFile.Servers) == 0 {
		fmt.Println("  No servers configured.")
		fmt.Println()
		fmt.Println("Add servers to ~/.config/openbyte/config.yaml:")
		fmt.Println()
		fmt.Println("  servers:")
		fmt.Println("    nyc:")
		fmt.Println("      url: https://speedtest-nyc.example.com")
		fmt.Println("      name: \"New York\"")
		fmt.Println("    ams:")
		fmt.Println("      url: https://speedtest-ams.example.com")
		fmt.Println("      name: \"Amsterdam\"")
		fmt.Println()
		return
	}

	fmt.Printf("  %-12s %-20s %s\n", "ALIAS", "NAME", "URL")
	fmt.Printf("  %-12s %-20s %s\n", "-----", "----", "---")
	aliases := make([]string, 0, len(configFile.Servers))
	for alias := range configFile.Servers {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	for _, alias := range aliases {
		server := configFile.Servers[alias]
		defaultMark := ""
		if alias == configFile.DefaultServer {
			defaultMark = " *"
		}
		name := server.Name
		if name == "" {
			name = alias
		}
		fmt.Printf("  %-12s %-20s %s%s\n", alias, name, server.URL, defaultMark)
	}
	fmt.Println()
	fmt.Println("  * = default server")
	fmt.Println()
	fmt.Println("Usage: openbyte client -S <alias> or openbyte client <alias>")
}

type ServerLatency struct {
	Alias   string
	URL     string
	Name    string
	Latency time.Duration
	Error   error
}

func selectFastestServer(configFile *ConfigFile, verbose bool) (*ServerLatency, error) {
	if configFile == nil || len(configFile.Servers) == 0 {
		return nil, fmt.Errorf("no servers configured for auto-selection")
	}
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	allResults := collectServerLatencies(configFile, client)
	fastest := pickFastestServer(allResults)

	if verbose {
		printServerLatencies(allResults, fastest)
	}

	if fastest == nil {
		return nil, fmt.Errorf("all servers unreachable")
	}

	return fastest, nil
}

func collectServerLatencies(configFile *ConfigFile, client *http.Client) []ServerLatency {
	results := make(chan ServerLatency, len(configFile.Servers))
	var wg sync.WaitGroup

	for alias, server := range configFile.Servers {
		wg.Add(1)
		go func(alias string, server ServerConfig) {
			defer wg.Done()
			results <- probeServerLatency(client, alias, server)
		}(alias, server)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	allResults := make([]ServerLatency, 0, len(configFile.Servers))
	for result := range results {
		allResults = append(allResults, result)
	}
	return allResults
}

func probeServerLatency(client *http.Client, alias string, server ServerConfig) ServerLatency {
	result := ServerLatency{
		Alias: alias,
		URL:   server.URL,
		Name:  server.Name,
	}

	healthURL := strings.TrimSuffix(server.URL, "/") + "/health"
	start := time.Now()
	resp, err := client.Get(healthURL)
	result.Latency = time.Since(start)
	if resp != nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	if err != nil {
		result.Error = err
		return result
	}
	if resp.StatusCode != http.StatusOK {
		result.Error = fmt.Errorf("health check failed: %d", resp.StatusCode)
	}
	return result
}

func pickFastestServer(allResults []ServerLatency) *ServerLatency {
	var fastest *ServerLatency
	for _, result := range allResults {
		if result.Error != nil {
			continue
		}
		if fastest == nil || result.Latency < fastest.Latency {
			r := result
			fastest = &r
		}
	}
	return fastest
}

func printServerLatencies(allResults []ServerLatency, fastest *ServerLatency) {
	fmt.Println("Server latencies:")
	for _, r := range allResults {
		status := fmt.Sprintf("%dms", r.Latency.Milliseconds())
		if r.Error != nil {
			status = "error"
		}
		marker := "  "
		if fastest != nil && r.Alias == fastest.Alias {
			marker = "→ "
		}
		name := r.Name
		if name == "" {
			name = r.Alias
		}
		fmt.Printf("%s%-12s %-20s %s\n", marker, r.Alias, name, status)
	}
	fmt.Println()
}
