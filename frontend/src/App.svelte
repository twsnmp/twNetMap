<script>
  import { onMount } from 'svelte';
  import { GetConfig } from '../wailsjs/go/main/App';
  
  import NetworkMap from './routes/NetworkMap.svelte';
  import ScanSettings from './routes/ScanSettings.svelte';
  import AISettings from './routes/AISettings.svelte';

  let activeTab = 'dashboard'; // 'dashboard' | 'scan' | 'ai'
  let config = {
    Subnet: '192.168.1.0/24',
    SnmpMode: 'v2c',
    SnmpCommunity: 'public',
    SnmpUser: '',
    SnmpPassword: '',
    Timeout: 3,
    Retry: 1,
    ActiveProvider: 'ollama',
    OllamaURL: 'http://localhost:11434',
    OllamaModel: 'llama3',
    APIKeyOpenAI: '',
    APIKeyGemini: '',
    Language: 'auto'
  };

  onMount(async () => {
    try {
      const cfg = await GetConfig();
      if (cfg) {
        config = { ...config, ...cfg };
      }
    } catch (err) {
      console.error('Failed to load initial config:', err);
    }
  });

  async function handleConfigChanged() {
    try {
      const cfg = await GetConfig();
      if (cfg) {
        config = { ...config, ...cfg };
      }
    } catch (err) {
      console.error('Failed to refresh config:', err);
    }
  }
</script>

<div class="flex flex-col h-screen w-screen bg-slate-950 text-slate-100 select-none overflow-hidden">
  <!-- Top Navigation Bar -->
  <header class="flex items-center justify-between px-6 py-4 bg-slate-900/60 border-b border-slate-800/80 backdrop-blur-md z-40">
    <div class="flex items-center gap-3">
      <!-- App Brand Logo / Icon -->
      <div class="flex items-center justify-center w-8 h-8 rounded-lg bg-gradient-to-tr from-sky-500 to-indigo-500 shadow-lg shadow-sky-500/20">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="white" class="w-5 h-5">
          <path stroke-linecap="round" stroke-linejoin="round" d="M18 18.72a9.094 9.094 0 0 0 3.741-.479 3 3 0 0 0-4.682-2.72m.94 3.198.001.031c0 .225-.012.447-.037.666A11.944 11.944 0 0 1 12 21c-2.17 0-4.207-.576-5.963-1.584A6.062 6.062 0 0 1 6 18.719m0 0a8.967 8.967 0 0 1-2.907-1.047M6 11.25a3 3 0 1 1-6 0 3 3 0 0 1 6 0ZM19.5 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
        </svg>
      </div>
      <div>
        <h1 class="text-sm font-bold tracking-wide uppercase bg-gradient-to-r from-sky-400 to-indigo-400 bg-clip-text text-transparent">twNetMap</h1>
        <p class="text-xxs text-slate-500 font-semibold tracking-wider">TWSNMP AI-Topology Suite</p>
      </div>
    </div>

    <!-- Navigation Tabs -->
    <nav class="flex items-center gap-1.5 bg-slate-950/80 p-1 rounded-xl border border-slate-800/50">
      <button 
        on:click={() => activeTab = 'dashboard'} 
        class={`flex items-center gap-1.5 px-4 py-2 rounded-lg text-xs font-semibold tracking-wide transition duration-150 ${activeTab === 'dashboard' ? 'bg-sky-500/10 text-sky-400 border border-sky-500/20' : 'text-slate-400 hover:text-slate-200 border border-transparent'}`}
      >
        Network Map
      </button>
      <button 
        on:click={() => { activeTab = 'scan'; handleConfigChanged(); }} 
        class={`flex items-center gap-1.5 px-4 py-2 rounded-lg text-xs font-semibold tracking-wide transition duration-150 ${activeTab === 'scan' ? 'bg-sky-500/10 text-sky-400 border border-sky-500/20' : 'text-slate-400 hover:text-slate-200 border border-transparent'}`}
      >
        Scan Settings
      </button>
      <button 
        on:click={() => { activeTab = 'ai'; handleConfigChanged(); }} 
        class={`flex items-center gap-1.5 px-4 py-2 rounded-lg text-xs font-semibold tracking-wide transition duration-150 ${activeTab === 'ai' ? 'bg-sky-500/10 text-sky-400 border border-sky-500/20' : 'text-slate-400 hover:text-slate-200 border border-transparent'}`}
      >
        AI Settings
      </button>
    </nav>

    <!-- Version badge -->
    <div class="text-xxs text-slate-500 font-mono select-none">
      v0.1.0
    </div>
  </header>

  <!-- Main View Router -->
  <main class="flex-grow w-full overflow-hidden relative">
    {#if activeTab === 'dashboard'}
      <div class="w-full h-full animate-fade-in">
        <NetworkMap {config} />
      </div>
    {:else if activeTab === 'scan'}
      <div class="w-full h-full overflow-y-auto p-8 bg-slate-950 animate-fade-in">
        <ScanSettings bind:config />
      </div>
    {:else if activeTab === 'ai'}
      <div class="w-full h-full overflow-y-auto p-8 bg-slate-950 animate-fade-in">
        <AISettings bind:config />
      </div>
    {/if}
  </main>
</div>

<style>
  /* Custom micro-animations */
  .animate-fade-in {
    animation: fadeIn 0.25s cubic-bezier(0.16, 1, 0.3, 1) forwards;
  }

  @keyframes fadeIn {
    from {
      opacity: 0;
      transform: translateY(4px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }
</style>
