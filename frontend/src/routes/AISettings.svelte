<script>
  import { onMount } from 'svelte';
  import { GetConfig, SaveConfig, GetHistory, DeleteNodeHistory, DeleteLinkHistory, ClearAllHistory } from '../../wailsjs/go/main/App';

  export let config = {
    Subnet: '192.168.1.0/24',
    SnmpConfigs: [],
    Timeout: 3,
    Retry: 1,
    ActiveProvider: 'ollama',
    OllamaURL: 'http://localhost:11434',
    OllamaModel: 'llama3',
    APIKeyOpenAI: '',
    APIKeyGemini: '',
    Language: 'auto'
  };

  let saving = false;
  let statusMessage = '';
  let statusType = 'success';

  let historyData = { nodes: [], links: [] };
  let loadingHistory = false;

  onMount(async () => {
    try {
      const cfg = await GetConfig();
      if (cfg) {
        config = { ...config, ...cfg };
      }
      await fetchHistory();
    } catch (err) {
      console.error('Failed to load config:', err);
    }
  });

  async function fetchHistory() {
    loadingHistory = true;
    try {
      const data = await GetHistory();
      if (data) {
        historyData = data;
      }
    } catch (err) {
      console.error('Failed to load history:', err);
    } finally {
      loadingHistory = false;
    }
  }

  async function handleDeleteNodeHistory(id) {
    if (confirm('Are you sure you want to delete this device history?')) {
      try {
        await DeleteNodeHistory(id);
        await fetchHistory();
      } catch (err) {
        alert('Failed to delete: ' + err.message);
      }
    }
  }

  async function handleDeleteLinkHistory(id) {
    if (confirm('Are you sure you want to delete this connection history?')) {
      try {
        await DeleteLinkHistory(id);
        await fetchHistory();
      } catch (err) {
        alert('Failed to delete: ' + err.message);
      }
    }
  }

  async function handleClearAllHistory() {
    if (confirm('Are you sure you want to clear ALL user editing history? This cannot be undone.')) {
      try {
        await ClearAllHistory();
        await fetchHistory();
      } catch (err) {
        alert('Failed to clear history: ' + err.message);
      }
    }
  }

  async function handleSave() {
    saving = true;
    statusMessage = '';
    try {
      // Basic validations
      if (config.ActiveProvider === 'openai' && !config.APIKeyOpenAI.trim()) {
        throw new Error('OpenAI API Key is required.');
      }
      if (config.ActiveProvider === 'gemini' && !config.APIKeyGemini.trim()) {
        throw new Error('Gemini API Key is required.');
      }
      await SaveConfig(config);
      statusType = 'success';
      statusMessage = 'AI settings saved successfully!';
    } catch (err) {
      statusType = 'error';
      statusMessage = err.message || 'Failed to save settings.';
    } finally {
      saving = false;
    }
  }
</script>

<div class="max-w-2xl mx-auto p-6 bg-slate-800/80 rounded-2xl border border-slate-700/60 shadow-2xl backdrop-blur-md">
  <div class="mb-6">
    <h2 class="text-2xl font-bold bg-gradient-to-r from-sky-400 to-indigo-400 bg-clip-text text-transparent">AI & Inference Settings</h2>
    <p class="text-sm text-slate-400 mt-1">Configure LLM integrations for device type and network topology inference.</p>
  </div>

  <form on:submit|preventDefault={handleSave} class="space-y-5">
    <div>
      <label for="provider" class="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">Active AI Provider</label>
      <select
        id="provider"
        bind:value={config.ActiveProvider}
        class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-3 text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50 focus:border-sky-500 transition duration-200"
      >
        <option value="ollama">Ollama (Local LLM)</option>
        <option value="openai">OpenAI (Cloud GPT)</option>
        <option value="gemini">Google Gemini (Cloud AI)</option>
      </select>
    </div>

    {#if config.ActiveProvider === 'ollama'}
      <div class="space-y-4">
        <div>
          <label for="ollamaUrl" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">Ollama Host URL</label>
          <input
            type="text"
            id="ollamaUrl"
            bind:value={config.OllamaURL}
            placeholder="http://localhost:11434"
            class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-3 text-slate-200 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-sky-500/50 focus:border-sky-500 transition duration-200"
          />
        </div>

        <div>
          <label for="ollamaModel" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">Ollama Model Name</label>
          <input
            type="text"
            id="ollamaModel"
            bind:value={config.OllamaModel}
            placeholder="e.g. llama3, mistral, gemma"
            class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-3 text-slate-200 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-sky-500/50 focus:border-sky-500 transition duration-200"
          />
        </div>
      </div>
    {/if}

    {#if config.ActiveProvider === 'openai'}
      <div>
        <label for="openaiKey" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">OpenAI API Key</label>
        <input
          type="password"
          id="openaiKey"
          bind:value={config.APIKeyOpenAI}
          placeholder="sk-••••••••••••••••••••••••"
          class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-3 text-slate-200 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-sky-500/50 focus:border-sky-500 transition duration-200"
        />
        <span class="text-xxs text-slate-500 mt-1 block">Your API Key is saved securely on your local disk in standard Bolt DB.</span>
      </div>
    {/if}

    {#if config.ActiveProvider === 'gemini'}
      <div>
        <label for="geminiKey" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">Gemini API Key</label>
        <input
          type="password"
          id="geminiKey"
          bind:value={config.APIKeyGemini}
          placeholder="AIzaSy••••••••••••••••••••••••"
          class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-3 text-slate-200 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-sky-500/50 focus:border-sky-500 transition duration-200"
        />
        <span class="text-xxs text-slate-500 mt-1 block">Your API Key is saved securely on your local disk in standard Bolt DB.</span>
      </div>
    {/if}

    <hr class="border-slate-700/50 my-6" />

    <div>
      <label for="language" class="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">System Language</label>
      <select
        id="language"
        bind:value={config.Language}
        class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-3 text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50 focus:border-sky-500 transition duration-200"
      >
        <option value="auto">Auto (Detect OS Language)</option>
        <option value="en">English</option>
        <option value="ja">日本語</option>
      </select>
    </div>

    <div class="flex items-center justify-between pt-4">
      <button
        type="submit"
        disabled={saving}
        class="bg-gradient-to-r from-sky-500 to-indigo-500 hover:from-sky-400 hover:to-indigo-400 disabled:from-slate-700 disabled:to-slate-700 text-white font-semibold rounded-xl px-6 py-3 shadow-lg shadow-sky-500/20 active:scale-95 transition duration-150"
      >
        {saving ? 'Saving...' : 'Save Configuration'}
      </button>

      {#if statusMessage}
        <span class={`text-sm ${statusType === 'success' ? 'text-emerald-400' : 'text-rose-400'} font-medium animate-pulse`}>
          {statusMessage}
        </span>
      {/if}
    </div>
  </form>

  <div class="mt-8 border-t border-slate-700/50 pt-8">
    <div class="flex items-center justify-between mb-4">
      <div>
        <h3 class="text-xl font-bold bg-gradient-to-r from-sky-400 to-indigo-400 bg-clip-text text-transparent">User Edit History (Learning Data)</h3>
        <p class="text-xs text-slate-400 mt-1">Manual edits to devices and links are saved here. They are used as context for future AI inferences.</p>
      </div>
      {#if (historyData.nodes && historyData.nodes.length > 0) || (historyData.links && historyData.links.length > 0)}
        <button
          on:click={handleClearAllHistory}
          class="bg-rose-500/20 hover:bg-rose-500/30 text-rose-300 border border-rose-500/30 font-semibold rounded-xl px-4 py-2 text-xs transition duration-150"
        >
          Clear All History
        </button>
      {/if}
    </div>

    {#if loadingHistory}
      <div class="text-center text-sm text-slate-500 py-6">Loading history...</div>
    {:else}
      <div class="space-y-6">
        <!-- Devices History -->
        <div>
          <h4 class="text-sm font-semibold text-slate-300 uppercase tracking-wider mb-2">Device Edits ({historyData.nodes ? historyData.nodes.length : 0})</h4>
          {#if historyData.nodes && historyData.nodes.length > 0}
            <div class="overflow-x-auto bg-slate-900/60 rounded-xl border border-slate-700/50">
              <table class="w-full text-left text-xs text-slate-300">
                <thead>
                  <tr class="border-b border-slate-800 bg-slate-900/80 text-slate-400">
                    <th class="px-4 py-3">ID (IP/MAC)</th>
                    <th class="px-4 py-3">Custom Label</th>
                    <th class="px-4 py-3">Custom Type</th>
                    <th class="px-4 py-3 text-right">Action</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-slate-800/40">
                  {#each historyData.nodes as node}
                    <tr>
                      <td class="px-4 py-3 font-mono text-slate-400">{node.id}</td>
                      <td class="px-4 py-3 font-semibold">{node.label}</td>
                      <td class="px-4 py-3">
                        <span class="px-2 py-0.5 rounded-full text-xxs font-medium bg-sky-500/10 text-sky-400 border border-sky-500/20">
                          {node.type}
                        </span>
                      </td>
                      <td class="px-4 py-3 text-right">
                        <button
                          on:click={() => handleDeleteNodeHistory(node.id)}
                          class="text-rose-400 hover:text-rose-300 font-semibold transition duration-150"
                        >
                          Delete
                        </button>
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {:else}
            <div class="text-sm text-slate-500 bg-slate-900/30 rounded-xl border border-slate-700/20 p-4 text-center">
              No device edit history yet.
            </div>
          {/if}
        </div>

        <!-- Links History -->
        <div>
          <h4 class="text-sm font-semibold text-slate-300 uppercase tracking-wider mb-2">Connection Edits ({historyData.links ? historyData.links.length : 0})</h4>
          {#if historyData.links && historyData.links.length > 0}
            <div class="overflow-x-auto bg-slate-900/60 rounded-xl border border-slate-700/50">
              <table class="w-full text-left text-xs text-slate-300">
                <thead>
                  <tr class="border-b border-slate-800 bg-slate-900/80 text-slate-400">
                    <th class="px-4 py-3">Connection Pair</th>
                    <th class="px-4 py-3">Type</th>
                    <th class="px-4 py-3">Style</th>
                    <th class="px-4 py-3">Status</th>
                    <th class="px-4 py-3 text-right">Action</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-slate-800/40">
                  {#each historyData.links as link}
                    <tr>
                      <td class="px-4 py-3 font-mono text-slate-400">{link.from} ↔ {link.to}</td>
                      <td class="px-4 py-3">{link.type || 'N/A'}</td>
                      <td class="px-4 py-3">
                        {#if link.style}
                          <span class="px-2 py-0.5 rounded-full text-xxs font-medium bg-slate-800 text-slate-300 border border-slate-700">
                            {link.style}
                          </span>
                        {:else}
                          <span class="text-slate-500">default</span>
                        {/if}
                      </td>
                      <td class="px-4 py-3">
                        {#if link.deleted}
                          <span class="px-2 py-0.5 rounded-full text-xxs font-medium bg-rose-500/10 text-rose-400 border border-rose-500/20">
                            Blocked/Deleted
                          </span>
                        {:else}
                          <span class="px-2 py-0.5 rounded-full text-xxs font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                            Custom Connection
                          </span>
                        {/if}
                      </td>
                      <td class="px-4 py-3 text-right">
                        <button
                          on:click={() => handleDeleteLinkHistory(link.id)}
                          class="text-rose-400 hover:text-rose-300 font-semibold transition duration-150"
                        >
                          Delete
                        </button>
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {:else}
            <div class="text-sm text-slate-500 bg-slate-900/30 rounded-xl border border-slate-700/20 p-4 text-center">
              No connection edit history yet.
            </div>
          {/if}
        </div>
      </div>
    {/if}
  </div>
</div>
