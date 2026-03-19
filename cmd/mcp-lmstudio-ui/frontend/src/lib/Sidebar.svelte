<script>
  import { createEventDispatcher, onMount } from 'svelte'
  import { GetDataDir } from '../../wailsjs/go/main/App'

  export let currentPage

  const dispatch = createEventDispatcher()
  let dataDir = ''

  onMount(async () => {
    try {
      dataDir = await GetDataDir()
    } catch (_) {}
  })

  const navItems = [
    { id: 'sessions', label: 'Sessions', icon: 'sessions' },
    { id: 'live', label: 'Live View', icon: 'live' },
    { id: 'archive', label: 'Chat Archive', icon: 'archive' },
    { id: 'profiles', label: 'Profiles', icon: 'profiles' },
    { id: 'settings', label: 'Settings', icon: 'settings' },
  ]
</script>

<aside class="fixed left-0 top-0 h-full w-64 bg-slate-900 text-white flex flex-col z-10 shadow-xl">
  <div class="p-6 border-b border-slate-700/50">
    <div class="flex items-center gap-3">
      <div class="w-9 h-9 bg-violet-500 rounded-lg flex items-center justify-center">
        <svg class="w-5 h-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M9.75 3.104v5.714a2.25 2.25 0 01-.659 1.591L5 14.5M9.75 3.104c-.251.023-.501.05-.75.082m.75-.082a24.301 24.301 0 014.5 0m0 0v5.714c0 .597.237 1.17.659 1.591L19.8 15.3M14.25 3.104c.251.023.501.05.75.082M19.8 15.3l-1.57.393A9.065 9.065 0 0112 15a9.065 9.065 0 00-6.23.693L5 14.5m14.8.8l1.402 1.402c1.232 1.232.65 3.318-1.067 3.611A48.309 48.309 0 0112 21c-2.773 0-5.491-.235-8.135-.687-1.718-.293-2.3-2.379-1.067-3.61L5 14.5" />
        </svg>
      </div>
      <div>
        <h1 class="text-base font-bold tracking-tight">LM Studio MCP</h1>
        <p class="text-xs text-slate-400">Console</p>
      </div>
    </div>
  </div>

  <nav class="flex-1 p-3 space-y-1 mt-2">
    {#each navItems as item}
      <button
        class="w-full text-left px-4 py-2.5 rounded-lg transition-all duration-150 flex items-center gap-3 text-sm {currentPage === item.id
          ? 'bg-violet-600 text-white shadow-md shadow-violet-600/20'
          : 'text-slate-300 hover:bg-slate-800 hover:text-white'}"
        on:click={() => dispatch('navigate', item.id)}
      >
        {#if item.icon === 'sessions'}
          <svg class="w-[18px] h-[18px] shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.75">
            <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 12h16.5m-16.5 3.75h16.5M3.75 19.5h16.5M5.625 4.5h12.75a1.875 1.875 0 010 3.75H5.625a1.875 1.875 0 010-3.75z" />
          </svg>
        {:else if item.icon === 'live'}
          <svg class="w-[18px] h-[18px] shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.75">
            <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 13.5l10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75z" />
          </svg>
        {:else if item.icon === 'archive'}
          <svg class="w-[18px] h-[18px] shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.75">
            <path stroke-linecap="round" stroke-linejoin="round" d="M20.25 7.5l-.625 10.632a2.25 2.25 0 01-2.247 2.118H6.622a2.25 2.25 0 01-2.247-2.118L3.75 7.5M10 11.25h4M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z" />
          </svg>
        {:else if item.icon === 'profiles'}
          <svg class="w-[18px] h-[18px] shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.75">
            <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 6a3.75 3.75 0 11-7.5 0 3.75 3.75 0 017.5 0zM4.501 20.118a7.5 7.5 0 0114.998 0A17.933 17.933 0 0112 21.75c-2.676 0-5.216-.584-7.499-1.632z" />
          </svg>
        {:else}
          <svg class="w-[18px] h-[18px] shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.75">
            <path stroke-linecap="round" stroke-linejoin="round" d="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.324.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 011.37.49l1.296 2.247a1.125 1.125 0 01-.26 1.431l-1.003.827c-.293.24-.438.613-.431.992a6.759 6.759 0 010 .255c-.007.378.138.75.43.99l1.005.828c.424.35.534.954.26 1.43l-1.298 2.247a1.125 1.125 0 01-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.57 6.57 0 01-.22.128c-.331.183-.581.495-.644.869l-.213 1.28c-.09.543-.56.941-1.11.941h-2.594c-.55 0-1.02-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 01-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 01-1.369-.49l-1.297-2.247a1.125 1.125 0 01.26-1.431l1.004-.827c.292-.24.437-.613.43-.992a6.932 6.932 0 010-.255c.007-.378-.138-.75-.43-.99l-1.004-.828a1.125 1.125 0 01-.26-1.43l1.297-2.247a1.125 1.125 0 011.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.087.22-.128.332-.183.582-.495.644-.869l.214-1.281z" />
            <circle cx="12" cy="12" r="3" />
          </svg>
        {/if}
        <span class="font-medium">{item.label}</span>
        {#if item.icon === 'live'}
          <span class="ml-auto w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
        {/if}
      </button>
    {/each}
  </nav>

  <div class="p-4 border-t border-slate-700/50">
    <p class="text-[11px] text-slate-500 leading-relaxed truncate" title={dataDir}>
      {dataDir || '.'}
    </p>
    <p class="text-[11px] text-slate-500 mt-1">Monitoring MCP sessions</p>
  </div>
</aside>
