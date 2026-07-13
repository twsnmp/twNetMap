<script>
  import { onMount, onDestroy } from 'svelte';
  import { 
    GetNetworkMap, 
    SaveNode, 
    DeleteNode, 
    AddLink, 
    DeleteLink, 
    UpdateNodePositions,
    StartScan,
    StopScan,
    RunAIInference,
    ClearMap,
    RearrangeNodes
  } from '../../wailsjs/go/main/App';

  // Import vis-network standalone
  import { Network, DataSet } from 'vis-network/standalone';

  export let config = {};

  let container;
  let network = null;
  let nodesDataSet = new DataSet([]);
  let edgesDataSet = new DataSet([]);

  // Scanning State
  let scanning = false;
  let scanProgress = 0;
  let scanMessage = 'Idle';
  let aiRunning = false;
  let aiMessage = '';

  // Selected item detail for editing
  let selectedNode = null;
  let editNodeLabel = '';
  let editNodeType = 'unknown';
  let showEditModal = false;

  // Add custom node modal
  let showAddNodeModal = false;
  let addNodeIP = '';
  let addNodeLabel = '';
  let addNodeType = 'unknown';

  // Add custom link modal
  let showAddLinkModal = false;
  let addLinkFrom = '';
  let addLinkTo = '';
  let addLinkType = 'lan';

  // Custom confirmation modals
  let showClearConfirmModal = false;
  let showDeleteConfirmModal = false;
  let showRearrangeModal = false;

  // Layout Settings
  let layoutMode = 'hierarchical'; // 'hierarchical' | 'force' | 'static'
  let nodeSpacing = 150;

  function changeLayoutMode(mode) {
    layoutMode = mode;
    localStorage.setItem('twNetMap_layoutMode', mode);
    applyLayoutSettings();
  }

  function updateSpacing(e) {
    nodeSpacing = parseInt(e.target.value);
    localStorage.setItem('twNetMap_nodeSpacing', nodeSpacing.toString());
    applyLayoutSettings();
  }

  function applyLayoutSettings() {
    if (!network) return;

    let options = {};
    if (layoutMode === 'hierarchical') {
      options = {
        layout: {
          hierarchical: {
            enabled: true,
            direction: 'UD',
            sortMethod: 'directed',
            nodeSpacing: nodeSpacing,
            levelSeparation: nodeSpacing,
            parentCentralization: true,
            edgeMinimization: true,
            blockShifting: true
          }
        },
        physics: {
          enabled: false
        }
      };
    } else if (layoutMode === 'force') {
      const gravConstant = -15 * nodeSpacing;
      options = {
        layout: {
          hierarchical: {
            enabled: false
          }
        },
        physics: {
          enabled: true,
          solver: 'barnesHut',
          barnesHut: {
            gravitationalConstant: gravConstant,
            centralGravity: 0.1,
            springLength: nodeSpacing,
            springConstant: 0.04,
            damping: 0.09,
            avoidOverlap: 1
          },
          stabilization: { iterations: 150 }
        }
      };
    } else if (layoutMode === 'static') {
      options = {
        layout: {
          hierarchical: {
            enabled: false
          }
        },
        physics: {
          enabled: false
        }
      };
    }

    network.setOptions(options);

    if (layoutMode === 'hierarchical') {
      setTimeout(() => {
        if (network) network.fit();
      }, 200);
    }
  }

  // Modern UI notifications
  let errorMessage = '';
  let successMessage = '';

  function showError(msg) {
    errorMessage = msg;
    setTimeout(() => { errorMessage = ''; }, 5000);
  }

  function showSuccess(msg) {
    successMessage = msg;
    setTimeout(() => { successMessage = ''; }, 3000);
  }

  // Wails Event Listeners unsubscribe callbacks
  let unsubProgress = null;
  let unsubComplete = null;
  let unsubAiStatus = null;
  let unsubNodeDetected = null;

  // Dynamic SVG builder for device icons
  function getSvgIcon(type, color = '#38bdf8') {
    let path = '';
    if (type === 'router') {
      path = `<circle cx="24" cy="24" r="12" fill="none" stroke="${color}" stroke-width="3"/><path d="M24 2v10M24 36v10M2 24h10M36 24h10" stroke="${color}" stroke-width="3" stroke-linecap="round"/>`;
    } else if (type === 'switch') {
      path = `<rect x="6" y="14" width="36" height="20" rx="3" fill="none" stroke="${color}" stroke-width="3"/><path d="M12 24h24M18 20l-6 4 6 4M30 20l6 4-6 4" stroke="${color}" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>`;
    } else if (type === 'wifi') {
      path = `<path d="M24 38h.01M16.2 30.2a11 11 0 0 1 15.6 0M10.5 24.5a19 19 0 0 1 27 0M4.8 18.8a27 27 0 0 1 38.4 0" fill="none" stroke="${color}" stroke-width="3" stroke-linecap="round"/>`;
    } else if (type === 'mobile') {
      path = `<rect x="14" y="6" width="20" height="36" rx="3" fill="none" stroke="${color}" stroke-width="3"/><path d="M14 34h20M24 38h.01" fill="none" stroke="${color}" stroke-width="3" stroke-linecap="round"/>`;
    } else if (type === 'pc') {
      path = `<rect x="8" y="8" width="32" height="22" rx="2" fill="none" stroke="${color}" stroke-width="3"/><path d="M20 30v8M14 38h20M6 42h36" stroke="${color}" stroke-width="3" stroke-linecap="round"/>`;
    } else if (type === 'server') {
      path = `<rect x="8" y="6" width="32" height="10" rx="1" fill="none" stroke="${color}" stroke-width="2.5"/><rect x="8" y="19" width="32" height="10" rx="1" fill="none" stroke="${color}" stroke-width="2.5"/><rect x="8" y="32" width="32" height="10" rx="1" fill="none" stroke="${color}" stroke-width="2.5"/><circle cx="14" cy="11" r="1.5" fill="${color}"/><circle cx="14" cy="24" r="1.5" fill="${color}"/><circle cx="14" cy="37" r="1.5" fill="${color}"/>`;
    } else if (type === 'printer') {
      path = `<path d="M12 16V6h24v10M8 16h32v18H8zM12 28h24v14H12z" fill="none" stroke="${color}" stroke-width="3" stroke-linejoin="round"/>`;
    } else {
      path = `<circle cx="24" cy="24" r="16" fill="none" stroke="${color}" stroke-width="3" stroke-dasharray="4 4"/><path d="M24 14a4 4 0 0 1 4 4c0 2-1.5 3-2.5 4s-1.5 2-1.5 3.5M24 33h.01" stroke="${color}" stroke-width="3" stroke-linecap="round" fill="none"/>`;
    }
    
    const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" width="48" height="48">${path}</svg>`;
    return 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(svg);
  }

  // Color mapping based on device type
  function getColorForType(type) {
    switch (type) {
      case 'router': return '#f59e0b'; // Amber
      case 'switch': return '#3b82f6'; // Blue
      case 'wifi': return '#06b6d4';   // Cyan
      case 'mobile': return '#f43f5e'; // Rose
      case 'pc': return '#10b981';     // Emerald
      case 'server': return '#8b5cf6'; // Violet
      case 'printer': return '#ec4899';// Pink
      default: return '#94a3b8';       // Slate
    }
  }

  onMount(async () => {
    // Load layouts settings from localStorage
    const savedMode = localStorage.getItem('twNetMap_layoutMode');
    if (savedMode) layoutMode = savedMode;
    const savedSpacing = localStorage.getItem('twNetMap_nodeSpacing');
    if (savedSpacing) nodeSpacing = parseInt(savedSpacing);

    // 1. Initialise Wails Events Listeners
    if (window.runtime) {
      unsubProgress = window.runtime.EventsOn('scan_progress', (data) => {
        scanProgress = data.percent;
        scanMessage = data.message;
      });

      unsubComplete = window.runtime.EventsOn('scan_complete', async (data) => {
        scanning = false;
        if (data.success) {
          scanMessage = `Scan complete. Found ${data.count} nodes. Run AI Inference next!`;
          scanProgress = 100;
          await loadMapData();
        } else {
          scanMessage = `Scan failed: ${data.error}`;
          scanProgress = 0;
        }
      });

      unsubAiStatus = window.runtime.EventsOn('ai_status', (msg) => {
        aiMessage = msg;
      });

      unsubNodeDetected = window.runtime.EventsOn('node_detected', (data) => {
        const label = data.sysName || data.ip;
        const type = 'unknown';
        const count = nodesDataSet.length;
        
        const visNode = {
          id: data.id,
          label: label,
          shape: 'image',
          image: getSvgIcon(type, getColorForType(type)),
          x: 100 + (count % 10) * 100,
          y: 100 + Math.floor(count / 10) * 100,
          raw: {
            id: data.id,
            ip: data.ip,
            mac: data.mac,
            vendor: data.vendor,
            label: label,
            type: type,
            sysName: data.sysName,
            sysDesc: data.sysDesc,
            reason: 'Detected during active scan'
          }
        };

        if (nodesDataSet.get(data.id)) {
          nodesDataSet.update(visNode);
        } else {
          nodesDataSet.add(visNode);
        }
      });
    }

    // 2. Load map
    await loadMapData();
  });

  onDestroy(() => {
    if (unsubProgress) unsubProgress();
    if (unsubComplete) unsubComplete();
    if (unsubAiStatus) unsubAiStatus();
    if (unsubNodeDetected) unsubNodeDetected();
    if (network) {
      network.destroy();
    }
  });

  async function loadMapData() {
    try {
      const data = await GetNetworkMap();
      if (!data) return;

      const nodes = data.nodes || [];
      const links = data.links || [];

      const visNodes = nodes.map(n => ({
        id: n.id,
        label: n.label,
        shape: 'image',
        image: getSvgIcon(n.type, getColorForType(n.type)),
        x: n.x,
        y: n.y,
        // Save the raw node object for reference
        raw: n
      }));

      const visEdges = links.map(l => ({
        id: l.id,
        from: l.from,
        to: l.to,
        label: l.type === 'lan' ? '' : l.type,
        color: { color: '#475569', highlight: '#38bdf8' },
        width: 2,
        raw: l
      }));

      nodesDataSet.clear();
      nodesDataSet.add(visNodes);

      edgesDataSet.clear();
      edgesDataSet.add(visEdges);

      // Create network graph if not already initialized
      if (!network) {
        const dataSet = { nodes: nodesDataSet, edges: edgesDataSet };
        const options = {
          interaction: {
            hover: true,
            multiselect: false,
            navigationButtons: true,
            keyboard: true
          },
          nodes: {
            font: { color: '#cbd5e1', size: 14, face: 'sans-serif' }
          }
        };

        network = new Network(container, dataSet, options);
        applyLayoutSettings();

        // Position change watcher (preserves custom layouts)
        network.on('dragEnd', async (params) => {
          if (params.nodes.length > 0) {
            const positions = network.getPositions(params.nodes);
            const updates = Object.keys(positions).map(id => ({
              id: id,
              x: positions[id].x,
              y: positions[id].y
            }));
            try {
              await UpdateNodePositions(updates);
            } catch (err) {
              console.error('Failed to update node positions:', err);
            }
          }
        });

        // Double click handler -> edit node properties
        network.on('doubleClick', (params) => {
          if (params.nodes.length > 0) {
            const nodeId = params.nodes[0];
            const visNode = nodesDataSet.get(nodeId);
            if (visNode && visNode.raw) {
              selectedNode = visNode.raw;
              editNodeLabel = selectedNode.label;
              editNodeType = selectedNode.type;
              showEditModal = true;
            }
          }
        });
      }
    } catch (err) {
      console.error('Failed to load map data:', err);
    }
  }

  // Active subnets scanner control
  async function triggerScan() {
    scanning = true;
    scanProgress = 0;
    scanMessage = 'Starting scan...';
    try {
      await StartScan(config.Subnet);
    } catch (err) {
      scanMessage = `Error: ${err.message || err}`;
      scanning = false;
    }
  }

  function stopActiveScan() {
    StopScan();
    scanning = false;
    scanMessage = 'Scan cancelled by user.';
    scanProgress = 0;
  }

  // LLM Topology Inference
  async function triggerAIInference() {
    aiRunning = true;
    aiMessage = 'Connecting to LLM...';
    scanMessage = 'Idle';
    try {
      const data = await RunAIInference();
      await loadMapData();
      aiMessage = 'AI analysis completed successfully!';
    } catch (err) {
      aiMessage = `AI Error: ${err.message || err}`;
    } finally {
      aiRunning = false;
    }
  }

  // Save manual properties edit
  async function saveNodeEdit() {
    if (!selectedNode) return;
    try {
      selectedNode.label = editNodeLabel;
      selectedNode.type = editNodeType;
      await SaveNode(selectedNode);
      await loadMapData();
      showEditModal = false;
      selectedNode = null;
      showSuccess('Saved node changes.');
    } catch (err) {
      showError('Failed to save node changes: ' + err);
    }
  }

  // Delete manual node
  function deleteSelectedNode() {
    showDeleteConfirmModal = true;
  }

  async function confirmDeleteNode() {
    if (!selectedNode) return;
    showDeleteConfirmModal = false;
    try {
      await DeleteNode(selectedNode.id);
      await loadMapData();
      showEditModal = false;
      selectedNode = null;
      showSuccess('Node deleted.');
    } catch (err) {
      showError('Failed to delete node: ' + err);
    }
  }

  // Add custom manual node
  async function handleAddNode() {
    if (!addNodeIP.trim() || !addNodeLabel.trim()) {
      showError('IP address and label are required.');
      return;
    }
    try {
      const newNode = {
        id: addNodeIP.trim(),
        ip: addNodeIP.trim(),
        mac: '',
        vendor: 'Manual Node',
        label: addNodeLabel.trim(),
        type: addNodeType,
        reason: 'Manually added by user',
        sysName: '',
        sysDesc: '',
        x: 100,
        y: 100,
        manuallyEdited: true
      };
      await SaveNode(newNode);
      await loadMapData();
      showAddNodeModal = false;
      addNodeIP = '';
      addNodeLabel = '';
      addNodeType = 'unknown';
      showSuccess('Custom node added.');
    } catch (err) {
      showError('Failed to add custom node: ' + err);
    }
  }

  // Add custom manual link
  async function handleAddLink() {
    if (!addLinkFrom || !addLinkTo) {
      showError('Please select both nodes.');
      return;
    }
    if (addLinkFrom === addLinkTo) {
      showError('Cannot connect a node to itself.');
      return;
    }
    try {
      await AddLink(addLinkFrom, addLinkTo, addLinkType);
      await loadMapData();
      showAddLinkModal = false;
      addLinkFrom = '';
      addLinkTo = '';
      addLinkType = 'lan';
      showSuccess('Connection link added.');
    } catch (err) {
      showError('Failed to add connection link: ' + err);
    }
  }

  // Clear Map
  function handleClearMap() {
    showClearConfirmModal = true;
  }

  async function confirmClearMap() {
    showClearConfirmModal = false;
    try {
      await ClearMap();
      selectedNode = null;
      showEditModal = false;
      scanMessage = 'Idle';
      aiMessage = '';
      await loadMapData();
      showSuccess('Network map cleared.');
    } catch (err) {
      showError('Failed to clear map: ' + err);
    }
  }

  // Force physical network layout layout reset
  function resetPhysics() {
    if (network) {
      if (layoutMode === 'force') {
        network.physics.options.enabled = true;
        network.stabilize();
      } else {
        network.fit();
      }
    }
  }

  async function triggerRearrange(preserveManual) {
    showRearrangeModal = false;
    try {
      await RearrangeNodes(preserveManual);
      await loadMapData();
      showSuccess('Rearranged map nodes successfully.');
    } catch (err) {
      showError('Failed to rearrange map: ' + err);
    }
  }
</script>

<div class="flex flex-col h-full w-full bg-slate-950 relative">
  {#if errorMessage}
    <div class="absolute top-4 left-1/2 transform -translate-x-1/2 z-50 bg-rose-950/95 border border-rose-800 text-rose-200 px-4 py-2.5 rounded-xl shadow-xl flex items-center gap-2 text-xs backdrop-blur-md transition-all duration-200">
      <span>⚠️</span> {errorMessage}
    </div>
  {/if}
  {#if successMessage}
    <div class="absolute top-4 left-1/2 transform -translate-x-1/2 z-50 bg-slate-900/95 border border-indigo-900 text-slate-200 px-4 py-2.5 rounded-xl shadow-xl flex items-center gap-2 text-xs backdrop-blur-md transition-all duration-200">
      <span>✅</span> {successMessage}
    </div>
  {/if}
  <!-- Controls Dashboard Header -->
  <div class="flex flex-wrap items-center justify-between gap-4 p-4 bg-slate-900 border-b border-slate-800">
    <div class="flex items-center gap-4">
      <div class="text-left">
        <h1 class="text-lg font-bold bg-gradient-to-r from-sky-400 to-indigo-400 bg-clip-text text-transparent">twNetMap Dashboard</h1>
        <p class="text-xs text-slate-400">Target Range: <span class="text-sky-400 font-mono">{config.Subnet || 'Not Configured'}</span></p>
      </div>

      <!-- Quick Action Buttons -->
      <div class="flex gap-2 ml-4">
        {#if scanning}
          <button on:click={stopActiveScan} class="bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold px-3 py-2 rounded-lg transition duration-200">
            Cancel Scan
          </button>
        {:else}
          <button on:click={triggerScan} class="bg-sky-600 hover:bg-sky-500 text-white text-xs font-semibold px-3 py-2 rounded-lg transition duration-200 shadow-md shadow-sky-600/10">
            Active Scan
          </button>
        {/if}

        <button 
          on:click={triggerAIInference} 
          disabled={aiRunning || scanning}
          class="bg-indigo-600 hover:bg-indigo-500 disabled:bg-slate-700 text-white text-xs font-semibold px-3 py-2 rounded-lg transition duration-200 shadow-md shadow-indigo-600/10"
        >
          {aiRunning ? 'Inferring...' : 'Run AI Inference'}
        </button>

        <div class="border-l border-slate-800 mx-1"></div>

        <button on:click={() => showAddNodeModal = true} class="bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-medium px-3 py-2 rounded-lg border border-slate-700 transition duration-200">
          + Add Node
        </button>
        <button on:click={() => showAddLinkModal = true} class="bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-medium px-3 py-2 rounded-lg border border-slate-700 transition duration-200">
          + Connect Nodes
        </button>
        <button on:click={handleClearMap} class="bg-slate-800 hover:bg-rose-950 text-slate-400 hover:text-rose-400 text-xs font-medium px-3 py-2 rounded-lg border border-slate-700 hover:border-rose-900/50 transition duration-200">
          Clear Map
        </button>
        <button on:click={() => showRearrangeModal = true} class="bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-medium px-3 py-2 rounded-lg border border-slate-700 transition duration-200" title="Auto Layout (Rearrange)">
          Auto Layout
        </button>
        <button on:click={resetPhysics} class="bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-slate-200 text-xs font-medium px-2 py-2 rounded-lg border border-slate-700 transition duration-200" title="Recenter & Stabilize">
          🔄
        </button>
      </div>
    </div>

    <!-- Active Scanning & Inference Status Overlays -->
    <div class="flex items-center gap-4 text-xs">
      {#if scanning || (scanMessage && scanMessage !== 'Idle')}
        <div class="flex flex-col items-start w-64 md:w-80">
          <span class="text-sky-400 font-medium truncate max-w-full" title={scanMessage}>{scanMessage}</span>
          {#if scanning}
            <div class="w-full bg-slate-800 rounded-full h-1.5 mt-1 overflow-hidden">
              <div class="bg-sky-400 h-1.5 rounded-full transition-all duration-300" style="width: {scanProgress}%"></div>
            </div>
          {/if}
        </div>
      {/if}

      {#if aiRunning || aiMessage}
        <div class="text-left md:text-right">
          <span class={`font-medium ${aiRunning ? 'text-indigo-400 animate-pulse' : 'text-slate-400'}`} title={aiMessage}>
            {aiMessage}
          </span>
        </div>
      {/if}
    </div>
  </div>

  <!-- Interactive Vis-Network Canvas -->
  <div class="relative flex-grow min-h-0 w-full bg-slate-950">
    <div bind:this={container} class="w-full h-full"></div>
    
    <!-- Layout Settings Panel -->
    <div class="absolute top-4 left-4 p-2.5 bg-slate-900/95 border border-slate-800 rounded-lg text-xxs space-y-2 z-10 w-44 backdrop-blur-md shadow-2xl">
      <div class="flex items-center justify-between text-[10px] text-slate-300 font-bold border-b border-slate-800/60 pb-1 mb-1.5">
        <span class="flex items-center gap-1">⚙️ Layout</span>
      </div>
      
      <div class="space-y-1.5">
        <span class="block text-slate-500 font-semibold text-[9px] uppercase tracking-wider">Mode</span>
        <div class="grid grid-cols-3 gap-1">
          <button 
            on:click={() => changeLayoutMode('hierarchical')} 
            class="py-1 rounded-md border transition text-center text-xs {layoutMode === 'hierarchical' ? 'bg-sky-600 border-sky-500 text-white shadow-md shadow-sky-600/10' : 'bg-slate-800 border-slate-700 text-slate-400 hover:bg-slate-750'}"
            title="Hierarchical Layout (樹状構造)"
          >
            🌲
          </button>
          <button 
            on:click={() => changeLayoutMode('force')} 
            class="py-1 rounded-md border transition text-center text-xs {layoutMode === 'force' ? 'bg-sky-600 border-sky-500 text-white shadow-md shadow-sky-600/10' : 'bg-slate-800 border-slate-700 text-slate-400 hover:bg-slate-750'}"
            title="Force-Directed Layout (力学モデル)"
          >
            ⚛️
          </button>
          <button 
            on:click={() => changeLayoutMode('static')} 
            class="py-1 rounded-md border transition text-center text-xs {layoutMode === 'static' ? 'bg-sky-600 border-sky-500 text-white shadow-md shadow-sky-600/10' : 'bg-slate-800 border-slate-700 text-slate-400 hover:bg-slate-750'}"
            title="Static Layout (自由配置・固定)"
          >
            📌
          </button>
        </div>
      </div>
      
      {#if layoutMode !== 'static'}
        <div class="space-y-1 pt-0.5">
          <div class="flex justify-between text-slate-500 font-semibold text-[9px] uppercase tracking-wider">
            <span>Spacing</span>
            <span class="text-sky-400 font-mono">{nodeSpacing}px</span>
          </div>
          <input type="range" min="50" max="300" step="10" bind:value={nodeSpacing} on:input={updateSpacing} class="w-full accent-sky-500 bg-slate-800 rounded-lg appearance-none h-1 cursor-pointer" />
        </div>
      {/if}
    </div>
    
    <!-- Legend -->
    <div class="absolute bottom-4 left-4 p-3 bg-slate-900/90 border border-slate-850 rounded-xl text-xs space-y-2 pointer-events-none backdrop-blur-md">
      <h4 class="font-bold text-slate-300 border-b border-slate-800 pb-1 mb-2">Device Types</h4>
      <div class="grid grid-cols-2 gap-x-4 gap-y-1.5">
        <div class="flex items-center gap-2"><span class="w-2.5 h-2.5 rounded-full" style="background-color: #f59e0b"></span><span class="text-slate-400">Router</span></div>
        <div class="flex items-center gap-2"><span class="w-2.5 h-2.5 rounded-full" style="background-color: #3b82f6"></span><span class="text-slate-400">Switch</span></div>
        <div class="flex items-center gap-2"><span class="w-2.5 h-2.5 rounded-full" style="background-color: #06b6d4"></span><span class="text-slate-400">Wifi AP</span></div>
        <div class="flex items-center gap-2"><span class="w-2.5 h-2.5 rounded-full" style="background-color: #f43f5e"></span><span class="text-slate-400">Mobile</span></div>
        <div class="flex items-center gap-2"><span class="w-2.5 h-2.5 rounded-full" style="background-color: #10b981"></span><span class="text-slate-400">PC / Endpoint</span></div>
        <div class="flex items-center gap-2"><span class="w-2.5 h-2.5 rounded-full" style="background-color: #8b5cf6"></span><span class="text-slate-400">Server</span></div>
        <div class="flex items-center gap-2"><span class="w-2.5 h-2.5 rounded-full" style="background-color: #ec4899"></span><span class="text-slate-400">Printer</span></div>
        <div class="flex items-center gap-2"><span class="w-2.5 h-2.5 rounded-full" style="background-color: #94a3b8"></span><span class="text-slate-400">Unknown</span></div>
      </div>
      <div class="text-slate-500 text-xxs mt-2 border-t border-slate-800/80 pt-1">Double click node to edit/delete</div>
    </div>
  </div>

  <!-- MODAL: Edit Node -->
  {#if showEditModal && selectedNode}
    <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div class="bg-slate-800 border border-slate-700/80 rounded-2xl w-full max-w-md p-6 shadow-2xl relative">
        <button on:click={() => showEditModal = false} class="absolute top-4 right-4 text-slate-400 hover:text-slate-200">✕</button>
        
        <h3 class="text-lg font-bold bg-gradient-to-r from-sky-400 to-indigo-400 bg-clip-text text-transparent mb-4">Edit Network Node</h3>
        
        <div class="space-y-4">
          <div>
            <label for="nodeLabel" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">Display Label</label>
            <input type="text" id="nodeLabel" bind:value={editNodeLabel} class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-2 text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50" />
          </div>

          <div>
            <label for="nodeType" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">Device Type</label>
            <select id="nodeType" bind:value={editNodeType} class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-2 text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50">
              <option value="router">Router</option>
              <option value="switch">Switch</option>
              <option value="wifi">Wifi AP</option>
              <option value="mobile">Mobile Device</option>
              <option value="pc">PC / Endpoint</option>
              <option value="server">Server</option>
              <option value="printer">Printer</option>
              <option value="unknown">Unknown</option>
            </select>
          </div>

          <!-- Metadata table -->
          <div class="bg-slate-900/60 rounded-xl p-3 border border-slate-700/50 text-xs text-slate-400 space-y-1.5">
            <div><span class="text-slate-500">IP Address:</span> <span class="font-mono text-slate-300">{selectedNode.ip || 'N/A'}</span></div>
            <div><span class="text-slate-500">MAC Address:</span> <span class="font-mono text-slate-300">{selectedNode.mac || 'N/A'}</span></div>
            <div><span class="text-slate-500">OUI Vendor:</span> <span class="text-slate-300">{selectedNode.vendor || 'N/A'}</span></div>
            {#if selectedNode.sysName}
              <div><span class="text-slate-500">SNMP Name:</span> <span class="text-slate-300">{selectedNode.sysName}</span></div>
            {/if}
            {#if selectedNode.sysDesc}
              <div class="line-clamp-2"><span class="text-slate-500">SNMP Desc:</span> <span class="text-slate-300">{selectedNode.sysDesc}</span></div>
            {/if}
            {#if selectedNode.reason}
              <div class="border-t border-slate-800/80 pt-1 mt-1 font-sans italic text-slate-500"><span class="text-slate-400 font-medium">AI Reason:</span> {selectedNode.reason}</div>
            {/if}
          </div>
        </div>

        <div class="flex items-center justify-between mt-6">
          <button on:click={deleteSelectedNode} class="bg-rose-950/60 hover:bg-rose-900 border border-rose-800/60 text-rose-300 font-semibold px-4 py-2 rounded-xl transition duration-150">
            Delete Node
          </button>
          <div class="flex gap-2">
            <button on:click={() => showEditModal = false} class="bg-slate-700 hover:bg-slate-650 text-slate-200 px-4 py-2 rounded-xl">Cancel</button>
            <button on:click={saveNodeEdit} class="bg-sky-600 hover:bg-sky-500 text-white font-semibold px-4 py-2 rounded-xl shadow-lg shadow-sky-600/10">Save</button>
          </div>
        </div>
      </div>
    </div>
  {/if}

  <!-- MODAL: Add Node -->
  {#if showAddNodeModal}
    <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div class="bg-slate-800 border border-slate-700/80 rounded-2xl w-full max-w-md p-6 shadow-2xl relative">
        <button on:click={() => showAddNodeModal = false} class="absolute top-4 right-4 text-slate-400 hover:text-slate-200">✕</button>
        
        <h3 class="text-lg font-bold bg-gradient-to-r from-sky-400 to-indigo-400 bg-clip-text text-transparent mb-4">Add Custom Node</h3>
        
        <div class="space-y-4">
          <div>
            <label for="addNodeIP" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">IP Address / Unique ID</label>
            <input type="text" id="addNodeIP" bind:value={addNodeIP} placeholder="e.g. 192.168.1.15" class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-2 text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50" />
          </div>

          <div>
            <label for="addNodeLabel" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">Display Label</label>
            <input type="text" id="addNodeLabel" bind:value={addNodeLabel} placeholder="e.g. File Server" class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-2 text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50" />
          </div>

          <div>
            <label for="addNodeType" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">Device Type</label>
            <select id="addNodeType" bind:value={addNodeType} class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-2 text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50">
              <option value="router">Router</option>
              <option value="switch">Switch</option>
              <option value="wifi">Wifi AP</option>
              <option value="mobile">Mobile Device</option>
              <option value="pc">PC / Endpoint</option>
              <option value="server">Server</option>
              <option value="printer">Printer</option>
              <option value="unknown">Unknown</option>
            </select>
          </div>
        </div>

        <div class="flex justify-end gap-2 mt-6">
          <button on:click={() => showAddNodeModal = false} class="bg-slate-700 hover:bg-slate-650 text-slate-200 px-4 py-2 rounded-xl">Cancel</button>
          <button on:click={handleAddNode} class="bg-sky-600 hover:bg-sky-500 text-white font-semibold px-4 py-2 rounded-xl shadow-lg shadow-sky-600/10">Add Node</button>
        </div>
      </div>
    </div>
  {/if}

  <!-- MODAL: Connect Nodes -->
  {#if showAddLinkModal}
    <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div class="bg-slate-800 border border-slate-700/80 rounded-2xl w-full max-w-md p-6 shadow-2xl relative">
        <button on:click={() => showAddLinkModal = false} class="absolute top-4 right-4 text-slate-400 hover:text-slate-200">✕</button>
        
        <h3 class="text-lg font-bold bg-gradient-to-r from-sky-400 to-indigo-400 bg-clip-text text-transparent mb-4">Connect Nodes</h3>
        
        <div class="space-y-4">
          <div>
            <label for="linkFrom" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">Source Node</label>
            <select id="linkFrom" bind:value={addLinkFrom} class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-2 text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50">
              <option value="">-- Select Source Node --</option>
              {#each nodesDataSet.get() as n}
                <option value={n.id}>{n.label} ({n.id})</option>
              {/each}
            </select>
          </div>

          <div>
            <label for="linkTo" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">Target Node</label>
            <select id="linkTo" bind:value={addLinkTo} class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-2 text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50">
              <option value="">-- Select Target Node --</option>
              {#each nodesDataSet.get() as n}
                <option value={n.id}>{n.label} ({n.id})</option>
              {/each}
            </select>
          </div>

          <div>
            <label for="linkType" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">Connection Link Type</label>
            <input type="text" id="linkType" bind:value={addLinkType} placeholder="e.g. lan, fiber, wifi" class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-2 text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50" />
          </div>
        </div>

        <div class="flex justify-end gap-2 mt-6">
          <button on:click={() => showAddLinkModal = false} class="bg-slate-700 hover:bg-slate-650 text-slate-200 px-4 py-2 rounded-xl">Cancel</button>
          <button on:click={handleAddLink} class="bg-sky-600 hover:bg-sky-500 text-white font-semibold px-4 py-2 rounded-xl shadow-lg shadow-sky-600/10">Add Connection</button>
        </div>
      </div>
    </div>
  {/if}

  <!-- MODAL: Confirm Rearrange Map -->
  {#if showRearrangeModal}
    <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div class="bg-slate-800 border border-slate-700/80 rounded-2xl w-full max-w-md p-6 shadow-2xl relative">
        <button on:click={() => showRearrangeModal = false} class="absolute top-4 right-4 text-slate-400 hover:text-slate-200">✕</button>
        
        <h3 class="text-lg font-bold text-sky-400 mb-2">Rearrange Network Map</h3>
        <p class="text-slate-300 text-sm mb-6">Choose how you want to rearrange the nodes on the network map. If links exist (AI Inference topology), nodes will be tiered by device type. Otherwise, they will be arranged in a 10-column grid by IP address.</p>
        
        <div class="flex flex-col gap-2 mb-6">
          <button on:click={() => triggerRearrange(true)} class="bg-sky-600 hover:bg-sky-500 text-white font-semibold px-4 py-3 rounded-xl text-xs shadow-lg shadow-sky-600/10 transition duration-150 text-left flex justify-between items-center">
            <span>Keep Manual Positions (Only rearrange auto-placed nodes)</span>
            <span>➔</span>
          </button>
          <button on:click={() => triggerRearrange(false)} class="bg-indigo-600 hover:bg-indigo-500 text-white font-semibold px-4 py-3 rounded-xl text-xs shadow-lg shadow-indigo-600/10 transition duration-150 text-left flex justify-between items-center">
            <span>Reset All Positions (Rearrange every node)</span>
            <span>➔</span>
          </button>
        </div>
        
        <div class="flex justify-end">
          <button on:click={() => showRearrangeModal = false} class="bg-slate-700 hover:bg-slate-650 text-slate-200 px-4 py-2.5 rounded-xl text-xs font-semibold transition duration-150">Cancel</button>
        </div>
      </div>
    </div>
  {/if}

  <!-- MODAL: Confirm Clear Map -->
  {#if showClearConfirmModal}
    <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div class="bg-slate-800 border border-slate-700/80 rounded-2xl w-full max-w-md p-6 shadow-2xl relative">
        <button on:click={() => showClearConfirmModal = false} class="absolute top-4 right-4 text-slate-400 hover:text-slate-200">✕</button>
        
        <h3 class="text-lg font-bold text-rose-500 mb-2">Clear Network Map?</h3>
        <p class="text-slate-300 text-sm mb-6">Are you sure you want to clear the entire network map? This will delete all nodes and links. This action cannot be undone.</p>
        
        <div class="flex justify-end gap-3">
          <button on:click={() => showClearConfirmModal = false} class="bg-slate-700 hover:bg-slate-650 text-slate-200 px-4 py-2.5 rounded-xl text-xs font-semibold transition duration-150">Cancel</button>
          <button on:click={confirmClearMap} class="bg-rose-600 hover:bg-rose-500 text-white font-semibold px-4 py-2.5 rounded-xl text-xs shadow-lg shadow-rose-600/10 transition duration-150">Clear Map</button>
        </div>
      </div>
    </div>
  {/if}

  <!-- MODAL: Confirm Delete Node -->
  {#if showDeleteConfirmModal && selectedNode}
    <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div class="bg-slate-800 border border-slate-700/80 rounded-2xl w-full max-w-md p-6 shadow-2xl relative">
        <button on:click={() => showDeleteConfirmModal = false} class="absolute top-4 right-4 text-slate-400 hover:text-slate-200">✕</button>
        
        <h3 class="text-lg font-bold text-rose-500 mb-2">Delete Node?</h3>
        <p class="text-slate-300 text-sm mb-6">Are you sure you want to delete node <span class="font-semibold text-slate-100">{selectedNode.label}</span>? This action cannot be undone.</p>
        
        <div class="flex justify-end gap-3">
          <button on:click={() => showDeleteConfirmModal = false} class="bg-slate-700 hover:bg-slate-650 text-slate-200 px-4 py-2.5 rounded-xl text-xs font-semibold transition duration-150">Cancel</button>
          <button on:click={confirmDeleteNode} class="bg-rose-600 hover:bg-rose-500 text-white font-semibold px-4 py-2.5 rounded-xl text-xs shadow-lg shadow-rose-600/10 transition duration-150">Delete</button>
        </div>
      </div>
    </div>
  {/if}
</div>
