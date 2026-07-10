<script>
  import { onMount } from 'svelte';
  import { GetNetworkMap, SaveNode, DeleteNode } from '../../wailsjs/go/main/App';

  let nodes = [];
  let links = [];
  let searchQuery = '';
  let filterType = 'all';

  // Loading and alerts state
  let loading = false;
  let successMessage = '';
  let errorMessage = '';

  // Modals state
  let showAddModal = false;
  let showEditModal = false;
  let showDeleteConfirmModal = false;

  // Form states
  let nodeForm = {
    id: '',
    ip: '',
    mac: '',
    vendor: '',
    label: '',
    type: 'unknown',
    sysName: '',
    sysDesc: '',
    reason: 'Manually added',
    x: 100,
    y: 100,
    manuallyEdited: false
  };

  let nodeToDelete = null;

  async function loadData() {
    loading = true;
    try {
      const data = await GetNetworkMap();
      if (data) {
        nodes = data.nodes || [];
        links = data.links || [];
      }
    } catch (err) {
      showError('Failed to load nodes: ' + err.message || err);
    } finally {
      loading = false;
    }
  }

  onMount(loadData);

  // Auto reload when tab changes to refresh data
  export function refresh() {
    loadData();
  }

  function showError(msg) {
    errorMessage = msg;
    setTimeout(() => { errorMessage = ''; }, 5000);
  }

  function showSuccess(msg) {
    successMessage = msg;
    setTimeout(() => { successMessage = ''; }, 3000);
  }

  // Filter & Search logic
  $: filteredNodes = nodes.filter(n => {
    const s = searchQuery.toLowerCase().trim();
    const matchesSearch = !s ||
      (n.ip && n.ip.toLowerCase().includes(s)) ||
      (n.mac && n.mac.toLowerCase().includes(s)) ||
      (n.label && n.label.toLowerCase().includes(s)) ||
      (n.vendor && n.vendor.toLowerCase().includes(s)) ||
      (n.sysName && n.sysName.toLowerCase().includes(s)) ||
      (n.sysDesc && n.sysDesc.toLowerCase().includes(s)) ||
      (n.id && n.id.toLowerCase().includes(s));

    const matchesType = filterType === 'all' || n.type === filterType;

    return matchesSearch && matchesType;
  });

  // Sort logic
  let sortBy = 'ip'; // 'label' | 'ip' | 'mac' | 'vendor' | 'type' | 'sysName' | 'reason'
  let sortOrder = 'asc'; // 'asc' | 'desc'

  function toggleSort(column) {
    if (sortBy === column) {
      sortOrder = sortOrder === 'asc' ? 'desc' : 'asc';
    } else {
      sortBy = column;
      sortOrder = 'asc';
    }
  }

  function ipToNum(ip) {
    if (!ip) return 0;
    const parts = ip.split('.').map(Number);
    if (parts.length !== 4 || parts.some(isNaN)) return 0;
    return parts[0] * 16777216 + parts[1] * 65536 + parts[2] * 256 + parts[3];
  }

  $: sortedNodes = [...filteredNodes].sort((a, b) => {
    let valA = '';
    let valB = '';

    if (sortBy === 'ip') {
      const numA = ipToNum(a.ip);
      const numB = ipToNum(b.ip);
      return sortOrder === 'asc' ? numA - numB : numB - numA;
    }

    switch (sortBy) {
      case 'label':
        valA = a.label || a.ip || a.id || '';
        valB = b.label || b.ip || b.id || '';
        break;
      case 'mac':
        valA = a.mac || '';
        valB = b.mac || '';
        break;
      case 'vendor':
        valA = a.vendor || '';
        valB = b.vendor || '';
        break;
      case 'type':
        valA = a.type || '';
        valB = b.type || '';
        break;
      case 'sysName':
        valA = a.sysName || a.sysDesc || '';
        valB = b.sysName || b.sysDesc || '';
        break;
      case 'reason':
        valA = a.reason || '';
        valB = b.reason || '';
        break;
      default:
        valA = a.id || '';
        valB = b.id || '';
    }

    valA = valA.toLowerCase();
    valB = valB.toLowerCase();

    if (valA < valB) return sortOrder === 'asc' ? -1 : 1;
    if (valA > valB) return sortOrder === 'asc' ? 1 : -1;
    return 0;
  });

  // Modal helpers
  function openAddModal() {
    nodeForm = {
      id: '',
      ip: '',
      mac: '',
      vendor: '',
      label: '',
      type: 'unknown',
      sysName: '',
      sysDesc: '',
      reason: 'Manually added',
      // Random offset to avoid completely overlapping nodes on the map
      x: 100 + (nodes.length % 5) * 60,
      y: 100 + Math.floor(nodes.length / 5) * 60,
      manuallyEdited: true
    };
    showAddModal = true;
  }

  function openEditModal(node) {
    nodeForm = { ...node };
    showEditModal = true;
  }

  function openDeleteConfirm(node) {
    nodeToDelete = node;
    showDeleteConfirmModal = true;
  }

  async function handleAddNode() {
    try {
      if (!nodeForm.ip.trim()) {
        throw new Error('IP Address is required');
      }

      // ID calculation (MAC if present, else IP)
      const id = nodeForm.mac.trim() || nodeForm.ip.trim();
      
      // Check if duplicate
      if (nodes.some(n => n.id === id)) {
        throw new Error(`A node with ID/IP/MAC '${id}' already exists.`);
      }

      const nodeToSave = {
        ...nodeForm,
        id: id,
        label: nodeForm.label.trim() || nodeForm.ip.trim(),
        manuallyEdited: true
      };

      await SaveNode(nodeToSave);
      showAddModal = false;
      showSuccess('Node added successfully!');
      await loadData();
    } catch (err) {
      showError(err.message || 'Failed to add node');
    }
  }

  async function handleEditNode() {
    try {
      if (!nodeForm.label.trim()) {
        nodeForm.label = nodeForm.ip || nodeForm.id;
      }
      await SaveNode({
        ...nodeForm,
        manuallyEdited: true
      });
      showEditModal = false;
      showSuccess('Node updated successfully!');
      await loadData();
    } catch (err) {
      showError(err.message || 'Failed to update node');
    }
  }

  async function handleDeleteNode() {
    if (!nodeToDelete) return;
    try {
      await DeleteNode(nodeToDelete.id);
      showDeleteConfirmModal = false;
      showSuccess('Node deleted successfully!');
      nodeToDelete = null;
      await loadData();
    } catch (err) {
      showError(err.message || 'Failed to delete node');
    }
  }

  function getBadgeColor(type) {
    switch (type) {
      case 'router': return 'bg-amber-500/10 text-amber-400 border border-amber-500/20';
      case 'switch': return 'bg-blue-500/10 text-blue-400 border border-blue-500/20';
      case 'pc': return 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20';
      case 'server': return 'bg-purple-500/10 text-purple-400 border border-purple-500/20';
      case 'printer': return 'bg-pink-500/10 text-pink-400 border border-pink-500/20';
      default: return 'bg-slate-500/10 text-slate-400 border border-slate-500/20';
    }
  }

  function formatType(type) {
    switch (type) {
      case 'router': return 'Router';
      case 'switch': return 'Switch';
      case 'pc': return 'PC / Endpoint';
      case 'server': return 'Server';
      case 'printer': return 'Printer';
      default: return 'Unknown';
    }
  }
</script>

<div class="relative w-full h-full">
  {#if errorMessage}
    <div class="fixed top-4 left-1/2 transform -translate-x-1/2 z-50 bg-rose-950/95 border border-rose-800 text-rose-200 px-4 py-2.5 rounded-xl shadow-xl flex items-center gap-2 text-xs backdrop-blur-md transition-all duration-200 animate-slide-in">
      <span>⚠️</span> {errorMessage}
    </div>
  {/if}
  {#if successMessage}
    <div class="fixed top-4 left-1/2 transform -translate-x-1/2 z-50 bg-slate-900/95 border border-indigo-900 text-slate-200 px-4 py-2.5 rounded-xl shadow-xl flex items-center gap-2 text-xs backdrop-blur-md transition-all duration-200 animate-slide-in">
      <span>✅</span> {successMessage}
    </div>
  {/if}

  <div class="max-w-7xl mx-auto">
    <!-- Header Block -->
    <div class="mb-6 flex justify-between items-center">
      <div>
        <h2 class="text-2xl font-bold bg-gradient-to-r from-sky-400 to-indigo-400 bg-clip-text text-transparent">Node List</h2>
        <p class="text-xs text-slate-400 mt-1">Manage, search, and edit scanned network devices and manual endpoints.</p>
      </div>
      <button 
        on:click={openAddModal} 
        class="bg-sky-600 hover:bg-sky-500 text-white text-xs font-semibold px-4 py-2.5 rounded-xl transition duration-200 shadow-md shadow-sky-600/10 flex items-center gap-1.5"
      >
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" class="w-4 h-4">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
        </svg>
        Add Node
      </button>
    </div>

    <!-- Filter and Search controls -->
    <div class="flex flex-col sm:flex-row gap-3 mb-6">
      <div class="relative flex-grow">
        <span class="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-slate-500">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" class="w-4 h-4">
            <path stroke-linecap="round" stroke-linejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.637 10.637Z" />
          </svg>
        </span>
        <input
          type="text"
          bind:value={searchQuery}
          placeholder="Search by IP, MAC, Label, Vendor, or SNMP properties..."
          class="w-full bg-slate-900 border border-slate-800 rounded-xl pl-10 pr-4 py-2.5 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-sky-500/30 focus:border-sky-500 transition duration-200"
        />
      </div>

      <div class="w-full sm:w-48">
        <select
          bind:value={filterType}
          class="w-full bg-slate-900 border border-slate-800 rounded-xl px-3 py-2.5 text-xs text-slate-300 focus:outline-none focus:ring-2 focus:ring-sky-500/30 focus:border-sky-500 transition duration-200"
        >
          <option value="all">All Devices</option>
          <option value="router">Routers</option>
          <option value="switch">Switches</option>
          <option value="pc">PCs / Endpoints</option>
          <option value="server">Servers</option>
          <option value="printer">Printers</option>
          <option value="unknown">Unknown</option>
        </select>
      </div>
    </div>

    <!-- Table content -->
    {#if loading}
      <div class="flex flex-col items-center justify-center py-20 text-slate-500 gap-3">
        <div class="w-8 h-8 rounded-full border-2 border-sky-500 border-t-transparent animate-spin"></div>
        <p class="text-xs">Loading node data...</p>
      </div>
    {:else if filteredNodes.length === 0}
      <div class="flex flex-col items-center justify-center py-16 bg-slate-900/40 rounded-2xl border border-slate-800/80 text-center px-4">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="w-10 h-10 text-slate-600 mb-3">
          <path stroke-linecap="round" stroke-linejoin="round" d="m9.75 9.75 4.5 4.5m0-4.5-4.5 4.5M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
        </svg>
        <h3 class="text-sm font-bold text-slate-400">No nodes found</h3>
        <p class="text-xxs text-slate-500 mt-1 max-w-sm">No device matches the search query or no scan results exist yet. Run an Active Scan or add nodes manually.</p>
      </div>
    {:else}
      <div class="w-full bg-slate-900 border border-slate-800 rounded-xl overflow-hidden shadow-xl">
        <div class="overflow-x-auto">
          <table class="w-full text-left border-collapse">
            <thead class="bg-slate-800/40 border-b border-slate-800 text-slate-400 font-semibold text-xxs tracking-wider uppercase select-none">
              <tr>
                <th class="px-4 py-3 cursor-pointer hover:text-slate-200 transition duration-150" on:click={() => toggleSort('label')}>
                  <div class="flex items-center gap-1">
                    Label
                    {#if sortBy === 'label'}
                      <span class="text-sky-400 text-[10px]">{sortOrder === 'asc' ? '▲' : '▼'}</span>
                    {/if}
                  </div>
                </th>
                <th class="px-4 py-3 cursor-pointer hover:text-slate-200 transition duration-150" on:click={() => toggleSort('ip')}>
                  <div class="flex items-center gap-1">
                    IP Address
                    {#if sortBy === 'ip'}
                      <span class="text-sky-400 text-[10px]">{sortOrder === 'asc' ? '▲' : '▼'}</span>
                    {/if}
                  </div>
                </th>
                <th class="px-4 py-3 cursor-pointer hover:text-slate-200 transition duration-150" on:click={() => toggleSort('mac')}>
                  <div class="flex items-center gap-1">
                    MAC Address
                    {#if sortBy === 'mac'}
                      <span class="text-sky-400 text-[10px]">{sortOrder === 'asc' ? '▲' : '▼'}</span>
                    {/if}
                  </div>
                </th>
                <th class="px-4 py-3 cursor-pointer hover:text-slate-200 transition duration-150" on:click={() => toggleSort('vendor')}>
                  <div class="flex items-center gap-1">
                    Vendor
                    {#if sortBy === 'vendor'}
                      <span class="text-sky-400 text-[10px]">{sortOrder === 'asc' ? '▲' : '▼'}</span>
                    {/if}
                  </div>
                </th>
                <th class="px-4 py-3 cursor-pointer hover:text-slate-200 transition duration-150" on:click={() => toggleSort('type')}>
                  <div class="flex items-center gap-1">
                    Device Type
                    {#if sortBy === 'type'}
                      <span class="text-sky-400 text-[10px]">{sortOrder === 'asc' ? '▲' : '▼'}</span>
                    {/if}
                  </div>
                </th>
                <th class="px-4 py-3 cursor-pointer hover:text-slate-200 transition duration-150" on:click={() => toggleSort('sysName')}>
                  <div class="flex items-center gap-1">
                    SNMP Name / Info
                    {#if sortBy === 'sysName'}
                      <span class="text-sky-400 text-[10px]">{sortOrder === 'asc' ? '▲' : '▼'}</span>
                    {/if}
                  </div>
                </th>
                <th class="px-4 py-3 cursor-pointer hover:text-slate-200 transition duration-150" on:click={() => toggleSort('reason')}>
                  <div class="flex items-center gap-1">
                    Discovery / Source
                    {#if sortBy === 'reason'}
                      <span class="text-sky-400 text-[10px]">{sortOrder === 'asc' ? '▲' : '▼'}</span>
                    {/if}
                  </div>
                </th>
                <th class="px-4 py-3 text-right">Actions</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-800/50 text-xs text-slate-300">
              {#each sortedNodes as node (node.id)}
                <tr class="hover:bg-slate-800/30 transition duration-150">
                  <!-- Label -->
                  <td class="px-4 py-3 font-semibold text-slate-200 font-sans">
                    {node.label || node.ip || node.id}
                  </td>
                  <!-- IP -->
                  <td class="px-4 py-3 font-mono text-slate-400 text-xxs">
                    {node.ip || '—'}
                  </td>
                  <!-- MAC -->
                  <td class="px-4 py-3 font-mono text-slate-400 text-xxs">
                    {node.mac || '—'}
                  </td>
                  <!-- Vendor -->
                  <td class="px-4 py-3 text-slate-400 max-w-[120px] truncate" title={node.vendor || ''}>
                    {node.vendor || '—'}
                  </td>
                  <!-- Type -->
                  <td class="px-4 py-3">
                    <span class={`inline-flex px-2 py-0.5 rounded-full text-xxs font-semibold ${getBadgeColor(node.type)}`}>
                      {formatType(node.type)}
                    </span>
                  </td>
                  <!-- SNMP Info -->
                  <td class="px-4 py-3 text-slate-400 max-w-[200px] truncate" title={node.sysDesc || ''}>
                    {#if node.sysName}
                      <span class="text-slate-200 font-medium">{node.sysName}</span>
                      {#if node.sysDesc}
                        <span class="text-slate-500 text-xxs font-normal"> - {node.sysDesc}</span>
                      {/if}
                    {:else if node.sysDesc}
                      <span class="text-xxs">{node.sysDesc}</span>
                    {:else}
                      <span class="text-slate-600 italic">No SNMP info</span>
                    {/if}
                  </td>
                  <!-- Reason -->
                  <td class="px-4 py-3 text-slate-500 text-xxs truncate max-w-[150px]" title={node.reason || ''}>
                    {node.reason || '—'}
                  </td>
                  <!-- Actions -->
                  <td class="px-4 py-3 text-right">
                    <div class="flex justify-end gap-1.5">
                      <button 
                        on:click={() => openEditModal(node)} 
                        class="p-1.5 rounded-lg bg-slate-800 hover:bg-sky-600/20 hover:text-sky-400 border border-slate-700 transition duration-150" 
                        title="Edit Node"
                      >
                        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" class="w-3.5 h-3.5">
                          <path stroke-linecap="round" stroke-linejoin="round" d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L6.83 18.5a4.5 4.5 0 0 1-1.897 1.13l-2.685.8.8-2.685a4.5 4.5 0 0 1 1.13-1.897L16.863 4.487Zm0 0L19.5 7.125" />
                        </svg>
                      </button>
                      <button 
                        on:click={() => openDeleteConfirm(node)} 
                        class="p-1.5 rounded-lg bg-slate-800 hover:bg-rose-600/20 hover:text-rose-400 border border-slate-700 transition duration-150" 
                        title="Delete Node"
                      >
                        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" class="w-3.5 h-3.5">
                          <path stroke-linecap="round" stroke-linejoin="round" d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" />
                        </svg>
                      </button>
                    </div>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
        <div class="px-4 py-3 bg-slate-800/10 border-t border-slate-800 text-xxs text-slate-500 flex justify-between select-none">
          <span>Showing {filteredNodes.length} of {nodes.length} nodes</span>
          {#if searchQuery || filterType !== 'all'}
            <button on:click={() => { searchQuery = ''; filterType = 'all'; }} class="text-sky-500 hover:text-sky-400">Clear filters</button>
          {/if}
        </div>
      </div>
    {/if}
  </div>

  <!-- MODAL: Add Node -->
  {#if showAddModal}
    <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
      <div class="bg-slate-800 border border-slate-700/80 rounded-2xl w-full max-w-md p-6 shadow-2xl relative animate-scale-up">
        <button on:click={() => showAddModal = false} class="absolute top-4 right-4 text-slate-400 hover:text-slate-200">✕</button>
        
        <h3 class="text-lg font-bold bg-gradient-to-r from-sky-400 to-indigo-400 bg-clip-text text-transparent mb-4 font-sans">Add Custom Node</h3>
        
        <form on:submit|preventDefault={handleAddNode} class="space-y-4">
          <div>
            <label for="addNodeIP" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">IP Address <span class="text-rose-500">*</span></label>
            <input 
              type="text" 
              id="addNodeIP" 
              bind:value={nodeForm.ip} 
              placeholder="e.g. 192.168.1.15" 
              required
              class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-2 text-xs text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50" 
            />
          </div>

          <div>
            <label for="addNodeMAC" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">MAC Address</label>
            <input 
              type="text" 
              id="addNodeMAC" 
              bind:value={nodeForm.mac} 
              placeholder="e.g. 00:11:22:33:44:55" 
              class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-2 text-xs text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50" 
            />
          </div>

          <div>
            <label for="addNodeLabel" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">Display Label</label>
            <input 
              type="text" 
              id="addNodeLabel" 
              bind:value={nodeForm.label} 
              placeholder="Defaults to IP Address" 
              class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-2 text-xs text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50" 
            />
          </div>

          <div>
            <label for="addNodeType" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">Device Type</label>
            <select 
              id="addNodeType" 
              bind:value={nodeForm.type} 
              class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-2 text-xs text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50"
            >
              <option value="router">Router</option>
              <option value="switch">Switch</option>
              <option value="pc">PC / Endpoint</option>
              <option value="server">Server</option>
              <option value="printer">Printer</option>
              <option value="unknown">Unknown</option>
            </select>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="addNodeSysName" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">SNMP SysName</label>
              <input 
                type="text" 
                id="addNodeSysName" 
                bind:value={nodeForm.sysName} 
                placeholder="e.g. gateway" 
                class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-2 text-xs text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50" 
              />
            </div>
            <div>
              <label for="addNodeVendor" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">OUI Vendor</label>
              <input 
                type="text" 
                id="addNodeVendor" 
                bind:value={nodeForm.vendor} 
                placeholder="e.g. Cisco Systems" 
                class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-2 text-xs text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50" 
              />
            </div>
          </div>

          <div>
            <label for="addNodeSysDesc" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">SNMP SysDesc</label>
            <textarea 
              id="addNodeSysDesc" 
              bind:value={nodeForm.sysDesc} 
              placeholder="Detailed hardware / description info..." 
              rows="2"
              class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-2 text-xs text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50 resize-none" 
            ></textarea>
          </div>

          <div class="flex justify-end gap-2 mt-6">
            <button 
              type="button" 
              on:click={() => showAddModal = false} 
              class="bg-slate-700 hover:bg-slate-650 text-slate-200 font-semibold px-4 py-2 rounded-xl text-xs transition duration-150"
            >
              Cancel
            </button>
            <button 
              type="submit" 
              class="bg-sky-600 hover:bg-sky-500 text-white font-semibold px-4 py-2 rounded-xl text-xs shadow-lg shadow-sky-600/10 transition duration-150"
            >
              Add Node
            </button>
          </div>
        </form>
      </div>
    </div>
  {/if}

  <!-- MODAL: Edit Node -->
  {#if showEditModal}
    <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
      <div class="bg-slate-800 border border-slate-700/80 rounded-2xl w-full max-w-md p-6 shadow-2xl relative animate-scale-up">
        <button on:click={() => showEditModal = false} class="absolute top-4 right-4 text-slate-400 hover:text-slate-200">✕</button>
        
        <h3 class="text-lg font-bold bg-gradient-to-r from-sky-400 to-indigo-400 bg-clip-text text-transparent mb-4 font-sans">Edit Network Node</h3>
        
        <form on:submit|preventDefault={handleEditNode} class="space-y-4">
          <div>
            <label for="editNodeLabel" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">Display Label <span class="text-rose-500">*</span></label>
            <input 
              type="text" 
              id="editNodeLabel" 
              bind:value={nodeForm.label} 
              required
              class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-2 text-xs text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50" 
            />
          </div>

          <div>
            <label for="editNodeType" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">Device Type</label>
            <select 
              id="editNodeType" 
              bind:value={nodeForm.type} 
              class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-2 text-xs text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50"
            >
              <option value="router">Router</option>
              <option value="switch">Switch</option>
              <option value="pc">PC / Endpoint</option>
              <option value="server">Server</option>
              <option value="printer">Printer</option>
              <option value="unknown">Unknown</option>
            </select>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="editNodeSysName" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">SNMP SysName</label>
              <input 
                type="text" 
                id="editNodeSysName" 
                bind:value={nodeForm.sysName} 
                class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-2 text-xs text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50" 
              />
            </div>
            <div>
              <label for="editNodeVendor" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">OUI Vendor</label>
              <input 
                type="text" 
                id="editNodeVendor" 
                bind:value={nodeForm.vendor} 
                class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-2 text-xs text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50" 
              />
            </div>
          </div>

          <div>
            <label for="editNodeSysDesc" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">SNMP SysDesc</label>
            <textarea 
              id="editNodeSysDesc" 
              bind:value={nodeForm.sysDesc} 
              rows="2"
              class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-2 text-xs text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50 resize-none" 
            ></textarea>
          </div>

          <!-- Metadata display -->
          <div class="bg-slate-900/60 rounded-xl p-3 border border-slate-700/50 text-xxs text-slate-400 space-y-1">
            <div><span class="text-slate-500 font-sans">Node ID:</span> <span class="font-mono text-slate-300">{nodeForm.id}</span></div>
            <div><span class="text-slate-500 font-sans">IP Address:</span> <span class="font-mono text-slate-300">{nodeForm.ip || '—'}</span></div>
            <div><span class="text-slate-500 font-sans">MAC Address:</span> <span class="font-mono text-slate-300">{nodeForm.mac || '—'}</span></div>
            {#if nodeForm.reason}
              <div class="border-t border-slate-800/80 pt-1 mt-1 font-sans italic text-slate-500"><span class="text-slate-400 font-semibold font-sans">Source / Reason:</span> {nodeForm.reason}</div>
            {/if}
          </div>

          <div class="flex justify-end gap-2 mt-6">
            <button 
              type="button" 
              on:click={() => showEditModal = false} 
              class="bg-slate-700 hover:bg-slate-650 text-slate-200 font-semibold px-4 py-2 rounded-xl text-xs transition duration-150"
            >
              Cancel
            </button>
            <button 
              type="submit" 
              class="bg-sky-600 hover:bg-sky-500 text-white font-semibold px-4 py-2 rounded-xl text-xs shadow-lg shadow-sky-600/10 transition duration-150"
            >
              Save Changes
            </button>
          </div>
        </form>
      </div>
    </div>
  {/if}

  <!-- MODAL: Delete Confirm -->
  {#if showDeleteConfirmModal && nodeToDelete}
    <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
      <div class="bg-slate-800 border border-slate-700/80 rounded-2xl w-full max-w-sm p-6 shadow-2xl relative animate-scale-up">
        <h3 class="text-md font-bold text-slate-100 mb-2 font-sans">Delete Network Node?</h3>
        <p class="text-xs text-slate-400 mb-5">
          Are you sure you want to delete <span class="text-sky-400 font-semibold">{nodeToDelete.label || nodeToDelete.ip || nodeToDelete.id}</span>? 
          This will also remove any network connections (links) connected to this node. This action cannot be undone.
        </p>

        <div class="flex justify-end gap-2">
          <button 
            on:click={() => { showDeleteConfirmModal = false; nodeToDelete = null; }} 
            class="bg-slate-700 hover:bg-slate-650 text-slate-200 font-semibold px-4 py-2 rounded-xl text-xs transition duration-150"
          >
            Cancel
          </button>
          <button 
            on:click={handleDeleteNode} 
            class="bg-rose-600 hover:bg-rose-500 text-white font-semibold px-4 py-2 rounded-xl text-xs shadow-lg shadow-rose-600/10 transition duration-150"
          >
            Delete Node
          </button>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  /* Extra transitions and scaling animations */
  .animate-scale-up {
    animation: scaleUp 0.18s cubic-bezier(0.34, 1.56, 0.64, 1) forwards;
  }

  .animate-slide-in {
    animation: slideIn 0.22s cubic-bezier(0.16, 1, 0.3, 1) forwards;
  }

  @keyframes scaleUp {
    from {
      opacity: 0;
      transform: scale(0.95);
    }
    to {
      opacity: 1;
      transform: scale(1);
    }
  }

  @keyframes slideIn {
    from {
      opacity: 0;
      transform: translate(-50%, -10px);
    }
    to {
      opacity: 1;
      transform: translate(-50%, 0);
    }
  }
</style>
