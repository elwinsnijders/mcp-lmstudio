<script>
  import { onMount } from 'svelte'
  import { LoadConfig, SaveConfig } from '../../wailsjs/go/main/App'
  import { addToast } from '../stores/toast.js'

  let config = null
  let editing = null
  let editForm = {}
  let newProfileKey = ''
  let showAddProfile = false

  onMount(async () => {
    await refresh()
  })

  async function refresh() {
    try {
      config = await LoadConfig()
    } catch (e) {
      addToast('Failed to load config: ' + e, 'error')
    }
  }

  function startEdit(key, profile) {
    editing = key
    editForm = { ...profile }
  }

  function cancelEdit() {
    editing = null
    editForm = {}
  }

  async function saveEdit() {
    if (!editing || !config) return
    config.profiles[editing] = { ...editForm }
    try {
      await SaveConfig(config)
      addToast('Profile saved', 'success')
      editing = null
      editForm = {}
    } catch (e) {
      addToast('Save failed: ' + e, 'error')
    }
  }

  async function addProfile() {
    if (!newProfileKey.trim() || !config) return
    const key = newProfileKey.trim().toLowerCase().replace(/\s+/g, '_')
    if (config.profiles[key]) {
      addToast('Profile key already exists', 'error')
      return
    }
    config.profiles[key] = {
      label: newProfileKey.trim(),
      description: '',
      system_prompt: '',
      model: '',
      temperature: 0.7,
      context_length: 0,
      top_p: 0,
      top_k: 0,
      min_p: 0,
      repeat_penalty: 0,
      max_output_tokens: 0,
      reasoning: '',
      integrations: [],
    }
    try {
      await SaveConfig(config)
      addToast('Profile added', 'success')
      newProfileKey = ''
      showAddProfile = false
      config = config
    } catch (e) {
      addToast('Save failed: ' + e, 'error')
    }
  }

  async function removeProfile(key) {
    if (!config) return
    delete config.profiles[key]
    config.profiles = config.profiles
    try {
      await SaveConfig(config)
      addToast('Profile removed', 'success')
      config = config
    } catch (e) {
      addToast('Remove failed: ' + e, 'error')
    }
  }

  $: profileKeys = config ? Object.keys(config.profiles || {}).sort() : []
  $: integrationKeys = config ? Object.keys(config.integrations || {}).sort() : []
</script>

<div class="flex flex-col gap-8">
  <div>
    <div class="flex items-center justify-between mb-4">
      <div>
        <h1 class="text-2xl font-semibold text-gray-900">Profiles</h1>
        <p class="text-sm text-gray-500 mt-1">Agent profiles from config.json</p>
      </div>
      <button
        class="px-3 py-1.5 text-xs font-medium text-white bg-violet-600 rounded-md hover:bg-violet-700 transition-colors"
        on:click={() => (showAddProfile = !showAddProfile)}
      >{showAddProfile ? 'Cancel' : 'Add Profile'}</button>
    </div>

    {#if showAddProfile}
      <div class="mb-4 p-4 bg-white rounded-xl border border-gray-200 shadow-sm flex items-center gap-3">
        <input
          type="text"
          placeholder="Profile key (e.g. coder)"
          bind:value={newProfileKey}
          class="flex-1 px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-violet-500 focus:border-violet-500 outline-none"
        />
        <button
          class="px-4 py-2 text-sm font-medium text-white bg-violet-600 rounded-lg hover:bg-violet-700 transition-colors"
          on:click={addProfile}
        >Create</button>
      </div>
    {/if}

    {#if !config}
      <div class="p-8 text-center text-gray-400">Loading...</div>
    {:else if profileKeys.length === 0}
      <div class="p-8 text-center text-gray-400 bg-white rounded-xl border border-gray-200">
        <p class="font-medium">No profiles configured</p>
        <p class="text-xs mt-1">Add profiles to config.json or click "Add Profile" above</p>
      </div>
    {:else}
      <div class="space-y-3">
        {#each profileKeys as key}
          {@const profile = config.profiles[key]}
          <div class="bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden">
            {#if editing === key}
              <div class="p-5 space-y-4">
                <div class="grid grid-cols-2 gap-4">
                  <div>
                    <label class="block text-xs font-medium text-gray-600 mb-1">Label</label>
                    <input type="text" bind:value={editForm.label}
                      class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-violet-500 focus:border-violet-500 outline-none" />
                  </div>
                  <div>
                    <label class="block text-xs font-medium text-gray-600 mb-1">Model</label>
                    <input type="text" bind:value={editForm.model}
                      class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-violet-500 focus:border-violet-500 outline-none" />
                  </div>
                </div>
                <div>
                  <label class="block text-xs font-medium text-gray-600 mb-1">Description</label>
                  <input type="text" bind:value={editForm.description}
                    class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-violet-500 focus:border-violet-500 outline-none" />
                </div>
                <div class="grid grid-cols-4 gap-3">
                  <div>
                    <label class="block text-xs font-medium text-gray-600 mb-1">Temperature</label>
                    <input type="number" step="0.1" bind:value={editForm.temperature}
                      class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-violet-500 focus:border-violet-500 outline-none" />
                  </div>
                  <div>
                    <label class="block text-xs font-medium text-gray-600 mb-1">Context Length</label>
                    <input type="number" bind:value={editForm.context_length}
                      class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-violet-500 focus:border-violet-500 outline-none" />
                  </div>
                  <div>
                    <label class="block text-xs font-medium text-gray-600 mb-1">Top P</label>
                    <input type="number" step="0.05" bind:value={editForm.top_p}
                      class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-violet-500 focus:border-violet-500 outline-none" />
                  </div>
                  <div>
                    <label class="block text-xs font-medium text-gray-600 mb-1">Reasoning</label>
                    <input type="text" bind:value={editForm.reasoning}
                      class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-violet-500 focus:border-violet-500 outline-none" />
                  </div>
                </div>
                <div>
                  <label class="block text-xs font-medium text-gray-600 mb-1">System Prompt</label>
                  <textarea bind:value={editForm.system_prompt} rows="4"
                    class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-violet-500 focus:border-violet-500 outline-none font-mono"></textarea>
                </div>
                <div class="flex gap-2 justify-end">
                  <button class="px-3 py-1.5 text-xs font-medium text-gray-600 bg-gray-100 rounded-md hover:bg-gray-200" on:click={cancelEdit}>Cancel</button>
                  <button class="px-3 py-1.5 text-xs font-medium text-white bg-violet-600 rounded-md hover:bg-violet-700" on:click={saveEdit}>Save</button>
                </div>
              </div>
            {:else}
              <div class="p-5">
                <div class="flex items-start justify-between">
                  <div class="flex-1">
                    <div class="flex items-center gap-2">
                      <span class="font-mono text-xs px-2 py-0.5 bg-slate-100 text-slate-700 rounded font-semibold">{key}</span>
                      <span class="text-sm font-medium text-gray-900">{profile.label || ''}</span>
                    </div>
                    {#if profile.description}
                      <p class="text-xs text-gray-500 mt-1">{profile.description}</p>
                    {/if}
                    <div class="flex flex-wrap gap-3 mt-3 text-xs text-gray-500">
                      {#if profile.model}
                        <span class="px-2 py-0.5 bg-gray-50 rounded">Model: {profile.model}</span>
                      {/if}
                      {#if profile.temperature}
                        <span class="px-2 py-0.5 bg-gray-50 rounded">Temp: {profile.temperature}</span>
                      {/if}
                      {#if profile.context_length}
                        <span class="px-2 py-0.5 bg-gray-50 rounded">CTX: {profile.context_length.toLocaleString()}</span>
                      {/if}
                      {#if profile.reasoning}
                        <span class="px-2 py-0.5 bg-gray-50 rounded">Reasoning: {profile.reasoning}</span>
                      {/if}
                      {#if profile.integrations?.length}
                        <span class="px-2 py-0.5 bg-violet-50 text-violet-600 rounded">
                          Integrations: {profile.integrations.join(', ')}
                        </span>
                      {/if}
                    </div>
                  </div>
                  <div class="flex gap-1 ml-4 shrink-0">
                    <button class="px-2 py-1 text-[10px] font-medium text-violet-600 bg-violet-50 rounded hover:bg-violet-100" on:click={() => startEdit(key, profile)}>Edit</button>
                    <button class="px-2 py-1 text-[10px] font-medium text-red-600 bg-red-50 rounded hover:bg-red-100" on:click={() => removeProfile(key)}>Remove</button>
                  </div>
                </div>
              </div>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </div>

  <div>
    <h2 class="text-lg font-semibold text-gray-900 mb-3">Integrations</h2>
    {#if integrationKeys.length === 0}
      <div class="p-6 text-center text-gray-400 bg-white rounded-xl border border-gray-200">
        <p class="text-sm">No integrations configured</p>
      </div>
    {:else}
      <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
        {#each integrationKeys as key}
          {@const integ = config.integrations[key]}
          <div class="bg-white rounded-xl border border-gray-200 shadow-sm p-4">
            <div class="flex items-center gap-2 mb-2">
              <span class="font-mono text-xs px-2 py-0.5 bg-amber-50 text-amber-700 rounded font-semibold">{key}</span>
              <span class="text-xs px-1.5 py-0.5 rounded border
                {integ.type === 'plugin' ? 'text-blue-600 bg-blue-50 border-blue-200' : 'text-purple-600 bg-purple-50 border-purple-200'}">
                {integ.type}
              </span>
            </div>
            <p class="text-sm text-gray-700">{integ.label || ''}</p>
            {#if integ.description}
              <p class="text-xs text-gray-500 mt-0.5">{integ.description}</p>
            {/if}
            {#if integ.id}
              <p class="text-xs text-gray-400 mt-1 font-mono">{integ.id}</p>
            {/if}
            {#if integ.server_url}
              <p class="text-xs text-gray-400 mt-1 font-mono truncate" title={integ.server_url}>{integ.server_url}</p>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>
