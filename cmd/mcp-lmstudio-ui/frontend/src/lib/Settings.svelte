<script>
  import { onMount } from 'svelte'
  import { LoadSettings, SaveSettings, GetDataDir, SetDataDir } from '../../wailsjs/go/main/App'
  import { addToast } from '../stores/toast.js'

  let settings = null
  let dataDir = ''
  let dirty = false

  onMount(async () => {
    try {
      settings = await LoadSettings()
      dataDir = await GetDataDir()
    } catch (e) {
      addToast('Failed to load settings: ' + e, 'error')
    }
  })

  function markDirty() {
    dirty = true
  }

  async function save() {
    if (!settings) return
    try {
      await SaveSettings(settings)
      dirty = false
      addToast('Settings saved to .env file', 'success')
    } catch (e) {
      addToast('Save failed: ' + e, 'error')
    }
  }

  async function updateDataDir() {
    if (!dataDir.trim()) return
    try {
      await SetDataDir(dataDir.trim())
      addToast('Data directory updated', 'success')
    } catch (e) {
      addToast('Invalid directory: ' + e, 'error')
    }
  }
</script>

<div class="space-y-8">
  <div>
    <h1 class="text-2xl font-semibold text-gray-900">Settings</h1>
    <p class="text-sm text-gray-500 mt-1">MCP bridge configuration (environment variables). Saved to .env file.</p>
  </div>

  {#if !settings}
    <div class="p-8 text-center text-gray-400">Loading...</div>
  {:else}
    <div class="bg-white rounded-xl border border-gray-200 shadow-sm p-6">
      <h2 class="text-sm font-semibold text-gray-900 mb-4">Data Directory</h2>
      <div class="flex items-center gap-3">
        <input
          type="text"
          bind:value={dataDir}
          class="flex-1 px-3 py-2 border border-gray-300 rounded-lg text-sm font-mono focus:ring-2 focus:ring-violet-500 focus:border-violet-500 outline-none"
        />
        <button
          class="px-4 py-2 text-sm font-medium text-white bg-violet-600 rounded-lg hover:bg-violet-700 transition-colors"
          on:click={updateDataDir}
        >Update</button>
      </div>
      <p class="text-xs text-gray-400 mt-2">Root directory containing config.json, sessions/, chatlogs/, progress/</p>
    </div>

    <div class="bg-white rounded-xl border border-gray-200 shadow-sm p-6 space-y-5">
      <h2 class="text-sm font-semibold text-gray-900">LM Studio Connection</h2>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-xs font-medium text-gray-600 mb-1">API Base URL</label>
          <input type="text" bind:value={settings.apiBase} on:input={markDirty}
            class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-violet-500 focus:border-violet-500 outline-none" />
        </div>
        <div>
          <label class="block text-xs font-medium text-gray-600 mb-1">API Token</label>
          <input type="password" bind:value={settings.apiToken} on:input={markDirty}
            class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-violet-500 focus:border-violet-500 outline-none" />
        </div>
        <div>
          <label class="block text-xs font-medium text-gray-600 mb-1">Default Model</label>
          <input type="text" bind:value={settings.model} on:input={markDirty}
            class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-violet-500 focus:border-violet-500 outline-none" />
        </div>
        <div>
          <label class="block text-xs font-medium text-gray-600 mb-1">Request Timeout (minutes)</label>
          <input type="number" bind:value={settings.requestTimeout} on:input={markDirty}
            class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-violet-500 focus:border-violet-500 outline-none" />
        </div>
        <div>
          <label class="block text-xs font-medium text-gray-600 mb-1">Default Context Length</label>
          <input type="number" bind:value={settings.contextLength} on:input={markDirty}
            class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-violet-500 focus:border-violet-500 outline-none" />
        </div>
      </div>
    </div>

    <div class="bg-white rounded-xl border border-gray-200 shadow-sm p-6 space-y-5">
      <h2 class="text-sm font-semibold text-gray-900">Token Limits</h2>
      <div class="grid grid-cols-3 gap-4">
        <div>
          <label class="block text-xs font-medium text-gray-600 mb-1">Max Session Tokens</label>
          <input type="number" bind:value={settings.maxSessionTokens} on:input={markDirty}
            class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-violet-500 focus:border-violet-500 outline-none" />
        </div>
        <div>
          <label class="block text-xs font-medium text-gray-600 mb-1">Warning Threshold</label>
          <input type="number" step="0.01" bind:value={settings.tokenWarningThreshold} on:input={markDirty}
            class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-violet-500 focus:border-violet-500 outline-none" />
          <p class="text-[10px] text-gray-400 mt-1">0.80 = warn at 80%</p>
        </div>
        <div>
          <label class="block text-xs font-medium text-gray-600 mb-1">Critical Threshold</label>
          <input type="number" step="0.01" bind:value={settings.tokenCriticalThreshold} on:input={markDirty}
            class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-violet-500 focus:border-violet-500 outline-none" />
          <p class="text-[10px] text-gray-400 mt-1">0.95 = critical at 95%</p>
        </div>
      </div>
    </div>

    <div class="bg-white rounded-xl border border-gray-200 shadow-sm p-6 space-y-5">
      <h2 class="text-sm font-semibold text-gray-900">Directories & Files</h2>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-xs font-medium text-gray-600 mb-1">Sessions Directory</label>
          <input type="text" bind:value={settings.sessionsDir} on:input={markDirty}
            class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm font-mono focus:ring-2 focus:ring-violet-500 focus:border-violet-500 outline-none" />
        </div>
        <div>
          <label class="block text-xs font-medium text-gray-600 mb-1">Progress Directory</label>
          <input type="text" bind:value={settings.progressDir} on:input={markDirty}
            class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm font-mono focus:ring-2 focus:ring-violet-500 focus:border-violet-500 outline-none" />
        </div>
        <div>
          <label class="block text-xs font-medium text-gray-600 mb-1">Chatlog Directory</label>
          <input type="text" bind:value={settings.chatlogDir} on:input={markDirty}
            class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm font-mono focus:ring-2 focus:ring-violet-500 focus:border-violet-500 outline-none" />
        </div>
        <div>
          <label class="block text-xs font-medium text-gray-600 mb-1">Config File</label>
          <input type="text" bind:value={settings.configFile} on:input={markDirty}
            class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm font-mono focus:ring-2 focus:ring-violet-500 focus:border-violet-500 outline-none" />
        </div>
        <div>
          <label class="block text-xs font-medium text-gray-600 mb-1">Log File</label>
          <input type="text" bind:value={settings.logFile} on:input={markDirty}
            class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm font-mono focus:ring-2 focus:ring-violet-500 focus:border-violet-500 outline-none" />
        </div>
      </div>
    </div>

    <div class="flex justify-end">
      <button
        class="px-6 py-2.5 text-sm font-medium rounded-lg transition-colors
          {dirty ? 'text-white bg-violet-600 hover:bg-violet-700' : 'text-gray-400 bg-gray-100 cursor-not-allowed'}"
        disabled={!dirty}
        on:click={save}
      >Save Settings</button>
    </div>
  {/if}
</div>
