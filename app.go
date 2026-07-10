package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"twNetMap/backend/ai"
	"twNetMap/backend/datastore"
	"twNetMap/backend/scanner"
)

// App struct
type App struct {
	ctx               context.Context
	db                *datastore.DB
	mu                sync.Mutex
	latestScanResults []*scanner.ScanResult
	scanCancel        context.CancelFunc
	scanning          bool
	aiRunning         bool
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// and we initialize the database.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	configDir, err := os.UserConfigDir()
	if err != nil {
		log.Printf("failed to get user config dir: %v", err)
		return
	}
	appDir := filepath.Join(configDir, "twNetMap")
	if err := os.MkdirAll(appDir, 0750); err != nil {
		log.Printf("failed to create app data dir %s: %v", appDir, err)
		return
	}

	database, err := datastore.NewDB(appDir)
	if err != nil {
		log.Printf("failed to initialize datastore: %v", err)
		return
	}
	a.db = database
}

// shutdown is called when the app closes
func (a *App) shutdown(ctx context.Context) {
	if a.db != nil {
		a.db.Close()
	}
}

// GetVersion returns application version
func (a *App) GetVersion() string {
	return "0.1.0"
}

// GetConfig retrieves the current settings.
func (a *App) GetConfig() (*datastore.Config, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return a.db.GetConfig()
}

// SaveConfig saves the configuration.
func (a *App) SaveConfig(cfg *datastore.Config) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.db.SaveConfig(cfg)
}

// StartScan triggers asynchronous subnet range scanning.
func (a *App) StartScan(target string) error {
	a.mu.Lock()
	if a.scanning {
		a.mu.Unlock()
		return fmt.Errorf("scan is already running")
	}
	a.scanning = true
	a.mu.Unlock()

	cfg, err := a.GetConfig()
	if err != nil {
		a.mu.Lock()
		a.scanning = false
		a.mu.Unlock()
		return err
	}

	scanCtx, cancel := context.WithCancel(context.Background())
	a.scanCancel = cancel

	go func() {
		defer func() {
			a.mu.Lock()
			a.scanning = false
			a.mu.Unlock()
		}()

		results, err := scanner.PerformScan(scanCtx, target, cfg, func(percent int, msg string) {
			wailsruntime.EventsEmit(a.ctx, "scan_progress", map[string]interface{}{
				"percent": percent,
				"message": msg,
			})
		})

		if err != nil {
			wailsruntime.EventsEmit(a.ctx, "scan_complete", map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
			return
		}

		a.mu.Lock()
		a.latestScanResults = results
		a.mu.Unlock()

		wailsruntime.EventsEmit(a.ctx, "scan_complete", map[string]interface{}{
			"success": true,
			"count":   len(results),
		})
	}()

	return nil
}

// StopScan cancels an active scanning process.
func (a *App) StopScan() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.scanCancel != nil {
		a.scanCancel()
		a.scanCancel = nil
	}
	a.scanning = false
}

// GetScanResultsJSON returns the latest raw scan results in JSON format for debugging.
func (a *App) GetScanResultsJSON() ([]*scanner.ScanResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.latestScanResults, nil
}

// RunAIInference takes the latest scan results, invokes the LLM, and updates the network map.
func (a *App) RunAIInference() (*datastore.NodeLinkData, error) {
	a.mu.Lock()
	if a.aiRunning {
		a.mu.Unlock()
		return nil, fmt.Errorf("AI inference is already running")
	}
	a.aiRunning = true
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.aiRunning = false
		a.mu.Unlock()
	}()

	a.mu.Lock()
	results := a.latestScanResults
	a.mu.Unlock()

	if len(results) == 0 {
		return nil, fmt.Errorf("no scan results available; run network scan first")
	}

	cfg, err := a.GetConfig()
	if err != nil {
		return nil, err
	}

	wailsruntime.EventsEmit(a.ctx, "ai_status", "Connecting to LLM and running inference...")
	llmResp, err := ai.RunInference(context.Background(), cfg, results)
	if err != nil {
		return nil, err
	}

	wailsruntime.EventsEmit(a.ctx, "ai_status", "Saving inferred nodes and connections...")

	// Read existing nodes to preserve manual edits/coordinates
	existingNodes, err := a.db.GetNodes()
	if err != nil {
		return nil, err
	}
	existingNodeMap := make(map[string]*datastore.Node)
	for _, n := range existingNodes {
		existingNodeMap[n.ID] = n
	}

	// 1. Map nodes
	var nodesToSave []*datastore.Node
	gridSpacing := 100.0
	xOffset := 100.0
	yOffset := 100.0

	for i, n := range llmResp.Nodes {
		// Find matching raw result to fetch additional properties (SysName, SysDesc, Vendor, IP, MAC)
		var match *scanner.ScanResult
		for _, r := range results {
			if r.IP == n.ID || r.MAC == n.ID {
				match = r
				break
			}
		}

		mac := ""
		vendor := ""
		sysName := ""
		sysDesc := ""
		ip := n.ID // Default ID is IP

		if match != nil {
			mac = match.MAC
			vendor = match.Vendor
			sysName = match.SysName
			sysDesc = match.SysDesc
			ip = match.IP
		}

		node := &datastore.Node{
			ID:             n.ID,
			IP:             ip,
			MAC:            mac,
			Vendor:         vendor,
			Label:          n.Label,
			Type:           n.Type,
			Reason:         n.Reason,
			SysName:        sysName,
			SysDesc:        sysDesc,
			X:              xOffset + float64(i%5)*gridSpacing,
			Y:              yOffset + float64(i/5)*gridSpacing,
			ManuallyEdited: false,
		}

		// Preserve coordinate/type/label edits if already in database
		if exist, ok := existingNodeMap[n.ID]; ok {
			node.X = exist.X
			node.Y = exist.Y
			if exist.ManuallyEdited {
				node.Label = exist.Label
				node.Type = exist.Type
				node.ManuallyEdited = true
			}
		}
		nodesToSave = append(nodesToSave, node)
	}

	// Save to DB
	if err := a.db.ClearAllNodes(); err != nil {
		return nil, err
	}
	if err := a.db.SaveNodes(nodesToSave); err != nil {
		return nil, err
	}

	// 2. Map links
	var linksToSave []*datastore.Link
	for i, l := range llmResp.Links {
		link := &datastore.Link{
			ID:            fmt.Sprintf("link_%d", i),
			From:          l.From,
			To:            l.To,
			Type:          l.Type,
			ManuallyAdded: false,
		}
		linksToSave = append(linksToSave, link)
	}

	if err := a.db.ClearAllLinks(); err != nil {
		return nil, err
	}
	if err := a.db.SaveLinks(linksToSave); err != nil {
		return nil, err
	}

	return a.GetNetworkMap()
}

// GetNetworkMap returns the currently saved nodes and links.
func (a *App) GetNetworkMap() (*datastore.NodeLinkData, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	nodes, err := a.db.GetNodes()
	if err != nil {
		return nil, err
	}
	links, err := a.db.GetLinks()
	if err != nil {
		return nil, err
	}

	return &datastore.NodeLinkData{
		Nodes: nodes,
		Links: links,
	}, nil
}

// SaveNode saves or updates a node manually.
func (a *App) SaveNode(node datastore.Node) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	node.ManuallyEdited = true
	return a.db.SaveNode(&node)
}

// DeleteNode deletes a node and any connected links.
func (a *App) DeleteNode(id string) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := a.db.DeleteNode(id); err != nil {
		return err
	}

	// Clean up links connected to this node
	links, err := a.db.GetLinks()
	if err != nil {
		return err
	}
	for _, l := range links {
		if l.From == id || l.To == id {
			if err := a.db.DeleteLink(l.ID); err != nil {
				log.Printf("failed to delete connected link %s: %v", l.ID, err)
			}
		}
	}
	return nil
}

// AddLink manually adds a connection between nodes.
func (a *App) AddLink(from, to, lType string) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	link := &datastore.Link{
		ID:            fmt.Sprintf("link_manual_%s_%s", from, to),
		From:          from,
		To:            to,
		Type:          lType,
		ManuallyAdded: true,
	}
	return a.db.SaveLink(link)
}

// DeleteLink manually deletes a connection.
func (a *App) DeleteLink(id string) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.db.DeleteLink(id)
}

// PositionNode represents a node position updates.
type PositionNode struct {
	ID string  `json:"id"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
}

// UpdateNodePositions saves updated positions of multiple nodes.
func (a *App) UpdateNodePositions(positions []PositionNode) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	nodes, err := a.db.GetNodes()
	if err != nil {
		return err
	}
	nodeMap := make(map[string]*datastore.Node)
	for _, n := range nodes {
		nodeMap[n.ID] = n
	}

	for _, pos := range positions {
		if n, ok := nodeMap[pos.ID]; ok {
			n.X = pos.X
			n.Y = pos.Y
			if err := a.db.SaveNode(n); err != nil {
				return err
			}
		}
	}
	return nil
}

// ClearMap clears the entire network map.
func (a *App) ClearMap() error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := a.db.ClearAllNodes(); err != nil {
		return err
	}
	return a.db.ClearAllLinks()
}
