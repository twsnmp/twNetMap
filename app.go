package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"twNetMap/backend/ai"
	"twNetMap/backend/datastore"
	"twNetMap/backend/scanner"

	"github.com/signintech/gopdf"
	"github.com/xuri/excelize/v2"
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
	version           string
}

// NewApp creates a new App application struct
func NewApp(version string) *App {
	return &App{
		version: version,
	}
}

// deviceTypeY defines the canonical Y-coordinate (vertical tier) for each device type
// in the tiered topology layout. Both RunAIInference and RearrangeNodes use this map
// so that layouts are always consistent.
var deviceTypeY = map[string]float64{
	"router":  100.0,
	"switch":  250.0,
	"server":  400.0,
	"pc":      550.0,
	"printer": 700.0,
	"wifi":    850.0,
	"mobile":  1000.0,
	"unknown": 1150.0,
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

	// Restore latest scan results from database
	if data, err := database.GetLatestScanResults(); err == nil && len(data) > 0 {
		var results []*scanner.ScanResult
		if err := json.Unmarshal(data, &results); err == nil {
			a.latestScanResults = results
		} else {
			log.Printf("failed to unmarshal restored scan results: %v", err)
		}
	}
}

// shutdown is called when the app closes
func (a *App) shutdown(ctx context.Context) {
	if a.db != nil {
		a.db.Close()
	}
}

// GetVersion returns application version
func (a *App) GetVersion() string {
	return a.version
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
		}, func(res *scanner.ScanResult) {
			id := res.IP
			if res.MAC != "" {
				id = res.MAC
			}
			wailsruntime.EventsEmit(a.ctx, "node_detected", map[string]interface{}{
				"id":      id,
				"ip":      res.IP,
				"mac":     res.MAC,
				"vendor":  res.Vendor,
				"sysName": res.SysName,
				"sysDesc": res.SysDesc,
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

		// Save raw scan results to database
		if data, err := json.Marshal(results); err == nil {
			_ = a.db.SaveLatestScanResults(data)
		}

		// Save scanned results as temporary nodes in DB
		existingNodes, err := a.db.GetNodes()
		if err == nil {
			existingNodeMap := make(map[string]*datastore.Node)
			for _, n := range existingNodes {
				existingNodeMap[n.ID] = n
			}

			var nodesToSave []*datastore.Node
			gridSpacing := 180.0
			xOffset := 100.0
			yOffset := 100.0
			columns := 8

			seenIDs := make(map[string]bool)
			for _, r := range results {
				id := r.IP
				if r.MAC != "" {
					id = r.MAC
				}

				if seenIDs[id] {
					continue // skip duplicate IDs to prevent DB overwrite and layout gaps
				}
				seenIDs[id] = true

				label := r.IP
				if r.SysName != "" {
					label = r.SysName
				}

				node := &datastore.Node{
					ID:             id,
					IP:             r.IP,
					MAC:            r.MAC,
					Vendor:         r.Vendor,
					Label:          label,
					Type:           "unknown",
					Reason:         "Detected during active scan",
					SysName:        r.SysName,
					SysDesc:        r.SysDesc,
					ManuallyEdited: false,
				}

				if exist, ok := existingNodeMap[id]; ok {
					node.Label = exist.Label
					if !exist.ManuallyEdited && r.SysName != "" {
						node.Label = r.SysName
					}
					node.Type = exist.Type
					node.ManuallyEdited = exist.ManuallyEdited
				}
				nodesToSave = append(nodesToSave, node)
			}

			// Sort nodesToSave by IP address
			sortNodesByIP(nodesToSave)

			// Assign coordinates based on sorted order (preserving manually edited positions)
			autoIndex := 0
			for _, node := range nodesToSave {
				if exist, ok := existingNodeMap[node.ID]; ok && exist.ManuallyEdited {
					node.X = exist.X
					node.Y = exist.Y
				} else {
					node.X = xOffset + float64(autoIndex%columns)*gridSpacing
					node.Y = yOffset + float64(autoIndex/columns)*150.0
					autoIndex++
				}
			}

			_ = a.db.SaveNodes(nodesToSave)
		}

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
	nodeHistories, _ := a.db.GetNodeHistories()
	linkHistories, _ := a.db.GetLinkHistories()

	llmResp, err := ai.RunInference(context.Background(), cfg, results, nodeHistories, linkHistories)
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

	// Make map of node history for quick lookup
	nodeHistMap := make(map[string]*datastore.NodeHistory)
	for _, nh := range nodeHistories {
		nodeHistMap[nh.ID] = nh
	}

	// 1. Map nodes
	var nodesToSave []*datastore.Node
	gridSpacing := 180.0
	xOffset := 100.0

	seenIDs := make(map[string]bool)
	for _, n := range llmResp.Nodes {
		if seenIDs[n.ID] {
			continue // skip duplicate IDs to prevent DB overwrite and layout gaps
		}
		seenIDs[n.ID] = true

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
			ManuallyEdited: false,
		}

		// Apply history if exists
		if nh, ok := nodeHistMap[node.ID]; ok {
			node.Label = nh.Label
			node.Type = nh.Type
			node.ManuallyEdited = true
		}

		// Preserve coordinate edits if already in database (takes precedence for position)
		if exist, ok := existingNodeMap[n.ID]; ok {
			if exist.ManuallyEdited {
				node.ManuallyEdited = true
				node.X = exist.X
				node.Y = exist.Y
			}
		}
		nodesToSave = append(nodesToSave, node)
	}

	// Sort nodesToSave by IP address so horizontal layout within each tier is ordered by IP
	sortNodesByIP(nodesToSave)

	// Assign coordinates based on tiered topology layout
	typeCount := make(map[string]int)

	for _, node := range nodesToSave {
		if exist, ok := existingNodeMap[node.ID]; ok && exist.ManuallyEdited {
			node.X = exist.X
			node.Y = exist.Y
		} else {
			t := node.Type
			y, found := deviceTypeY[t]
			if !found {
				y = deviceTypeY["unknown"]
				t = "unknown"
			}
			col := typeCount[t] % 8
			row := typeCount[t] / 8
			node.X = xOffset + float64(col)*gridSpacing
			node.Y = y + float64(row)*80.0
			typeCount[t]++
		}
	}

	// Save to DB
	if err := a.db.ClearAllNodes(); err != nil {
		return nil, err
	}
	if err := a.db.SaveNodes(nodesToSave); err != nil {
		return nil, err
	}

	// 2. Map links with history logic
	linkHistMap := make(map[string]*datastore.LinkHistory)
	for _, lh := range linkHistories {
		linkHistMap[lh.ID] = lh
	}

	var linksToSave []*datastore.Link
	existingLinks := make(map[string]bool)
	linkIdx := 0

	for _, l := range llmResp.Links {
		lhID := getLinkHistoryID(l.From, l.To)
		if hist, ok := linkHistMap[lhID]; ok {
			if hist.Deleted {
				// Skip deleted link
				continue
			}
			// Use historical override type and style
			link := &datastore.Link{
				ID:            fmt.Sprintf("link_%d", linkIdx),
				From:          l.From,
				To:            l.To,
				Type:          hist.Type,
				Style:         hist.Style,
				ManuallyAdded: false,
			}
			linksToSave = append(linksToSave, link)
			existingLinks[lhID] = true
			linkIdx++
			continue
		}

		link := &datastore.Link{
			ID:            fmt.Sprintf("link_%d", linkIdx),
			From:          l.From,
			To:            l.To,
			Type:          l.Type,
			Style:         l.Style,
			ManuallyAdded: false,
		}
		linksToSave = append(linksToSave, link)
		existingLinks[lhID] = true
		linkIdx++
	}

	// Add manual links that LLM did not generate
	for _, hist := range linkHistories {
		if !hist.Deleted {
			lhID := getLinkHistoryID(hist.From, hist.To)
			if !existingLinks[lhID] {
				link := &datastore.Link{
					ID:            fmt.Sprintf("link_manual_%s_%s", hist.From, hist.To),
					From:          hist.From,
					To:            hist.To,
					Type:          hist.Type,
					Style:         hist.Style,
					ManuallyAdded: true,
				}
				linksToSave = append(linksToSave, link)
				existingLinks[lhID] = true
				linkIdx++
			}
		}
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

func getLinkHistoryID(from, to string) string {
	if from < to {
		return from + "_" + to
	}
	return to + "_" + from
}

// SaveNode saves or updates a node manually.
func (a *App) SaveNode(node datastore.Node) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	node.ManuallyEdited = true
	if err := a.db.SaveNode(&node); err != nil {
		return err
	}
	// Save to history
	nh := &datastore.NodeHistory{
		ID:        node.ID,
		IP:        node.IP,
		MAC:       node.MAC,
		Label:     node.Label,
		Type:      node.Type,
		UpdatedAt: time.Now(),
	}
	return a.db.SaveNodeHistory(nh)
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
			if err := a.DeleteLink(l.ID); err != nil {
				log.Printf("failed to delete connected link %s: %v", l.ID, err)
			}
		}
	}
	return nil
}

// AddLink manually adds a connection between nodes.
func (a *App) AddLink(from, to, label, style string) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	link := &datastore.Link{
		ID:            fmt.Sprintf("link_manual_%s_%s", from, to),
		From:          from,
		To:            to,
		Type:          label,
		Style:         style,
		ManuallyAdded: true,
	}
	if err := a.db.SaveLink(link); err != nil {
		return err
	}
	// Save to history
	lh := &datastore.LinkHistory{
		ID:        getLinkHistoryID(from, to),
		From:      from,
		To:        to,
		Type:      label,
		Style:     style,
		Deleted:   false,
		UpdatedAt: time.Now(),
	}
	return a.db.SaveLinkHistory(lh)
}

// UpdateLink updates an existing link's label and style.
func (a *App) UpdateLink(id, label, style string) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	link, err := a.db.GetLink(id)
	if err != nil {
		return err
	}
	link.Type = label
	link.Style = style
	if err := a.db.SaveLink(link); err != nil {
		return err
	}
	// Save to history
	lh := &datastore.LinkHistory{
		ID:        getLinkHistoryID(link.From, link.To),
		From:      link.From,
		To:        link.To,
		Type:      label,
		Style:     style,
		Deleted:   false,
		UpdatedAt: time.Now(),
	}
	return a.db.SaveLinkHistory(lh)
}

// DeleteLink manually deletes a connection.
func (a *App) DeleteLink(id string) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	link, err := a.db.GetLink(id)
	if err != nil {
		// If not found in current map, just try to delete from DB anyway
		return a.db.DeleteLink(id)
	}

	if err := a.db.DeleteLink(id); err != nil {
		return err
	}

	// Update history
	lhID := getLinkHistoryID(link.From, link.To)
	if link.ManuallyAdded {
		// If it was manually added, delete the manual add history
		return a.db.DeleteLinkHistory(lhID)
	} else {
		// If it was auto-detected (AI/LLM-generated), record that it was deleted by user
		lh := &datastore.LinkHistory{
			ID:        lhID,
			From:      link.From,
			To:        link.To,
			Type:      link.Type,
			Deleted:   true,
			UpdatedAt: time.Now(),
		}
		return a.db.SaveLinkHistory(lh)
	}
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
			n.ManuallyEdited = true // Mark as manually edited when dragged
			if err := a.db.SaveNode(n); err != nil {
				return err
			}
		}
	}
	return nil
}

// RearrangeNodes recalculates positions of all nodes.
// If preserveManual is true, manually edited coordinates are kept.
// If preserveManual is false, it forces a complete layout reset for all nodes.
func (a *App) RearrangeNodes(preserveManual bool) (*datastore.NodeLinkData, error) {
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

	gridSpacing := 180.0
	xOffset := 100.0
	yOffset := 100.0
	columns := 8

	// Sort nodes by IP address first, so horizontal layout matches IP order
	sortNodesByIP(nodes)

	if len(links) > 0 {
		// Tiered layout by device type for topology (uses shared deviceTypeY for consistency)
		typeCount := make(map[string]int)

		for _, node := range nodes {
			if preserveManual && node.ManuallyEdited {
				continue
			}
			t := node.Type
			y, found := deviceTypeY[t]
			if !found {
				y = deviceTypeY["unknown"]
				t = "unknown"
			}
			col := typeCount[t] % 8
			row := typeCount[t] / 8
			node.X = xOffset + float64(col)*gridSpacing
			node.Y = y + float64(row)*80.0
			typeCount[t]++
			if !preserveManual {
				node.ManuallyEdited = false
			}
		}
	} else {
		// Grid layout sorted by IP address
		autoIndex := 0
		for _, node := range nodes {
			if preserveManual && node.ManuallyEdited {
				continue
			}
			node.X = xOffset + float64(autoIndex%columns)*gridSpacing
			node.Y = yOffset + float64(autoIndex/columns)*150.0
			autoIndex++
			if !preserveManual {
				node.ManuallyEdited = false
			}
		}
	}

	if err := a.db.SaveNodes(nodes); err != nil {
		return nil, err
	}

	return a.GetNetworkMap()
}

// sortNodesByIP sorts a slice of *datastore.Node by their IP address.
func sortNodesByIP(nodes []*datastore.Node) {
	sort.Slice(nodes, func(i, j int) bool {
		ip1 := net.ParseIP(nodes[i].IP)
		ip2 := net.ParseIP(nodes[j].IP)
		if ip1 == nil && ip2 == nil {
			return nodes[i].IP < nodes[j].IP
		}
		if ip1 == nil {
			return false // invalid IP goes to the end
		}
		if ip2 == nil {
			return true
		}
		return bytes.Compare(ip1.To16(), ip2.To16()) < 0
	})
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

// GetHistory retrieves all user edit histories.
func (a *App) GetHistory() (*datastore.NodeLinkHistoryData, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	nodes, err := a.db.GetNodeHistories()
	if err != nil {
		return nil, err
	}
	links, err := a.db.GetLinkHistories()
	if err != nil {
		return nil, err
	}
	return &datastore.NodeLinkHistoryData{
		Nodes: nodes,
		Links: links,
	}, nil
}

// DeleteNodeHistory deletes a specific node history.
func (a *App) DeleteNodeHistory(id string) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.db.DeleteNodeHistory(id)
}

// DeleteLinkHistory deletes a specific link history.
func (a *App) DeleteLinkHistory(id string) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.db.DeleteLinkHistory(id)
}

// ClearAllHistory deletes all history records.
func (a *App) ClearAllHistory() error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.db.ClearAllHistory()
}

// ExportMap exports the map or data to a file.
func (a *App) ExportMap(format string, pngBase64 string) (string, error) {
	if a.db == nil {
		return "", fmt.Errorf("database not initialized")
	}

	var title string
	var defaultFilename string
	var filters []wailsruntime.FileFilter

	switch format {
	case "png":
		title = "Export as PNG Image"
		defaultFilename = "network_map.png"
		filters = []wailsruntime.FileFilter{{DisplayName: "PNG Image (*.png)", Pattern: "*.png"}}
	case "svg":
		title = "Export as SVG Image"
		defaultFilename = "network_map.svg"
		filters = []wailsruntime.FileFilter{{DisplayName: "SVG Image (*.svg)", Pattern: "*.svg"}}
	case "pdf":
		title = "Export as PDF Document"
		defaultFilename = "network_map.pdf"
		filters = []wailsruntime.FileFilter{{DisplayName: "PDF Document (*.pdf)", Pattern: "*.pdf"}}
	case "drawio":
		title = "Export as Draw.io Diagram"
		defaultFilename = "network_map.drawio"
		filters = []wailsruntime.FileFilter{{DisplayName: "Draw.io Diagram (*.drawio)", Pattern: "*.drawio"}}
	case "json_map":
		title = "Export Map Data (JSON)"
		defaultFilename = "network_map_data.json"
		filters = []wailsruntime.FileFilter{{DisplayName: "JSON Map Data (*.json)", Pattern: "*.json"}}
	case "json_scan":
		title = "Export Scan Data (JSON)"
		defaultFilename = "scan_results.json"
		filters = []wailsruntime.FileFilter{{DisplayName: "JSON Scan Data (*.json)", Pattern: "*.json"}}
	case "csv":
		title = "Export Node List (CSV)"
		defaultFilename = "node_list.csv"
		filters = []wailsruntime.FileFilter{{DisplayName: "CSV Node List (*.csv)", Pattern: "*.csv"}}
	case "excel":
		title = "Export Excel Document"
		defaultFilename = "network_map.xlsx"
		filters = []wailsruntime.FileFilter{{DisplayName: "Excel Document (*.xlsx)", Pattern: "*.xlsx"}}
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}

	selectedFile, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           title,
		DefaultFilename: defaultFilename,
		Filters:         filters,
	})
	if err != nil {
		return "", err
	}
	if selectedFile == "" {
		return "", nil // user cancelled
	}

	// 1. Get database records
	nodes, err := a.db.GetNodes()
	if err != nil {
		return "", fmt.Errorf("failed to fetch nodes: %v", err)
	}
	links, err := a.db.GetLinks()
	if err != nil {
		return "", fmt.Errorf("failed to fetch links: %v", err)
	}

	// 2. Decode PNG bytes if provided and needed
	var pngBytes []byte
	if pngBase64 != "" {
		parts := strings.SplitN(pngBase64, ",", 2)
		base64Data := parts[0]
		if len(parts) > 1 {
			base64Data = parts[1]
		}
		decoded, err := base64.StdEncoding.DecodeString(base64Data)
		if err != nil {
			return "", fmt.Errorf("failed to decode base64 PNG: %v", err)
		}
		pngBytes = decoded
	}

	// 3. Export formatting logic
	switch format {
	case "png":
		if len(pngBytes) == 0 {
			return "", fmt.Errorf("no PNG data provided")
		}
		if err := os.WriteFile(selectedFile, pngBytes, 0644); err != nil {
			return "", err
		}

	case "svg":
		minX, maxX := 0.0, 800.0
		minY, maxY := 0.0, 600.0
		if len(nodes) > 0 {
			minX = nodes[0].X
			maxX = nodes[0].X
			minY = nodes[0].Y
			maxY = nodes[0].Y
			for _, n := range nodes {
				if n.X < minX {
					minX = n.X
				}
				if n.X > maxX {
					maxX = n.X
				}
				if n.Y < minY {
					minY = n.Y
				}
				if n.Y > maxY {
					maxY = n.Y
				}
			}
		}
		margin := 100.0
		minX -= margin
		minY -= margin
		maxX += margin
		maxY += margin
		width := maxX - minX
		height := maxY - minY

		var svgBuf bytes.Buffer
		svgBuf.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="%f %f %f %f" width="100%%" height="100%%" style="background-color: #f8fafc;">`, minX, minY, width, height))
		svgBuf.WriteString(`<defs><marker id="arrow" viewBox="0 0 10 10" refX="20" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse"><path d="M 0 0 L 10 5 L 0 10 z" fill="#94a3b8"/></marker></defs>`)

		nodeMap := make(map[string]*datastore.Node)
		for _, n := range nodes {
			nodeMap[n.ID] = n
		}

		for _, l := range links {
			fromNode, ok1 := nodeMap[l.From]
			toNode, ok2 := nodeMap[l.To]
			if ok1 && ok2 {
				strokeWidth := "2"
				dashArray := ""
				if l.Style == "thin" {
					strokeWidth = "1"
				} else if l.Style == "thick" {
					strokeWidth = "4"
				} else if l.Style == "dotted" {
					strokeWidth = "2"
					dashArray = `stroke-dasharray="4,4"`
				}
				svgBuf.WriteString(fmt.Sprintf(`<line x1="%f" y1="%f" x2="%f" y2="%f" stroke="#94a3b8" stroke-width="%s" %s />`,
					fromNode.X, fromNode.Y, toNode.X, toNode.Y, strokeWidth, dashArray))

				midX := (fromNode.X + toNode.X) / 2
				midY := (fromNode.Y + toNode.Y) / 2
				if l.Type != "" {
					svgBuf.WriteString(fmt.Sprintf(`<text x="%f" y="%f" text-anchor="middle" font-size="10" fill="#64748b">%s</text>`, midX, midY-5, l.Type))
				}
			}
		}

		for _, n := range nodes {
			color := "#64748b"
			switch n.Type {
			case "router":
				color = "#ef4444"
			case "switch":
				color = "#10b981"
			case "pc":
				color = "#3b82f6"
			case "server":
				color = "#f59e0b"
			case "printer":
				color = "#8b5cf6"
			}
			svgBuf.WriteString(fmt.Sprintf(`<circle cx="%f" cy="%f" r="24" fill="%s" stroke="#ffffff" stroke-width="3" />`, n.X, n.Y, color))

			initial := "U"
			if len(n.Type) > 0 {
				initial = strings.ToUpper(n.Type[:1])
			}
			svgBuf.WriteString(fmt.Sprintf(`<text x="%f" y="%f" dy="6" text-anchor="middle" font-size="16" font-weight="bold" fill="#ffffff">%s</text>`, n.X, n.Y, initial))
			svgBuf.WriteString(fmt.Sprintf(`<text x="%f" y="%f" text-anchor="middle" font-size="11" font-weight="600" fill="#1e293b">%s</text>`, n.X, n.Y+38, n.Label))
			if n.IP != "" && n.IP != n.Label {
				svgBuf.WriteString(fmt.Sprintf(`<text x="%f" y="%f" text-anchor="middle" font-size="9" fill="#64748b">%s</text>`, n.X, n.Y+50, n.IP))
			}
		}
		svgBuf.WriteString(`</svg>`)
		if err := os.WriteFile(selectedFile, svgBuf.Bytes(), 0644); err != nil {
			return "", err
		}

	case "pdf":
		if len(pngBytes) == 0 {
			return "", fmt.Errorf("no PNG data provided")
		}
		tmpFile, err := os.CreateTemp("", "twNetMap-*.png")
		if err != nil {
			return "", err
		}
		defer os.Remove(tmpFile.Name())
		if _, err := tmpFile.Write(pngBytes); err != nil {
			return "", err
		}
		tmpFile.Close()

		pdf := gopdf.GoPdf{}
		pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
		pdf.AddPage()

		// PDF Width: 595, Height: 842. Margin: 20
		err = pdf.Image(tmpFile.Name(), 20, 20, &gopdf.Rect{W: 555, H: 416})
		if err != nil {
			return "", fmt.Errorf("failed to add image to PDF: %v", err)
		}

		if err := pdf.WritePdf(selectedFile); err != nil {
			return "", err
		}

	case "drawio":
		minX, minY := 0.0, 0.0
		if len(nodes) > 0 {
			minX = nodes[0].X
			minY = nodes[0].Y
			for _, n := range nodes {
				if n.X < minX {
					minX = n.X
				}
				if n.Y < minY {
					minY = n.Y
				}
			}
		}

		var drawioBuf bytes.Buffer
		drawioBuf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<mxfile host="Electron" modified="2026-07-14T00:00:00.000Z" agent="5.0" version="20.0.0" type="device">
  <diagram id="netmap" name="Network Map">
    <mxGraphModel dx="1000" dy="1000" grid="1" gridSize="10" guides="1" tooltips="1" connect="1" arrows="1" fold="1" page="1" pageScale="1" pageWidth="827" pageHeight="1169" math="0" shadow="0">
      <root>
        <mxCell id="0" />
        <mxCell id="1" parent="0" />`)

		for _, n := range nodes {
			x := n.X - minX + 50.0
			y := n.Y - minY + 50.0
			fillColor := "#F5F5F5"
			strokeColor := "#666666"
			switch n.Type {
			case "router":
				fillColor = "#F8CECC"
				strokeColor = "#B85450"
			case "switch":
				fillColor = "#D5E8D4"
				strokeColor = "#82B366"
			case "pc":
				fillColor = "#DAE8FC"
				strokeColor = "#6C8EBF"
			case "server":
				fillColor = "#FFF2CC"
				strokeColor = "#D6B656"
			case "printer":
				fillColor = "#E1D5E7"
				strokeColor = "#9673A6"
			}
			value := fmt.Sprintf("%s\n%s", n.Label, n.IP)
			value = strings.ReplaceAll(value, "\n", "&lt;br/&gt;")
			drawioBuf.WriteString(fmt.Sprintf(`
        <mxCell id="%s" value="%s" style="rounded=1;whiteSpace=wrap;html=1;fillColor=%s;strokeColor=%s;fontStyle=1" vertex="1" parent="1">
          <mxGeometry x="%f" y="%f" width="100" height="50" as="geometry" />
        </mxCell>`, n.ID, value, fillColor, strokeColor, x, y))
		}

		for _, l := range links {
			style := "endArrow=none;html=1;rounded=0;"
			if l.Style == "thin" {
				style += "strokeWidth=1;"
			} else if l.Style == "thick" {
				style += "strokeWidth=4;"
			} else if l.Style == "dotted" {
				style += "strokeWidth=2;dashed=1;"
			} else {
				style += "strokeWidth=2;"
			}
			drawioBuf.WriteString(fmt.Sprintf(`
        <mxCell id="%s" value="%s" style="%s" edge="1" parent="1" source="%s" target="%s">
          <mxGeometry relative="1" as="geometry" />
        </mxCell>`, l.ID, l.Type, style, l.From, l.To))
		}

		drawioBuf.WriteString(`
      </root>
    </mxGraphModel>
  </diagram>
</mxfile>`)
		if err := os.WriteFile(selectedFile, drawioBuf.Bytes(), 0644); err != nil {
			return "", err
		}

	case "json_map":
		data := &datastore.NodeLinkData{Nodes: nodes, Links: links}
		indent, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(selectedFile, indent, 0644); err != nil {
			return "", err
		}

	case "json_scan":
		var scanResults []*scanner.ScanResult
		a.mu.Lock()
		scanResults = a.latestScanResults
		a.mu.Unlock()

		if len(scanResults) == 0 {
			// Try to restore from DB if memory is empty
			if data, err := a.db.GetLatestScanResults(); err == nil && len(data) > 0 {
				_ = json.Unmarshal(data, &scanResults)
			}
		}

		indent, err := json.MarshalIndent(scanResults, "", "  ")
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(selectedFile, indent, 0644); err != nil {
			return "", err
		}

	case "csv":
		var buf bytes.Buffer
		writer := csv.NewWriter(&buf)
		writer.Write([]string{"ID", "IP", "MAC", "Vendor", "Label", "Type", "SysName", "SysDesc", "X", "Y", "ManuallyEdited"})
		for _, n := range nodes {
			writer.Write([]string{
				n.ID,
				n.IP,
				n.MAC,
				n.Vendor,
				n.Label,
				n.Type,
				n.SysName,
				n.SysDesc,
				fmt.Sprintf("%f", n.X),
				fmt.Sprintf("%f", n.Y),
				fmt.Sprintf("%t", n.ManuallyEdited),
			})
		}
		writer.Flush()
		if err := os.WriteFile(selectedFile, buf.Bytes(), 0644); err != nil {
			return "", err
		}

	case "excel":
		xlsx := excelize.NewFile()
		xlsx.SetSheetName("Sheet1", "Map Image")

		if len(pngBytes) > 0 {
			tmpFile, err := os.CreateTemp("", "twNetMap-*.png")
			if err != nil {
				return "", err
			}
			defer os.Remove(tmpFile.Name())
			if _, err := tmpFile.Write(pngBytes); err != nil {
				return "", err
			}
			tmpFile.Close()

			err = xlsx.AddPicture("Map Image", "B2", tmpFile.Name(), &excelize.GraphicOptions{
				ScaleX: 0.8,
				ScaleY: 0.8,
			})
			if err != nil {
				return "", fmt.Errorf("failed to add image to Excel: %v", err)
			}
		}

		xlsx.NewSheet("Node List")

		headerStyle, _ := xlsx.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
			Fill: excelize.Fill{Type: "pattern", Color: []string{"4F81BD"}, Pattern: 1},
		})

		headers := []string{"ID", "IP", "MAC", "Vendor", "Label", "Type", "SysName", "SysDesc", "X", "Y", "ManuallyEdited"}
		for colIdx, h := range headers {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
			xlsx.SetCellValue("Node List", cell, h)
			xlsx.SetCellStyle("Node List", cell, cell, headerStyle)
		}

		for rIdx, n := range nodes {
			row := rIdx + 2
			xlsx.SetCellValue("Node List", fmt.Sprintf("A%d", row), n.ID)
			xlsx.SetCellValue("Node List", fmt.Sprintf("B%d", row), n.IP)
			xlsx.SetCellValue("Node List", fmt.Sprintf("C%d", row), n.MAC)
			xlsx.SetCellValue("Node List", fmt.Sprintf("D%d", row), n.Vendor)
			xlsx.SetCellValue("Node List", fmt.Sprintf("E%d", row), n.Label)
			xlsx.SetCellValue("Node List", fmt.Sprintf("F%d", row), n.Type)
			xlsx.SetCellValue("Node List", fmt.Sprintf("G%d", row), n.SysName)
			xlsx.SetCellValue("Node List", fmt.Sprintf("H%d", row), n.SysDesc)
			xlsx.SetCellValue("Node List", fmt.Sprintf("I%d", row), n.X)
			xlsx.SetCellValue("Node List", fmt.Sprintf("J%d", row), n.Y)
			xlsx.SetCellValue("Node List", fmt.Sprintf("K%d", row), n.ManuallyEdited)
		}

		if err := xlsx.SaveAs(selectedFile); err != nil {
			return "", err
		}
	}

	return selectedFile, nil
}


