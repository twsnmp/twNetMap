<script>
  import { onMount } from 'svelte';
  import { GetConfig, SaveConfig } from '../../wailsjs/go/main/App';

  export let config = {
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

  let saving = false;
  let statusMessage = '';
  let statusType = 'success';

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

  async function handleSave() {
    saving = true;
    statusMessage = '';
    try {
      // Validate subnet
      if (!config.Subnet.trim()) {
        throw new Error('Subnet target range cannot be empty.');
      }
      await SaveConfig(config);
      statusType = 'success';
      statusMessage = 'Settings saved successfully!';
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
    <h2 class="text-2xl font-bold bg-gradient-to-r from-sky-400 to-indigo-400 bg-clip-text text-transparent">Scan Configuration</h2>
    <p class="text-sm text-slate-400 mt-1">Setup network scanning IP targets and SNMP authentication keys.</p>
  </div>

  <form on:submit|preventDefault={handleSave} class="space-y-5">
    <div>
      <label for="subnet" class="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">Subnet / Range / IP</label>
      <input
        type="text"
        id="subnet"
        bind:value={config.Subnet}
        placeholder="e.g. 192.168.1.0/24, 192.168.2.1-192.168.2.50, 192.168.3.100"
        class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-3 text-slate-200 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-sky-500/50 focus:border-sky-500 transition duration-200"
      />
      <span class="text-xxs text-slate-500 mt-1 block">Supports comma-separated CIDR formats, IP ranges separated by hyphen, or single IP addresses. Limit to 1024 total hosts.</span>
    </div>

    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
      <div>
        <label for="timeout" class="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">Timeout (Seconds)</label>
        <input
          type="number"
          id="timeout"
          bind:value={config.Timeout}
          min="1"
          max="30"
          class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-3 text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50 focus:border-sky-500 transition duration-200"
        />
      </div>

      <div>
        <label for="retry" class="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">Retry Count</label>
        <input
          type="number"
          id="retry"
          bind:value={config.Retry}
          min="0"
          max="5"
          class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-3 text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50 focus:border-sky-500 transition duration-200"
        />
      </div>
    </div>

    <hr class="border-slate-700/50 my-6" />

    <div class="mb-4">
      <h3 class="text-sm font-semibold text-slate-300 mb-3">SNMP Settings</h3>
      
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
        <div>
          <label for="snmpMode" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">SNMP Mode</label>
          <select
            id="snmpMode"
            bind:value={config.SnmpMode}
            class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-3 text-slate-200 focus:outline-none focus:ring-2 focus:ring-sky-500/50 focus:border-sky-500 transition duration-200"
          >
            <option value="v2c">v2c (Community String)</option>
            <option value="v3auth">v3auth (Username / Auth Password)</option>
            <option value="v3authpriv">v3authpriv (Auth & Priv Encryption)</option>
          </select>
        </div>

        {#if config.SnmpMode === 'v2c'}
          <div>
            <label for="snmpCommunity" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">Community String</label>
            <input
              type="text"
              id="snmpCommunity"
              bind:value={config.SnmpCommunity}
              placeholder="public"
              class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-3 text-slate-200 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-sky-500/50 focus:border-sky-500 transition duration-200"
            />
          </div>
        {:else}
          <div>
            <label for="snmpUser" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">SNMPv3 Username</label>
            <input
              type="text"
              id="snmpUser"
              bind:value={config.SnmpUser}
              placeholder="e.g. admin"
              class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-3 text-slate-200 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-sky-500/50 focus:border-sky-500 transition duration-200"
            />
          </div>
        {/if}
      </div>

      {#if config.SnmpMode !== 'v2c'}
        <div class="mb-4">
          <label for="snmpPassword" class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">SNMPv3 Password (Auth/Priv)</label>
          <input
            type="password"
            id="snmpPassword"
            bind:value={config.SnmpPassword}
            placeholder="••••••••••••"
            class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-3 text-slate-200 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-sky-500/50 focus:border-sky-500 transition duration-200"
          />
        </div>
      {/if}
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
</div>
