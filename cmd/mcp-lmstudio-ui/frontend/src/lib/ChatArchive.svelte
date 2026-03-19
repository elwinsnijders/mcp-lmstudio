<script>
  import { onMount } from 'svelte'
  import { ListSessions, LoadChatLog } from '../../wailsjs/go/main/App'

  export let sessionId = null

  let sessions = []
  let selected = sessionId
  let events = []
  let messages = []
  let loading = false
  let expanded = {}

  $: if (selected) {
    loadChat(selected)
  }

  onMount(async () => {
    try {
      const result = await ListSessions()
      sessions = (result || []).filter((s) => s.hasChatLog)
    } catch (_) {}

    if (sessionId) {
      selected = sessionId
    }
  })

  async function loadChat(id) {
    loading = true
    expanded = {}
    try {
      const result = await LoadChatLog(id)
      events = result || []
      messages = collapseEvents(events)
    } catch (e) {
      console.error('Failed to load chat:', e)
      events = []
      messages = []
    } finally {
      loading = false
    }
  }

  function collapseEvents(evts) {
    let msgs = []
    let pendingDelta = ''

    for (const ev of evts) {
      switch (ev.type) {
        case 'user_message':
          if (pendingDelta) {
            msgs.push({ role: 'assistant', content: pendingDelta, stats: null, ts: ev.ts })
            pendingDelta = ''
          }
          msgs.push({ role: 'user', content: ev.content, ts: ev.ts })
          break
        case 'ai_delta':
          pendingDelta += ev.content
          break
        case 'ai_complete':
          msgs.push({ role: 'assistant', content: ev.content || pendingDelta, stats: ev.stats, ts: ev.ts })
          pendingDelta = ''
          break
        case 'error':
          msgs.push({ role: 'error', content: ev.content, ts: ev.ts })
          pendingDelta = ''
          break
        case 'tool_use':
          msgs.push({ role: 'tool', content: ev.content, tool: ev.tool, ts: ev.ts })
          break
      }
    }

    if (pendingDelta) {
      msgs.push({ role: 'assistant', content: pendingDelta, stats: null, ts: '' })
    }

    return msgs
  }

  function formatTime(ts) {
    if (!ts) return ''
    try {
      const d = new Date(ts)
      return d.toLocaleTimeString('en-GB', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' })
    } catch (_) {
      return ''
    }
  }

  function formatStats(stats) {
    if (!stats) return ''
    const parts = []
    if (stats.input_tokens) parts.push(`Input: ${stats.input_tokens}`)
    if (stats.output_tokens) parts.push(`Output: ${stats.output_tokens}`)
    if (stats.tokens_per_sec) parts.push(`${stats.tokens_per_sec.toFixed(1)} tokens/s`)
    if (stats.time_to_first_sec) parts.push(`TTFT: ${stats.time_to_first_sec.toFixed(2)}s`)
    if (stats.response_id) parts.push(`ID: ${stats.response_id}`)
    return parts.join(' | ')
  }

  function toggleRaw(idx) {
    expanded[idx] = !expanded[idx]
    expanded = expanded
  }

  function getSessionInfo(id) {
    return sessions.find((s) => s.id === id)
  }

  $: sessionInfo = selected ? getSessionInfo(selected) : null
</script>

<div class="flex flex-col h-[calc(100vh-4rem)]">
  <div class="flex items-center justify-between mb-4 shrink-0">
    <div>
      <h1 class="text-2xl font-semibold text-gray-900">Chat Archive</h1>
      <p class="text-sm text-gray-500 mt-1">Browse full chat history for debugging</p>
    </div>
    <div class="flex items-center gap-3">
      <select
        bind:value={selected}
        class="px-3 py-1.5 text-xs font-mono border border-gray-300 rounded-md focus:ring-2 focus:ring-violet-500 focus:border-violet-500 outline-none"
      >
        <option value="">Select a session...</option>
        {#each sessions as s}
          <option value={s.id}>{s.id} [{s.status}] {s.profile || ''}</option>
        {/each}
      </select>
    </div>
  </div>

  {#if sessionInfo}
    <div class="mb-3 shrink-0 px-4 py-2.5 bg-slate-50 rounded-lg border border-slate-200 flex items-center gap-6 text-xs text-slate-600">
      <span><strong>Task:</strong> {sessionInfo.task?.slice(0, 100) || '-'}</span>
      <span><strong>Model:</strong> {sessionInfo.model || '-'}</span>
      <span><strong>Tokens:</strong> {sessionInfo.tokensUsed?.toLocaleString() || 0} / {sessionInfo.tokensMax?.toLocaleString() || 0}</span>
      <span><strong>Exchanges:</strong> {sessionInfo.exchanges || 0}</span>
    </div>
  {/if}

  <div class="flex-1 bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden flex flex-col min-h-0">
    <div class="flex-1 overflow-y-auto p-6 space-y-6">
      {#if !selected}
        <div class="flex items-center justify-center h-full text-gray-400">
          <div class="text-center">
            <svg class="w-12 h-12 text-gray-300 mx-auto mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1">
              <path stroke-linecap="round" stroke-linejoin="round" d="M20.25 7.5l-.625 10.632a2.25 2.25 0 01-2.247 2.118H6.622a2.25 2.25 0 01-2.247-2.118L3.75 7.5M10 11.25h4M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z" />
            </svg>
            <p class="font-medium text-sm">Select a session to view</p>
          </div>
        </div>
      {:else if loading}
        <div class="flex items-center justify-center h-full text-gray-400">
          <p>Loading chat...</p>
        </div>
      {:else if messages.length === 0}
        <div class="flex items-center justify-center h-full text-gray-400">
          <p class="text-sm">No messages in this chat log</p>
        </div>
      {:else}
        {#each messages as msg, i}
          <div class="group">
            <div class="flex items-center gap-2 mb-1.5">
              <span class="text-[10px] font-semibold uppercase tracking-wider
                {msg.role === 'user' ? 'text-violet-500' :
                 msg.role === 'assistant' ? 'text-emerald-600' :
                 msg.role === 'error' ? 'text-red-500' :
                 'text-amber-600'}">
                {msg.role === 'user' ? 'User' :
                 msg.role === 'assistant' ? 'AI' :
                 msg.role === 'error' ? 'Error' :
                 'Tool'}
              </span>
              <span class="text-[10px] text-gray-300">{formatTime(msg.ts)}</span>
              {#if msg.role === 'assistant'}
                <button
                  class="text-[10px] text-gray-300 hover:text-gray-500 opacity-0 group-hover:opacity-100 transition-opacity"
                  on:click={() => toggleRaw(i)}
                >{expanded[i] ? 'Hide stats' : 'Show stats'}</button>
              {/if}
            </div>

            {#if msg.role === 'user'}
              <div class="pl-3 border-l-2 border-violet-200">
                <pre class="whitespace-pre-wrap text-sm text-gray-800 font-sans leading-relaxed">{msg.content}</pre>
              </div>
            {:else if msg.role === 'assistant'}
              <div class="pl-3 border-l-2 border-emerald-200">
                <pre class="whitespace-pre-wrap text-sm text-gray-800 font-sans leading-relaxed">{msg.content}</pre>
              </div>
              {#if expanded[i] && msg.stats}
                <div class="mt-2 ml-3 px-3 py-2 bg-slate-50 rounded text-[11px] text-slate-500 font-mono">
                  {formatStats(msg.stats)}
                </div>
              {/if}
            {:else if msg.role === 'error'}
              <div class="pl-3 border-l-2 border-red-200">
                <div class="px-3 py-2 bg-red-50 rounded text-sm text-red-700">{msg.content}</div>
              </div>
            {:else if msg.role === 'tool'}
              <div class="pl-3 border-l-2 border-amber-200">
                <div class="px-3 py-2 bg-amber-50 rounded text-xs font-mono text-amber-700">
                  {msg.tool || 'tool'}: {msg.content}
                </div>
              </div>
            {/if}
          </div>
        {/each}
      {/if}
    </div>

    {#if messages.length > 0}
      <div class="border-t border-gray-100 px-4 py-2 flex items-center justify-between bg-gray-50/50">
        <span class="text-[11px] text-gray-400">{messages.length} messages | {events.length} raw events</span>
      </div>
    {/if}
  </div>
</div>
