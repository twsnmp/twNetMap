<script>
  import { onMount } from 'svelte';
  import { GetConfig, SaveConfig } from '../../wailsjs/go/main/App';
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

  // State for adding a new SNMP configuration
  let newSnmp = {
    SnmpMode: 'v2c',
    SnmpCommunity: 'public',
    SnmpUser: '',
    SnmpPassword: ''
  };

  onMount(async () => {
    try {
      const cfg = await GetConfig();
      if (cfg) {
        config = { ...config, ...cfg };
      }
    } catch (err) {
      console.error('Failed to load config:', err);
    }
  });

  function addConfig() {
    if (!config.SnmpConfigs) {
      config.SnmpConfigs = [];
    }
    
    // Simple validation
    if (newSnmp.SnmpMode === 'v2c' && !newSnmp.SnmpCommunity.trim()) {
      return;
    }
    if (newSnmp.SnmpMode !== 'v2c' && !newSnmp.SnmpUser.trim()) {
      return;
    }

    config.SnmpConfigs = [
      ...config.SnmpConfigs,
      {
        SnmpMode: newSnmp.SnmpMode,
        SnmpCommunity: newSnmp.SnmpCommunity,
        SnmpUser: newSnmp.SnmpUser,
        SnmpPassword: newSnmp.SnmpPassword
      }
    ];

    // Reset inputs
    newSnmp = {
      SnmpMode: 'v2c',
      SnmpCommunity: 'public',
      SnmpUser: '',
      SnmpPassword: ''
    };
  }

  function removeConfig(index) {
    config.SnmpConfigs = config.SnmpConfigs.filter((_, i) => i !== index);
  }

  function moveConfig(index, direction) {
    const list = [...(config.SnmpConfigs || [])];
    const targetIndex = index + direction;
    if (targetIndex < 0 || targetIndex >= list.length) return;
    
    // Swap
    const temp = list[index];
    list[index] = list[targetIndex];
    list[targetIndex] = temp;
    
    config.SnmpConfigs = list;
  }

  async function handleSave() {
    saving = true;
    statusMessage = '';
    try {
      // Validate subnet
      if (!config.Subnet.trim()) {
        throw new Error($t('validationSubnetEmpty'));
      }
      await SaveConfig(config);
      statusType = 'success';
      statusMessage = $t('toastSettingsSaved');
    } catch (err) {
      statusType = 'error';
      statusMessage = err.message || $t('toastSettingsSaveFailed', { err });
    } finally {
      saving = false;
    }
  }
</script>

<div class="max-w-2xl mx-auto p-6 bg-slate-800/80 rounded-2xl border border-slate-700/60 shadow-2xl backdrop-blur-md">
  <div class="mb-6">
    <h2 class="text-2xl font-bold bg-gradient-to-r from-sky-400 to-indigo-400 bg-clip-text text-transparent">{$t('scanConfigTitle')}</h2>
    <p class="text-sm text-slate-400 mt-1">{$t('scanConfigDesc')}</p>
  </div>

  <form on:submit|preventDefault={handleSave} class="space-y-5">
    <div>
      <label for="subnet" class="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">{$t('subnetRangeIp')}</label>
      <input
        type="text"
        id="subnet"
        bind:value={config.Subnet}
        placeholder="e.g. 192.168.1.0/24, 192.168.2.1-192.168.2.50, 192.168.3.100"
        class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-2 text-sm text-slate-200 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-sky-500/50 focus:border-sky-500 transition duration-200"
      />
      <span class="text-xxs text-slate-500 mt-1 block">{$t('subnetHelp')}</span>
    </div>

    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
      <div>
        <label for="timeout" class="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">{$t('timeoutSeconds')}</label>
        <input
          type="number"
          id="timeout"
          bind:value={config.Timeout}
          min="1"
          max="30"
          class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-2 text-sm text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50 focus:border-sky-500 transition duration-200"
        />
      </div>

      <div>
        <label for="retry" class="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">{$t('retryCount')}</label>
        <input
          type="number"
          id="retry"
          bind:value={config.Retry}
          min="0"
          max="5"
          class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-2 text-sm text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50 focus:border-sky-500 transition duration-200"
        />
      </div>
    </div>

    <hr class="border-slate-700/50 my-6" />

    <div class="mb-4 space-y-4">
      <h3 class="text-sm font-semibold text-slate-300">{$t('snmpSettingsList')}</h3>
      
      {#if config.SnmpConfigs && config.SnmpConfigs.length > 0}
        <div class="space-y-2">
          {#each config.SnmpConfigs as snmp, index}
            <div class="flex items-center gap-3 p-3 bg-slate-900/60 border border-slate-700/60 rounded-xl transition duration-150 hover:border-slate-600/80">
              <div class="flex-grow grid grid-cols-1 sm:grid-cols-3 gap-2">
                <div>
                  <span class="text-xxs text-slate-500 uppercase tracking-wider block font-medium">{$t('snmpMode')}</span>
                  <span class="text-xs text-slate-200 font-semibold">{snmp.SnmpMode}</span>
                </div>
                <div>
                  <span class="text-xxs text-slate-500 uppercase tracking-wider block font-medium whitespace-nowrap">
                    {snmp.SnmpMode === 'v2c' ? $t('communityString') : $t('username')}
                  </span>
                  <span class="text-xs text-slate-200 font-mono truncate block">
                    {snmp.SnmpMode === 'v2c' ? snmp.SnmpCommunity : snmp.SnmpUser}
                  </span>
                </div>
                <div>
                  {#if snmp.SnmpMode !== 'v2c'}
                    <span class="text-xxs text-slate-500 uppercase tracking-wider block font-medium">{$t('passwordAuthPriv')}</span>
                    <span class="text-xs text-slate-400 font-mono">••••••••</span>
                  {/if}
                </div>
              </div>
              
              <div class="flex items-center gap-1.5 border-l border-slate-800 pl-3">
                <button 
                  type="button" 
                  on:click={() => moveConfig(index, -1)} 
                  disabled={index === 0}
                  class="p-1 hover:bg-slate-800 text-slate-400 hover:text-slate-200 rounded-lg disabled:opacity-20 disabled:hover:bg-transparent transition duration-150"
                  title="Move Up"
                >
                  <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 15l7-7 7 7" />
                  </svg>
                </button>
                <button 
                  type="button" 
                  on:click={() => moveConfig(index, 1)} 
                  disabled={index === config.SnmpConfigs.length - 1}
                  class="p-1 hover:bg-slate-800 text-slate-400 hover:text-slate-200 rounded-lg disabled:opacity-20 disabled:hover:bg-transparent transition duration-150"
                  title="Move Down"
                >
                  <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                  </svg>
                </button>
                <button 
                  type="button" 
                  on:click={() => removeConfig(index)} 
                  class="p-1 hover:bg-rose-500/10 text-rose-400 hover:text-rose-300 rounded-lg transition duration-150"
                  title="Delete"
                >
                  <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                  </svg>
                </button>
              </div>
            </div>
          {/each}
        </div>
      {:else}
        <div class="text-center py-6 bg-slate-900/30 border border-dashed border-slate-800 rounded-xl text-slate-500 text-xs italic">
          {$t('noSnmpCredentials')}
        </div>
      {/if}

      <!-- Add New SNMP Configuration Box -->
      <div class="p-4 bg-slate-900/40 border border-dashed border-slate-700/40 rounded-2xl space-y-4">
        <h4 class="text-xs font-semibold text-slate-400 uppercase tracking-wider">{$t('addSnmpCredential')}</h4>
        
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <label class="block text-xxs font-semibold text-slate-500 uppercase tracking-wider mb-2">{$t('snmpMode')}</label>
            <select
              bind:value={newSnmp.SnmpMode}
              class="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 h-[38px] text-xs text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50 transition duration-200"
            >
              <option value="v2c">v2c ({$t('communityString')})</option>
              <option value="v3auth">v3auth ({$t('username')} / Auth Password)</option>
              <option value="v3authpriv">v3authpriv (Auth & Priv Encryption)</option>
            </select>
          </div>

          {#if newSnmp.SnmpMode === 'v2c'}
            <div>
              <label class="block text-xxs font-semibold text-slate-500 uppercase tracking-wider mb-2">{$t('communityString')}</label>
              <input
                type="text"
                bind:value={newSnmp.SnmpCommunity}
                placeholder="public"
                class="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 h-[38px] text-xs text-slate-200 placeholder-slate-600 focus:outline-none focus:ring-2 focus:ring-sky-500/50 transition duration-200"
              />
            </div>
          {:else}
            <div>
              <label class="block text-xxs font-semibold text-slate-500 uppercase tracking-wider mb-2">{$t('username')}</label>
              <input
                type="text"
                bind:value={newSnmp.SnmpUser}
                placeholder="e.g. admin"
                class="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 h-[38px] text-xs text-slate-200 placeholder-slate-600 focus:outline-none focus:ring-2 focus:ring-sky-500/50 transition duration-200"
              />
            </div>
          {/if}
        </div>

        {#if newSnmp.SnmpMode !== 'v2c'}
          <div>
            <label class="block text-xxs font-semibold text-slate-500 uppercase tracking-wider mb-2">{$t('passwordAuthPriv')}</label>
            <input
              type="password"
              bind:value={newSnmp.SnmpPassword}
              placeholder="••••••••••••"
              class="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 h-[38px] text-xs text-slate-200 placeholder-slate-600 focus:outline-none focus:ring-2 focus:ring-sky-500/50 transition duration-200"
            />
          </div>
        {/if}

        <button
          type="button"
          on:click={addConfig}
          class="w-full bg-slate-800 hover:bg-slate-700 text-sky-400 hover:text-sky-300 font-semibold rounded-xl py-2.5 text-xs border border-slate-700/50 hover:border-slate-600/50 transition duration-150"
        >
          {$t('addCredentialBtn')}
        </button>
      </div>
    </div>

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
</div>
