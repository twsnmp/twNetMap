<script>
  import { onMount } from 'svelte';
  import { GetScanResultsJSON } from '../../wailsjs/go/main/App';

  // Props
  export let show = false;
  export let node = null;
  export let onClose = () => {};

  let loading = false;
  let scanData = null;
  let jsonString = '';
  let copied = false;
  let errorMsg = '';

  $: if (show && node) {
    loadScanData();
  }

  async function loadScanData() {
    loading = true;
    errorMsg = '';
    scanData = null;
    jsonString = '';
    copied = false;

    try {
      const results = await GetScanResultsJSON();
      if (results && results.length > 0) {
        // Find matching scan result by IP or MAC
        const match = results.find(r => 
          (node.ip && r.ip === node.ip) || 
          (node.mac && r.mac === node.mac) ||
          r.ip === node.id || 
          r.mac === node.id
        );
        
        if (match) {
          scanData = match;
          jsonString = JSON.stringify(match, null, 2);
        } else {
          errorMsg = 'No scan data found for this node. (It may be manually added or not scanned yet.)';
        }
      } else {
        errorMsg = 'No scan results available. Please run a network scan first.';
      }
    } catch (err) {
      errorMsg = 'Failed to load data: ' + (err.message || err);
    } finally {
      loading = false;
    }
  }

  function handleCopy() {
    if (!jsonString) return;
    navigator.clipboard.writeText(jsonString).then(() => {
      copied = true;
      setTimeout(() => { copied = false; }, 2000);
    });
  }
</script>

{#if show}
  <div class="fixed inset-0 z-[100] flex items-center justify-center bg-black/75 backdrop-blur-sm p-4">
    <div class="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-3xl max-h-[85vh] flex flex-col shadow-2xl relative text-slate-200">
      
      <!-- Header -->
      <div class="px-6 py-4 border-b border-slate-800 flex justify-between items-center bg-slate-950/40 rounded-t-2xl">
        <div>
          <h3 class="text-md font-bold bg-gradient-to-r from-sky-400 to-indigo-400 bg-clip-text text-transparent flex items-center gap-2 font-sans">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" class="w-5 h-5 text-sky-400">
              <path stroke-linecap="round" stroke-linejoin="round" d="M17.25 6.75 22.5 12l-5.25 5.25m-10.5 0L1.5 12l5.25-5.25m7.5-3-4.5 16.5" />
            </svg>
            Scan Data Details (LLM Payload)
          </h3>
          <p class="text-xxs text-slate-400 mt-0.5 font-sans">
            {#if node}
              Target Device: <span class="text-slate-200 font-semibold">{node.label || node.ip || node.id}</span>
              {#if node.ip} ({node.ip}){/if}
            {/if}
          </p>
        </div>
        <button on:click={onClose} class="text-slate-400 hover:text-slate-200 text-lg">✕</button>
      </div>

      <!-- Content -->
      <div class="flex-grow overflow-y-auto p-6 space-y-6">
        
        <!-- Explanation Info -->
        <div class="bg-slate-950/50 border border-slate-800 rounded-xl p-4 text-xs text-slate-300 space-y-2 font-sans">
          <p class="font-semibold text-sky-400">💡 Reference Info for Classification</p>
          <p class="leading-relaxed text-slate-400">
            This data represents the raw scan payload sent to the LLM to infer the device types and network connections.
            Refer to the detected features below (open ports, banners, SNMP, etc.) to determine the correct device type.
          </p>
        </div>

        {#if loading}
          <div class="flex flex-col items-center justify-center py-20 text-slate-500 gap-3 font-sans">
            <div class="w-8 h-8 rounded-full border-2 border-sky-500 border-t-transparent animate-spin"></div>
            <p class="text-xs">Loading scan data...</p>
          </div>
        {:else if errorMsg}
          <div class="flex flex-col items-center justify-center py-12 text-center text-slate-400 bg-slate-950/30 rounded-xl border border-slate-800/80 p-6 font-sans">
            <span class="text-2xl mb-2">⚠️</span>
            <p class="text-xs max-w-md">{errorMsg}</p>
          </div>
        {:else if scanData}
          <!-- Structured Information Summary Cards -->
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <!-- Left Info Card: Hardware & SNMP -->
            <div class="bg-slate-950/30 border border-slate-850 rounded-xl p-4 space-y-3">
              <h4 class="text-xs font-bold text-slate-400 uppercase tracking-wider border-b border-slate-800/60 pb-1.5 flex items-center gap-1.5 font-sans">
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" class="w-3.5 h-3.5 text-slate-500">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.324.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 0 1 1.37.49l1.296 2.247a1.125 1.125 0 0 1-.26 1.43l-1.003.828c-.293.241-.438.613-.43.992a7.723 7.723 0 0 1 0 .255c-.008.378.137.75.43.991l1.004.827c.424.35.534.954.26 1.43l-1.298 2.247a1.125 1.125 0 0 1-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.47 6.47 0 0 1-.22.128c-.331.183-.581.495-.644.869l-.213 1.281c-.09.543-.56.94-1.11.94h-2.594c-.55 0-1.019-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 0 1-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 0 1-1.369-.49l-1.297-2.247a1.125 1.125 0 0 1 .26-1.43l1.004-.827c.292-.24.437-.613.43-.991a6.936 6.936 0 0 1 0-.255c.007-.38-.138-.751-.43-.992l-1.004-.827a1.125 1.125 0 0 1-.26-1.43l1.297-2.247a1.125 1.125 0 0 1 1.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.086.22-.128.332-.183.582-.495.644-.869l.214-1.28Z" />
                  <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
                </svg>
                Basic Info & SNMP
              </h4>
              <div class="text-xs space-y-2 font-sans text-slate-300">
                <div class="flex justify-between"><span class="text-slate-500">IP Address:</span> <span class="font-mono">{scanData.ip || '—'}</span></div>
                <div class="flex justify-between"><span class="text-slate-500">MAC Address:</span> <span class="font-mono">{scanData.mac || '—'}</span></div>
                <div class="flex justify-between"><span class="text-slate-500">OUI Vendor:</span> <span class="text-slate-200">{scanData.vendor || '—'}</span></div>
                <div class="flex justify-between"><span class="text-slate-500">SysName:</span> <span class="text-sky-400 font-semibold">{scanData.sysName || '—'}</span></div>
                <div class="flex flex-col gap-1 mt-1">
                  <span class="text-slate-500">SysDesc (SNMP Description):</span>
                  <div class="bg-slate-950/40 border border-slate-800 rounded p-2 text-xxs font-mono text-slate-400 max-h-24 overflow-y-auto leading-relaxed">
                    {scanData.sysDesc || 'No SNMP response or SysDesc available.'}
                  </div>
                </div>
              </div>
            </div>

            <!-- Right Info Card: Ports & Connections -->
            <div class="bg-slate-950/30 border border-slate-850 rounded-xl p-4 space-y-3">
              <h4 class="text-xs font-bold text-slate-400 uppercase tracking-wider border-b border-slate-800/60 pb-1.5 flex items-center gap-1.5 font-sans">
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" class="w-3.5 h-3.5 text-slate-500">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M12 21a9.004 9.004 0 0 0 8.716-6.747M12 21a9.004 9.004 0 0 1-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9s2.015-9 4.5-9m0 0a9.003 9.003 0 0 1 8.716 5.253M12 3a9.003 9.003 0 0 0-8.716 5.253m0 0A9.003 9.003 0 0 1 12 12" />
                </svg>
                Open Ports & LLDP
              </h4>
              <div class="text-xs space-y-3 font-sans text-slate-300">
                <div>
                  <span class="text-slate-500 block mb-1">Open Ports (TCP):</span>
                  {#if scanData.openPorts && scanData.openPorts.length > 0}
                    <div class="flex flex-wrap gap-1">
                      {#each scanData.openPorts as port}
                        <span class="bg-slate-800 text-slate-200 border border-slate-700/60 rounded px-1.5 py-0.5 text-xxs font-mono">
                          {port}
                        </span>
                      {/each}
                    </div>
                  {:else}
                    <span class="text-slate-600 italic">No open ports detected.</span>
                  {/if}
                </div>

                <div>
                  <span class="text-slate-500 block mb-1">LLDP Neighbors:</span>
                  {#if scanData.lldpNeighbors && scanData.lldpNeighbors.length > 0}
                    <div class="space-y-1.5">
                      {#each scanData.lldpNeighbors as nb}
                        <div class="bg-slate-950/40 border border-slate-800/80 rounded p-2 text-xxs text-slate-400">
                          <div class="flex justify-between font-semibold text-slate-300">
                            <span>SysName: {nb.sysName || '—'}</span>
                            <span class="font-mono text-sky-400">{nb.ip || '—'}</span>
                          </div>
                          {#if nb.portId}<div>Port: <span class="font-mono">{nb.portId}</span></div>{/if}
                          {#if nb.chassisId}<div>Chassis ID: <span class="font-mono">{nb.chassisId}</span></div>{/if}
                        </div>
                      {/each}
                    </div>
                  {:else}
                    <span class="text-slate-600 italic">No LLDP neighbors found.</span>
                  {/if}
                </div>
              </div>
            </div>
          </div>

          <!-- Banners section -->
          {#if scanData.banners && Object.keys(scanData.banners).length > 0}
            <div class="bg-slate-950/20 border border-slate-850 rounded-xl p-4 space-y-2">
              <h4 class="text-xs font-bold text-slate-400 uppercase tracking-wider border-b border-slate-800/60 pb-1.5 flex items-center gap-1.5 font-sans">
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" class="w-3.5 h-3.5 text-slate-500">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M7.5 8.25h9m-9 3H12m-9.75 1.51c0 1.6 1.123 2.994 2.707 3.227 1.129.166 2.27.293 3.423.379L12 21l3.62-3.113c1.153-.086 2.294-.213 3.423-.379 1.584-.233 2.707-1.626 2.707-3.228V6.741c0-1.602-1.123-2.995-2.707-3.228A48.394 48.394 0 0 0 12 3c-2.392 0-4.744.175-7.043.513C3.373 3.746 2.25 5.14 2.25 6.741v5.779Z" />
                </svg>
                Banners & HTTP Responses
              </h4>
              <div class="space-y-2">
                {#each Object.entries(scanData.banners) as [port, banner]}
                  <div class="text-xxs">
                    <span class="text-indigo-400 font-mono font-semibold block mb-0.5">Port {port}:</span>
                    <pre class="bg-slate-950/50 border border-slate-850/80 rounded p-2 text-slate-400 font-mono overflow-x-auto whitespace-pre-wrap max-h-32 text-[10px] leading-normal">{banner}</pre>
                  </div>
                {/each}
              </div>
            </div>
          {/if}

          <!-- Raw JSON Section -->
          <div class="space-y-2">
            <div class="flex justify-between items-center">
              <span class="text-xs font-bold text-slate-400 uppercase tracking-wider flex items-center gap-1.5 font-sans">
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" class="w-3.5 h-3.5 text-slate-500">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M14.25 9.75 16.5 12l-2.25 2.25m-4.5 0L7.5 12l2.25-2.25M6 20.25h12A2.25 2.25 0 0 0 20.25 18V6A2.25 2.25 0 0 0 18 3.75H6A2.25 2.25 0 0 0 3.75 6v12A2.25 2.25 0 0 0 6 20.25Z" />
                </svg>
                Raw JSON Data
              </span>
              <button 
                on:click={handleCopy} 
                class="bg-slate-850 hover:bg-slate-800 text-slate-300 text-xxs font-semibold px-2.5 py-1.5 rounded-lg border border-slate-700/80 transition duration-150 flex items-center gap-1 font-sans"
              >
                {#if copied}
                  <span class="text-emerald-400 font-medium">Copied!</span>
                {:else}
                  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" class="w-3.5 h-3.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 17.25v3.375c0 .621-.504 1.125-1.125 1.125h-9.75a1.125 1.125 0 0 1-1.125-1.125v-9a1.125 1.125 0 0 1 1.125-1.125H6.75a9.06 9.06 0 0 1 1.5.124m7.5 10.376A8.965 8.965 0 0 0 12 12.75c-.497 0-.982.04-1.455.12m1.455-1.125V3.75m0 0a9 9 0 0 0-1 18m1-18a9 9 0 0 1 1 18" />
                  </svg>
                  Copy JSON
                {/if}
              </button>
            </div>
            
            <div class="relative bg-slate-950 rounded-xl border border-slate-800 overflow-hidden">
              <pre class="p-4 text-slate-300 font-mono text-[10px] overflow-auto max-h-[300px] leading-relaxed select-text">{jsonString}</pre>
            </div>
          </div>
        {/if}

      </div>

      <!-- Footer -->
      <div class="px-6 py-3 border-t border-slate-800 bg-slate-950/20 flex justify-end rounded-b-2xl">
        <button 
          on:click={onClose} 
          class="bg-sky-600 hover:bg-sky-500 text-white text-xs font-semibold px-5 py-2 rounded-xl transition duration-150 shadow-md shadow-sky-600/10 font-sans"
        >
          Close
        </button>
      </div>

    </div>
  </div>
{/if}
