<script>
  import { onMount, onDestroy, tick } from 'svelte'
  import { GetActiveSessions, LoadChatLog, StartChatWatch, StopChatWatch } from '../../wailsjs/go/main/App'
  import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'

  let activeSessions = []
  let selectedSession = ''
  let messages = []
  let streamBuffer = ''
  let autoScroll = true
  let chatContainer
  let refreshInterval

  $: if (selectedSession) {
    loadSession(selectedSession)
  }

  onMount(async () => {
    await refreshSessions()
    refreshInterval = setInterval(refreshSessions, 5000)
    EventsOn('chat:event', onChatEvent)
  })

  onDestroy(() => {
    EventsOff('chat:event')
    StopChatWatch().catch(() => {})
    if (refreshInterval) clearInterval(refreshInterval)
  })

  async function refreshSessions() {
    try {
      const ids = await GetActiveSessions()
      activeSessions = ids || []
      if (activeSessions.length > 0 && !selectedSession) {
        selectedSession = activeSessions[0]
      }
    } catch (_) {}
  }

  async function loadSession(sessionId) {
    try {
      await StopChatWatch()
    } catch (_) {}

    messages = []
    streamBuffer = ''

    try {
      const events = await LoadChatLog(sessionId)
      if (events) {
        processHistoricEvents(events)
      }
    } catch (_) {}

    StartChatWatch(sessionId)
    await scrollToBottom()
  }

  function processHistoricEvents(events) {
    let msgs = []
    let pendingAI = ''

    for (const ev of events) {
      switch (ev.type) {
        case 'user_message':
          if (pendingAI) {
            msgs.push({ role: 'assistant', content: pendingAI, stats: null })
            pendingAI = ''
          }
          msgs.push({ role: 'user', content: ev.content })
          break
        case 'ai_delta':
          pendingAI += ev.content
          break
        case 'ai_complete':
          msgs.push({ role: 'assistant', content: ev.content || pendingAI, stats: ev.stats })
          pendingAI = ''
          break
        case 'error':
          msgs.push({ role: 'error', content: ev.content })
          pendingAI = ''
          break
        case 'tool_use':
          msgs.push({ role: 'tool', content: ev.content, tool: ev.tool })
          break
      }
    }

    if (pendingAI) {
      streamBuffer = pendingAI
    }

    messages = msgs
  }

  async function onChatEvent(event) {
    switch (event.type) {
      case 'user_message':
        if (streamBuffer) {
          messages = [...messages, { role: 'assistant', content: streamBuffer, stats: null }]
          streamBuffer = ''
        }
        messages = [...messages, { role: 'user', content: event.content }]
        break
      case 'ai_delta':
        streamBuffer += event.content
        streamBuffer = streamBuffer
        break
      case 'ai_complete':
        messages = [...messages, { role: 'assistant', content: event.content || streamBuffer, stats: event.stats }]
        streamBuffer = ''
        break
      case 'error':
        messages = [...messages, { role: 'error', content: event.content }]
        streamBuffer = ''
        break
      case 'tool_use':
        messages = [...messages, { role: 'tool', content: event.content, tool: event.tool }]
        break
    }

    if (autoScroll) {
      await scrollToBottom()
    }
  }

  async function scrollToBottom() {
    await tick()
    if (chatContainer) {
      chatContainer.scrollTop = chatContainer.scrollHeight
    }
  }

  function handleScroll() {
    if (!chatContainer) return
    const threshold = 100
    const atBottom = chatContainer.scrollHeight - chatContainer.scrollTop - chatContainer.clientHeight < threshold
    autoScroll = atBottom
  }

  function formatStats(stats) {
    if (!stats) return ''
    const parts = []
    if (stats.input_tokens) parts.push(`in: ${stats.input_tokens}`)
    if (stats.output_tokens) parts.push(`out: ${stats.output_tokens}`)
    if (stats.tokens_per_sec) parts.push(`${stats.tokens_per_sec.toFixed(1)} t/s`)
    return parts.join(' | ')
  }
</script>

<div class="flex flex-col h-[calc(100vh-4rem)]">
  <div class="flex items-center justify-between mb-4 shrink-0">
    <div>
      <h1 class="text-2xl font-semibold text-gray-900">Live View</h1>
      <p class="text-sm text-gray-500 mt-1">Watch active AI sessions stream in real-time</p>
    </div>
    <div class="flex items-center gap-3">
      {#if activeSessions.length > 0}
        <select
          bind:value={selectedSession}
          class="px-3 py-1.5 text-xs font-mono border border-gray-300 rounded-md focus:ring-2 focus:ring-violet-500 focus:border-violet-500 outline-none"
        >
          {#each activeSessions as id}
            <option value={id}>{id}</option>
          {/each}
        </select>
      {/if}
      <div class="flex items-center gap-1.5">
        <span class="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
        <span class="text-xs text-gray-500">Streaming</span>
      </div>
    </div>
  </div>

  <div
    class="flex-1 bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden flex flex-col min-h-0"
  >
    <div
      bind:this={chatContainer}
      on:scroll={handleScroll}
      class="flex-1 overflow-y-auto p-4 space-y-4"
    >
      {#if messages.length === 0 && !streamBuffer}
        <div class="flex-1 flex items-center justify-center h-full">
          <div class="text-center text-gray-400">
            <svg class="w-16 h-16 text-gray-200 mx-auto mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1">
              <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 13.5l10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75z" />
            </svg>
            <p class="font-medium text-sm">
              {#if activeSessions.length === 0}
                No active sessions
              {:else}
                Waiting for activity...
              {/if}
            </p>
            <p class="text-xs mt-1">
              {#if activeSessions.length === 0}
                Start a session via the MCP bridge to see live output
              {:else}
                Select a session above and watch the AI respond in real-time
              {/if}
            </p>
          </div>
        </div>
      {:else}
        {#each messages as msg, i}
          {#if msg.role === 'user'}
            <div class="flex justify-end">
              <div class="max-w-[80%] px-4 py-3 rounded-2xl rounded-br-md bg-violet-600 text-white text-sm">
                <pre class="whitespace-pre-wrap font-sans">{msg.content}</pre>
              </div>
            </div>
          {:else if msg.role === 'assistant'}
            <div class="flex justify-start">
              <div class="max-w-[80%]">
                <div class="px-4 py-3 rounded-2xl rounded-bl-md bg-gray-100 text-gray-800 text-sm">
                  <pre class="whitespace-pre-wrap font-sans leading-relaxed">{msg.content}</pre>
                </div>
                {#if msg.stats}
                  <div class="mt-1 text-[10px] text-gray-400 px-2">
                    {formatStats(msg.stats)}
                  </div>
                {/if}
              </div>
            </div>
          {:else if msg.role === 'error'}
            <div class="flex justify-center">
              <div class="px-4 py-2 rounded-lg bg-red-50 border border-red-200 text-red-700 text-xs">
                {msg.content}
              </div>
            </div>
          {:else if msg.role === 'tool'}
            <div class="flex justify-center">
              <div class="px-3 py-1.5 rounded-lg bg-amber-50 border border-amber-200 text-amber-700 text-[11px] font-mono">
                Tool: {msg.tool || 'unknown'} {msg.content ? '- ' + msg.content : ''}
              </div>
            </div>
          {/if}
        {/each}

        {#if streamBuffer}
          <div class="flex justify-start">
            <div class="max-w-[80%]">
              <div class="px-4 py-3 rounded-2xl rounded-bl-md bg-gray-100 text-gray-800 text-sm">
                <pre class="whitespace-pre-wrap font-sans leading-relaxed">{streamBuffer}</pre>
                <span class="inline-block w-2 h-4 bg-violet-500 animate-pulse ml-0.5 align-text-bottom"></span>
              </div>
            </div>
          </div>
        {/if}
      {/if}
    </div>

    {#if !autoScroll && (messages.length > 0 || streamBuffer)}
      <div class="border-t border-gray-100 px-4 py-2 text-center">
        <button
          class="text-xs text-violet-600 hover:text-violet-700 font-medium"
          on:click={scrollToBottom}
        >
          Scroll to bottom
        </button>
      </div>
    {/if}
  </div>
</div>
