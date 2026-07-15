<script>
  import { onMount } from 'svelte';
  import { GetConfig, SaveConfig, GetHistory, DeleteNodeHistory, DeleteLinkHistory, ClearAllHistory } from '../../wailsjs/go/main/App';
  import { t } from '../i18n.js';

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
    if (confirm($t('confirmDeleteDeviceHist'))) {
      try {
        await DeleteNodeHistory(id);
        await fetchHistory();
      } catch (err) {
        alert($t('toastDeleteHistoryFailed', { err: err.message || err }));
      }
    }
  }

  async function handleDeleteLinkHistory(id) {
    if (confirm($t('confirmDeleteConnectionHist'))) {
      try {
        await DeleteLinkHistory(id);
        await fetchHistory();
      } catch (err) {
        alert($t('toastDeleteHistoryFailed', { err: err.message || err }));
      }
    }
  }

  async function handleClearAllHistory() {
    if (confirm($t('confirmClearAllHist'))) {
      try {
        await ClearAllHistory();
        await fetchHistory();
      } catch (err) {
        alert($t('toastClearHistoryFailed', { err: err.message || err }));
      }
    }
  }

  async function handleSave() {
    saving = true;
    statusMessage = '';
    try {
      // Basic validations
      if (config.ActiveProvider === 'openai' && !config.APIKeyOpenAI.trim()) {
        throw new Error($t('validationOpenaiKeyRequired'));
      }
      if (config.ActiveProvider === 'gemini' && !config.APIKeyGemini.trim()) {
        throw new Error($t('validationGeminiKeyRequired'));
      }
      await SaveConfig(config);
      statusType = 'success';
      statusMessage = $t('toastAiSettingsSaved');
    } catch (err) {
      statusType = 'error';
      statusMessage = err.message || $t('toastAiSettingsSaveFailed', { err });
    } finally {
      saving = false;
    }
  }
</script>

<div class="max-w-2xl mx-auto p-6 bg-slate-800/80 rounded-2xl border border-slate-700/60 shadow-2xl backdrop-blur-md">
  <div class="mb-6">
    <h2 class="text-2xl font-bold bg-gradient-to-r from-sky-400 to-indigo-400 bg-clip-text text-transparent">{$t('aiInferenceTitle')}</h2>
    <p class="text-sm text-slate-400 mt-1">{$t('aiInferenceDesc')}</p>
  </div>

  <form on:submit|preventDefault={handleSave} class="space-y-5">
    <div>
      <label for="provider" class="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">{$t('activeAiProvider')}</label>
      <select
        id="provider"
        bind:value={config.ActiveProvider}
        class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-3 text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50 focus:border-sky-500 transition duration-200"
      >
        <option value="ollama">{$t('ollamaLocal')}</option>
        <option value="openai">{$t('openaiCloud')}</option>
        <option value="gemini">{$t('geminiCloud')}</option>
      </select>
    </div>

    {#if config.ActiveProvider === 'ollama'}
      <div class="space-y-4">
        <div>
          <label for="ollamaUrl" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">{$t('ollamaHostUrl')}</label>
          <input
            type="text"
            id="ollamaUrl"
            bind:value={config.OllamaURL}
            placeholder="http://localhost:11434"
            class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-3 text-slate-200 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-sky-500/50 focus:border-sky-500 transition duration-200"
          />
        </div>

        <div>
          <label for="ollamaModel" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">{$t('ollamaModelName')}</label>
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
        <label for="openaiKey" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">{$t('openaiApiKey')}</label>
        <input
          type="password"
          id="openaiKey"
          bind:value={config.APIKeyOpenAI}
          placeholder="sk-••••••••••••••••••••••••"
          class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-3 text-slate-200 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-sky-500/50 focus:border-sky-500 transition duration-200"
        />
        <span class="text-xxs text-slate-500 mt-1 block">{$t('apiKeySecureNote')}</span>
      </div>
    {/if}

    {#if config.ActiveProvider === 'gemini'}
      <div>
        <label for="geminiKey" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">{$t('geminiApiKey')}</label>
        <input
          type="password"
          id="geminiKey"
          bind:value={config.APIKeyGemini}
          placeholder="AIzaSy••••••••••••••••••••••••"
          class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-3 text-slate-200 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-sky-500/50 focus:border-sky-500 transition duration-200"
        />
        <span class="text-xxs text-slate-500 mt-1 block">{$t('apiKeySecureNote')}</span>
      </div>
    {/if}



    <div class="flex items-center justify-between pt-4">
      <button
        type="submit"
        disabled={saving}
        class="bg-gradient-to-r from-sky-500 to-indigo-500 hover:from-sky-400 hover:to-indigo-400 disabled:from-slate-700 disabled:to-slate-700 text-white font-semibold rounded-xl px-6 py-3 shadow-lg shadow-sky-500/20 active:scale-95 transition duration-150"
      >
        {saving ? $t('savingBtn') : $t('saveConfigBtn')}
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
        <h3 class="text-xl font-bold bg-gradient-to-r from-sky-400 to-indigo-400 bg-clip-text text-transparent">{$t('userEditHistoryTitle')}</h3>
        <p class="text-xs text-slate-400 mt-1">{$t('userEditHistoryDesc')}</p>
      </div>
      {#if (historyData.nodes && historyData.nodes.length > 0) || (historyData.links && historyData.links.length > 0)}
        <button
          on:click={handleClearAllHistory}
          class="bg-rose-500/20 hover:bg-rose-500/30 text-rose-300 border border-rose-500/30 font-semibold rounded-xl px-4 py-2 text-xs transition duration-150"
        >
          {$t('clearAllHistory')}
        </button>
      {/if}
    </div>

    {#if loadingHistory}
      <div class="text-center text-sm text-slate-500 py-6">{$t('loadingHistory')}</div>
    {:else}
      <div class="space-y-6">
        <!-- Devices History -->
        <div>
          <h4 class="text-sm font-semibold text-slate-300 uppercase tracking-wider mb-2">{$t('deviceEdits', { count: historyData.nodes ? historyData.nodes.length : 0 })}</h4>
          {#if historyData.nodes && historyData.nodes.length > 0}
            <div class="overflow-x-auto bg-slate-900/60 rounded-xl border border-slate-700/50">
              <table class="w-full text-left text-xs text-slate-300">
                <thead>
                  <tr class="border-b border-slate-800 bg-slate-900/80 text-slate-400">
                    <th class="px-4 py-3">{$t('colIdIpMac')}</th>
                    <th class="px-4 py-3">{$t('colCustomLabel')}</th>
                    <th class="px-4 py-3">{$t('colCustomType')}</th>
                    <th class="px-4 py-3 text-right">{$t('colAction')}</th>
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
                          {$t('btnDelete')}
                        </button>
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {:else}
            <div class="text-sm text-slate-500 bg-slate-900/30 rounded-xl border border-slate-700/20 p-4 text-center">
              {$t('noDeviceHistory')}
            </div>
          {/if}
        </div>

        <!-- Links History -->
        <div>
          <h4 class="text-sm font-semibold text-slate-300 uppercase tracking-wider mb-2">{$t('connectionEdits', { count: historyData.links ? historyData.links.length : 0 })}</h4>
          {#if historyData.links && historyData.links.length > 0}
            <div class="overflow-x-auto bg-slate-900/60 rounded-xl border border-slate-700/50">
              <table class="w-full text-left text-xs text-slate-300">
                <thead>
                  <tr class="border-b border-slate-800 bg-slate-900/80 text-slate-400">
                    <th class="px-4 py-3">{$t('colConnectionPair')}</th>
                    <th class="px-4 py-3">{$t('colType')}</th>
                    <th class="px-4 py-3">{$t('colStyle')}</th>
                    <th class="px-4 py-3">{$t('colStatus')}</th>
                    <th class="px-4 py-3 text-right">{$t('colAction')}</th>
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
                            {$t('statusBlockedDeleted')}
                          </span>
                        {:else}
                          <span class="px-2 py-0.5 rounded-full text-xxs font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                            {$t('statusCustomConnection')}
                          </span>
                        {/if}
                      </td>
                      <td class="px-4 py-3 text-right">
                        <button
                          on:click={() => handleDeleteLinkHistory(link.id)}
                          class="text-rose-400 hover:text-rose-300 font-semibold transition duration-150"
                        >
                          {$t('btnDelete')}
                        </button>
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {:else}
            <div class="text-sm text-slate-500 bg-slate-900/30 rounded-xl border border-slate-700/20 p-4 text-center">
              {$t('noConnectionHistory')}
            </div>
          {/if}
        </div>
      </div>
    {/if}
  </div>
</div>
