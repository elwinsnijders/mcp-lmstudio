<script>
  import { onMount, onDestroy, createEventDispatcher } from 'svelte'
  import { ListSessions, ListGroups } from '../../wailsjs/go/main/App'

  const dispatch = createEventDispatcher()

  let sessions = []
  let groups = []
  let filter = ''
  let loading = true
  let interval
  let expandedGroups = {}

  $: groupMap = buildGroupMap(groups)
  $: grouped = buildGroupedView(sessions, groupMap, filter)

  function buildGroupMap(groups) {
    const map = {}
    for (const g of groups) {
      map[g.id] = g
    }
    return map
  }

  function buildGroupedView(sessions, groupMap, filter) {
    const lc = filter ? filter.toLowerCase() : ''
    const matchesFilter = (s) => {
      if (!lc) return true
      return (
        s.id?.toLowerCase().includes(lc) ||
        s.task?.toLowerCase().includes(lc) ||
        s.profile?.toLowerCase().includes(lc) ||
        s.model?.toLowerCase().includes(lc) ||
        s.status?.toLowerCase().includes(lc) ||
        s.groupId?.toLowerCase().includes(lc)
      )
    }

    const groupSessions = {}
    const ungrouped = []

    for (const s of sessions) {
      if (!matchesFilter(s)) continue
      if (s.groupId) {
        if (!groupSessions[s.groupId]) groupSessions[s.groupId] = []
        groupSessions[s.groupId].push(s)
      } else {
        ungrouped.push(s)
      }
    }

    const items = []

    const matchedGroupIds = new Set(Object.keys(groupSessions))
    const sortedGroups = Object.values(groupMap)
      .filter(g => matchedGroupIds.has(g.id) || (lc && g.type?.toLowerCase().includes(lc)))
      .sort((a, b) => b.updatedAt.localeCompare(a.updatedAt))

    for (const g of sortedGroups) {
      const gSessions = (groupSessions[g.id] || []).sort((a, b) => (a.groupStep || 0) - (b.groupStep || 0))
      items.push({ type: 'group', group: g, sessions: gSessions })
    }

    for (const s of ungrouped) {
      items.push({ type: 'session', session: s })
    }

    return items
  }

  async function refresh() {
    try {
      const [sessResult, grpResult] = await Promise.all([ListSessions(), ListGroups()])
      sessions = sessResult || []
      groups = grpResult || []
    } catch (e) {
      console.error('Failed to load sessions:', e)
    } finally {
      loading = false
    }
  }

  onMount(() => {
    refresh()
    interval = setInterval(refresh, 3000)
  })

  onDestroy(() => {
    if (interval) clearInterval(interval)
  })

  function toggleGroup(id) {
    expandedGroups[id] = !expandedGroups[id]
    expandedGroups = expandedGroups
  }

  function statusColor(status) {
    if (status === 'active') return 'text-emerald-700 bg-emerald-50 border-emerald-200'
    if (status === 'completed') return 'text-slate-600 bg-slate-50 border-slate-200'
    if (status === 'paused') return 'text-amber-700 bg-amber-50 border-amber-200'
    if (status === 'token_limit') return 'text-red-700 bg-red-50 border-red-200'
    if (status === 'running') return 'text-emerald-700 bg-emerald-50 border-emerald-200'
    if (status === 'failed') return 'text-red-700 bg-red-50 border-red-200'
    return 'text-gray-600 bg-gray-50 border-gray-200'
  }

  function groupTypeColor(type) {
    if (type === 'queue') return 'text-indigo-700 bg-indigo-50 border-indigo-200'
    if (type === 'chain') return 'text-teal-700 bg-teal-50 border-teal-200'
    if (type === 'loop') return 'text-orange-700 bg-orange-50 border-orange-200'
    return 'text-gray-600 bg-gray-50 border-gray-200'
  }

  function groupTypeLabel(type) {
    if (type === 'queue') return 'Queue'
    if (type === 'chain') return 'Chain'
    if (type === 'loop') return 'Loop'
    return type
  }

  function tokenBarColor(pct) {
    if (pct >= 95) return 'bg-red-500'
    if (pct >= 80) return 'bg-amber-500'
    return 'bg-violet-500'
  }

  function formatTime(ts) {
    if (!ts) return ''
    try {
      const d = new Date(ts)
      return d.toLocaleString('en-GB', {
        month: 'short', day: 'numeric',
        hour: '2-digit', minute: '2-digit',
        hour12: false,
      })
    } catch (_) {
      return ts
    }
  }

  function truncateTask(task, max = 80) {
    if (!task) return ''
    return task.length > max ? task.slice(0, max) + '...' : task
  }

  $: totalCount = sessions.length
</script>

<div class="flex flex-col h-[calc(100vh-4rem)]">
  <div class="flex items-center justify-between mb-4 shrink-0">
    <div>
      <h1 class="text-2xl font-semibold text-gray-900">Sessions</h1>
      <p class="text-sm text-gray-500 mt-1">All worker AI sessions (auto-refreshes every 3s)</p>
    </div>
    <div class="flex items-center gap-3">
      <span class="text-xs text-gray-400">{totalCount} sessions</span>
      <button
        class="px-3 py-1.5 text-xs font-medium text-gray-600 bg-gray-100 rounded-md hover:bg-gray-200 transition-colors"
        on:click={refresh}
      >Refresh</button>
    </div>
  </div>

  <div class="mb-3 shrink-0">
    <input
      type="text"
      placeholder="Filter by ID, task, profile, model, status, group..."
      bind:value={filter}
      class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-violet-500 focus:border-violet-500 outline-none transition"
    />
  </div>

  <div class="flex-1 bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden flex flex-col min-h-0">
    <div class="bg-gray-50 border-b border-gray-200 px-4 py-2 grid grid-cols-[120px_70px_100px_1fr_110px_90px_90px] gap-2 text-xs font-semibold text-gray-500 uppercase tracking-wider shrink-0">
      <span>Session ID</span>
      <span>Status</span>
      <span>Profile</span>
      <span>Task</span>
      <span>Tokens</span>
      <span class="text-right">Last Active</span>
      <span class="text-right">Actions</span>
    </div>

    <div class="flex-1 overflow-y-auto text-sm">
      {#if loading}
        <div class="p-12 text-center text-gray-400">
          <p>Loading sessions...</p>
        </div>
      {:else if grouped.length === 0}
        <div class="p-12 text-center text-gray-400">
          <svg class="w-12 h-12 text-gray-300 mx-auto mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1">
            <path stroke-linecap="round" stroke-linejoin="round" d="M20.25 8.511c.884.284 1.5 1.128 1.5 2.097v4.286c0 1.136-.847 2.1-1.98 2.193-.34.027-.68.052-1.02.072v3.091l-3-3c-1.354 0-2.694-.055-4.02-.163a2.115 2.115 0 01-.825-.242m9.345-8.334a2.126 2.126 0 00-.476-.095 48.64 48.64 0 00-8.048 0c-1.131.094-1.976 1.057-1.976 2.192v4.286c0 .837.46 1.58 1.155 1.951m9.345-8.334V6.637c0-1.621-1.152-3.026-2.76-3.235A48.455 48.455 0 0011.25 3c-2.115 0-4.198.137-6.24.402-1.608.209-2.76 1.614-2.76 3.235v6.226c0 1.621 1.152 3.026 2.76 3.235.577.075 1.157.14 1.74.194V21l4.155-4.155" />
          </svg>
          <p class="font-medium">No sessions found</p>
          <p class="text-xs text-gray-400 mt-1">Sessions will appear when the MCP server processes start_task calls</p>
        </div>
      {:else}
        {#each grouped as item}
          {#if item.type === 'group'}
            {@const g = item.group}
            {@const isExpanded = expandedGroups[g.id] !== false}
            {@const pct = g.totalSteps > 0 ? (g.currentStep / g.totalSteps) * 100 : 0}
            <button
              class="w-full px-4 py-2.5 flex items-center gap-3 bg-gray-50/80 border-b border-gray-200 hover:bg-gray-100/80 transition-colors text-left cursor-pointer"
              on:click={() => toggleGroup(g.id)}
            >
              <svg class="w-3.5 h-3.5 text-gray-400 transition-transform {isExpanded ? 'rotate-90' : ''}" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" />
              </svg>
              <span class="inline-flex px-2 py-0.5 rounded border text-[10px] font-bold uppercase tracking-wider {groupTypeColor(g.type)}">
                {groupTypeLabel(g.type)}
              </span>
              <span class="inline-flex px-1.5 py-0.5 rounded border text-[10px] font-semibold {statusColor(g.status)}">
                {g.status}
              </span>
              <span class="text-xs text-gray-600 font-medium">
                Step {g.currentStep}/{g.totalSteps}
              </span>
              <div class="w-20 h-1.5 bg-gray-200 rounded-full overflow-hidden">
                <div class="h-full rounded-full transition-all {g.status === 'failed' ? 'bg-red-500' : g.status === 'completed' ? 'bg-emerald-500' : 'bg-violet-500'}" style="width: {Math.min(pct, 100)}%"></div>
              </div>
              {#if g.chainMode}
                <span class="text-[10px] text-gray-400">mode: {g.chainMode}</span>
              {/if}
              {#if g.stoppedEarly}
                <span class="text-[10px] text-orange-500 font-medium">stopped early</span>
              {/if}
              <span class="ml-auto text-[10px] text-gray-400">{item.sessions.length} sessions</span>
              <span class="text-xs text-gray-400">{formatTime(g.updatedAt)}</span>
            </button>

            {#if isExpanded}
              {#each item.sessions as session (session.id)}
                <div class="px-4 py-2.5 grid grid-cols-[120px_70px_100px_1fr_110px_90px_90px] gap-2 items-center border-b border-gray-50 hover:bg-gray-50/70 transition-colors pl-12">
                  <span class="font-mono text-xs text-gray-700 truncate flex items-center gap-1.5" title={session.id}>
                    <span class="text-[10px] text-gray-400 font-semibold shrink-0">#{session.groupStep}</span>
                    {session.id}
                  </span>
                  <span>
                    <span class="inline-flex px-1.5 py-0.5 rounded border text-[10px] font-semibold {statusColor(session.status)}">
                      {session.status}
                    </span>
                  </span>
                  <span class="text-gray-600 truncate text-xs" title={session.profile}>
                    {session.profile || '-'}
                  </span>
                  <span class="text-gray-600 truncate text-xs" title={session.task}>
                    {truncateTask(session.task)}
                  </span>
                  <span class="flex items-center gap-2">
                    <div class="flex-1 h-1.5 bg-gray-100 rounded-full overflow-hidden">
                      <div class="h-full rounded-full transition-all {tokenBarColor(session.tokensPercent)}" style="width: {Math.min(session.tokensPercent, 100)}%"></div>
                    </div>
                    <span class="text-[10px] text-gray-400 whitespace-nowrap">{session.tokensPercent.toFixed(0)}%</span>
                  </span>
                  <span class="text-right text-xs text-gray-400">
                    {formatTime(session.lastActiveAt)}
                  </span>
                  <span class="text-right flex justify-end gap-1">
                    {#if session.hasChatLog}
                      <button
                        class="px-2 py-1 text-[10px] font-medium text-violet-600 bg-violet-50 rounded hover:bg-violet-100 transition-colors"
                        on:click|stopPropagation={() => dispatch('viewChat', session.id)}
                        title="View chat log"
                      >Chat</button>
                    {/if}
                    {#if session.status === 'active'}
                      <button
                        class="px-2 py-1 text-[10px] font-medium text-emerald-600 bg-emerald-50 rounded hover:bg-emerald-100 transition-colors"
                        on:click|stopPropagation={() => dispatch('watchLive', session.id)}
                        title="Watch live"
                      >Live</button>
                    {/if}
                  </span>
                </div>
              {/each}
            {/if}

          {:else}
            {@const session = item.session}
            <div class="px-4 py-3 grid grid-cols-[120px_70px_100px_1fr_110px_90px_90px] gap-2 items-center border-b border-gray-50 hover:bg-gray-50/70 transition-colors">
              <span class="font-mono text-xs text-gray-700 truncate" title={session.id}>
                {session.id}
              </span>
              <span>
                <span class="inline-flex px-1.5 py-0.5 rounded border text-[10px] font-semibold {statusColor(session.status)}">
                  {session.status}
                </span>
              </span>
              <span class="text-gray-600 truncate text-xs" title={session.profile}>
                {session.profile || '-'}
              </span>
              <span class="text-gray-600 truncate text-xs" title={session.task}>
                {truncateTask(session.task)}
              </span>
              <span class="flex items-center gap-2">
                <div class="flex-1 h-1.5 bg-gray-100 rounded-full overflow-hidden">
                  <div class="h-full rounded-full transition-all {tokenBarColor(session.tokensPercent)}" style="width: {Math.min(session.tokensPercent, 100)}%"></div>
                </div>
                <span class="text-[10px] text-gray-400 whitespace-nowrap">{session.tokensPercent.toFixed(0)}%</span>
              </span>
              <span class="text-right text-xs text-gray-400">
                {formatTime(session.lastActiveAt)}
              </span>
              <span class="text-right flex justify-end gap-1">
                {#if session.hasChatLog}
                  <button
                    class="px-2 py-1 text-[10px] font-medium text-violet-600 bg-violet-50 rounded hover:bg-violet-100 transition-colors"
                    on:click={() => dispatch('viewChat', session.id)}
                    title="View chat log"
                  >Chat</button>
                {/if}
                {#if session.status === 'active'}
                  <button
                    class="px-2 py-1 text-[10px] font-medium text-emerald-600 bg-emerald-50 rounded hover:bg-emerald-100 transition-colors"
                    on:click={() => dispatch('watchLive', session.id)}
                    title="Watch live"
                  >Live</button>
                {/if}
              </span>
            </div>
          {/if}
        {/each}
      {/if}
    </div>
  </div>
</div>
