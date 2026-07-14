# twNetMap

[日本語 (Japanese)](file:///Users/ymi/prj/twsnmp/twNetMap/README_ja.md)

An AI-powered network discovery tool that automatically generates network maps from scanned data. Built with Go, Wails v2, and Svelte.

---

## Key Features

1. **Active & Passive Network Scanning**
   - **Ping Check**: Verifies host reachability using ICMP (unprivileged UDP ping with a fallback to OS native commands).
   - **ARP Table Parsing**: Automatically extracts IP-MAC mapping tables from the local system and active SNMP agents.
   - **Port Scanning**: Scans common TCP ports (21, 22, 23, 25, 80, 110, 143, 161, 443, 3306, 3389, 5432, 8080, 9100).
   - **SNMP Query (v2c/v3)**: Queries remote agents to retrieve system info (`sysName`, `sysDesc`), physical MAC addresses, and Link Layer Discovery Protocol (LLDP) neighbor details.
   - **Service Banner Grabbing**: Connects to open ports to capture SSH/FTP banners and parses/cleans HTML response titles.

2. **AI-Driven Topology Inference**
   - Integrates with multiple LLM providers: **Ollama**, **OpenAI**, and **Google Gemini** using [langchaingo](file:///Users/ymi/prj/twsnmp/twNetMap/go.mod#L8).
   - Classifies device types into standard categories: `router`, `switch`, `wifi`, `mobile`, `pc`, `server`, `printer`, or `unknown`.
   - Utilizes structural reasoning (e.g., LLDP topology information) to automatically construct link relationships between devices.
   - **Feedback Loop**: Incorporates the user's manual modifications (node details or deleted links) back into the prompt history to adapt future inferences to user preferences.

3. **Interactive Network Map Visualization**
   - Renders maps dynamically using `vis-network`.
   - Allows users to add, edit, and delete nodes and links manually.
   - Drag-and-drop nodes to customize layouts or trigger automatic node rearrangement.

4. **Comprehensive Data Export**
   - **Graphics/Documents**: PNG, SVG, PDF
   - **Diagrams**: Draw.io (`.drawio`)
   - **Data**: JSON Map Data, JSON Raw Scan Results, CSV Node List, Excel Document (`.xlsx`)

---

## Technical Stack

- **Backend (Go)**
  - Application Framework: [Wails v2](https://wails.io) (v2.12.0)
  - Database: [bbolt](https://github.com/etcd-io/bbolt) (embedded key-value store)
  - LLM Orchestration: [langchaingo](https://github.com/tmc/langchaingo)
  - SNMP Clients: [gosnmp](https://github.com/gosnmp/gosnmp)
  - Exporters: [gopdf](https://github.com/signintech/gopdf), [excelize](https://github.com/xuri/excelize)
- **Frontend (Svelte & CSS)**
  - UI Library: Svelte 5
  - Build System: Vite
  - Styles: Tailwind CSS 3
  - Visualization: `vis-network`

---

## Project Structure

- [main.go](file:///Users/ymi/prj/twsnmp/twNetMap/main.go): The desktop entry point that boots the Wails application.
- [app.go](file:///Users/ymi/prj/twsnmp/twNetMap/app.go): Wails binding methods exposing core database operations, scanning control, AI logic, and file dialogs.
- `backend/`:
  - [ai/ai.go](file:///Users/ymi/prj/twsnmp/twNetMap/backend/ai/ai.go): Formulates system/user LLM prompts and handles provider authentication (Gemini, OpenAI, Ollama).
  - [scanner/scanner.go](file:///Users/ymi/prj/twsnmp/twNetMap/backend/scanner/scanner.go): Handles IP range parsing, ICMP/Ping, TCP port sweeps, SNMP walking, and banner grabbing.
  - [datastore/db.go](file:///Users/ymi/prj/twsnmp/twNetMap/backend/datastore/db.go): Manages local `bbolt` buckets storing scan results, node configurations, and user action history.
- `frontend/`:
  - `src/App.svelte`: Root view coordinating layout and page routing.
  - `src/routes/`:
    - `NetworkMap.svelte`: The visual interface displaying nodes/links and handling user map actions.
    - `NodeList.svelte`: Direct list/table editor for discovered devices.
    - `ScanSettings.svelte` / `AISettings.svelte`: Admin dashboards for scan targets and AI provider settings.

---

## Getting Started

### Prerequisites
- Go 1.26.5 or higher
- Node.js (with npm)
- Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)

### Run in Development Mode
To launch the application in debug mode with hot reloading:
```bash
wails dev
```

### Build Production Binary
To compile the standalone production binary for your operating system:
```bash
wails build
```
