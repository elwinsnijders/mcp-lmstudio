<script>
  import { onMount, onDestroy, tick } from 'svelte'
  import { GetActiveSessions, LoadChatLog, StartChatWatch, StopChatWatch } from '../../wailsjs/go/main/App'
  import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
  import { marked } from 'marked'

  marked.setOptions({ breaks: true, gfm: true })

  function md(text) {
    if (!text) return ''
    return marked.parse(text)
  }

  function mkAssistant(content, stats) {
    return { role: 'assistant', content, html: md(content), stats }
  }

  let activeSessions = []
  let selectedSession = ''
  let messages = []
  let streamBuffer = ''
  let reasoningBuffer = ''
  let isReasoning = false
  let statusPhase = ''
  let statusProgress = 0
  let autoScroll = true
  let chatContainer
  let refreshInterval

  let _rawStream = ''
  let _rawReasoning = ''
  let _streamPos = 0
  let _reasonPos = 0
  let _typeTimer = null

  const TYPE_INTERVAL = 16
  const BASE_CHARS = 2
  const CATCHUP_DIVISOR = 8

  /** Safety cap for tool args/output in UI (full data is in artifacts / tail chatlog). */
  const TOOL_PREVIEW = 8192
  /** Cap very large assistant bubbles from old logs before markdown. */
  const ASSISTANT_PREVIEW = 128000

  function capPreview(s, max = TOOL_PREVIEW) {
    if (!s || s.length <= max) return s
    return s.slice(0, max) + '\n... (truncated)'
  }

  let loadDebounceTimer = null

  function startTypewriter() {
    if (_typeTimer) return
    _typeTimer = setInterval(typewriterTick, TYPE_INTERVAL)
  }

  function stopTypewriter() {
    if (_typeTimer) { clearInterval(_typeTimer); _typeTimer = null }
  }

  function typewriterTick() {
    let changed = false

    if (_streamPos < _rawStream.length) {
      const backlog = _rawStream.length - _streamPos
      const step = Math.max(BASE_CHARS, Math.ceil(backlog / CATCHUP_DIVISOR))
      _streamPos = Math.min(_streamPos + step, _rawStream.length)
      streamBuffer = _rawStream.slice(0, _streamPos)
      changed = true
    }

    if (_reasonPos < _rawReasoning.length) {
      const backlog = _rawReasoning.length - _reasonPos
      const step = Math.max(BASE_CHARS, Math.ceil(backlog / CATCHUP_DIVISOR))
      _reasonPos = Math.min(_reasonPos + step, _rawReasoning.length)
      reasoningBuffer = _rawReasoning.slice(0, _reasonPos)
      changed = true
    }

    if (changed && autoScroll) {
      scrollToBottom()
    }

    if (_streamPos >= _rawStream.length && _reasonPos >= _rawReasoning.length) {
      stopTypewriter()
    }
  }

  $: if (selectedSession) {
    const sid = selectedSession
    clearTimeout(loadDebounceTimer)
    loadDebounceTimer = setTimeout(() => {
      loadSession(sid)
    }, 75)
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
    if (loadDebounceTimer) clearTimeout(loadDebounceTimer)
    stopTypewriter()
  })

  async function refreshSessions() {
    try {
      const prev = new Set(activeSessions)
      const ids = await GetActiveSessions()
      activeSessions = ids || []
      if (activeSessions.length > 0) {
        const newest = activeSessions[0]
        if (!selectedSession || !prev.has(newest)) {
          selectedSession = newest
        }
      }
    } catch (_) {}
  }

  async function loadSession(sessionId) {
    const loadId = sessionId
    try {
      await StopChatWatch()
    } catch (_) {}

    stopTypewriter()
    messages = []
    _rawStream = ''
    _rawReasoning = ''
    _streamPos = 0
    _reasonPos = 0
    streamBuffer = ''
    reasoningBuffer = ''
    isReasoning = false
    statusPhase = ''
    statusProgress = 0

    try {
      const events = await LoadChatLog(sessionId)
      if (loadId !== selectedSession) return
      if (events) {
        processHistoricEvents(events)
      }
    } catch (_) {}

    if (loadId !== selectedSession) return
    StartChatWatch(sessionId)
    await scrollToBottom()
  }

  function processHistoricEvents(events) {
    let msgs = []
    let pendingAI = ''
    let pendingReasoning = ''

    for (const ev of events) {
      switch (ev.type) {
        case 'user_message':
          if (pendingAI.trim()) {
            msgs.push(mkAssistant(pendingAI.trim(), null))
          }
          pendingAI = ''
          pendingReasoning = ''
          msgs.push({ role: 'user', content: ev.content })
          break
        case 'reasoning_start':
          if (pendingAI.trim()) {
            msgs.push(mkAssistant(pendingAI.trim(), null))
          }
          pendingAI = ''
          pendingReasoning = ''
          break
        case 'reasoning_delta':
          pendingReasoning += ev.content || ''
          break
        case 'reasoning_end':
          if (pendingReasoning) {
            msgs.push({ role: 'reasoning', content: pendingReasoning })
            pendingReasoning = ''
          }
          break
        case 'ai_delta':
          pendingAI += ev.content
          break
        case 'ai_complete': {
          let c = ev.content || pendingAI
          if (c && c.length > ASSISTANT_PREVIEW) {
            c = c.slice(0, ASSISTANT_PREVIEW) + '\n... (truncated)'
          }
          msgs.push(mkAssistant(c, ev.stats))
          pendingAI = ''
          break
        }
        case 'error':
          msgs.push({ role: 'error', content: ev.content })
          pendingAI = ''
          break
        case 'tool_use':
          if (pendingAI.trim()) {
            msgs.push(mkAssistant(pendingAI.trim(), null))
          }
          pendingAI = ''
          msgs.push({ role: 'tool', content: ev.content, tool: ev.tool })
          break
        case 'tool_call_start':
          if (pendingAI.trim()) {
            msgs.push(mkAssistant(pendingAI.trim(), null))
          }
          pendingAI = ''
          msgs.push({ role: 'tool_start', tool: ev.tool })
          break
        case 'tool_call_result':
          msgs.push({
            role: 'tool_result',
            tool: ev.tool,
            arguments: capPreview(ev.arguments, TOOL_PREVIEW),
            output: capPreview(ev.output, TOOL_PREVIEW),
            success: ev.success,
            reason: ev.reason
          })
          break
        case 'status':
          break
      }
    }

    if (pendingReasoning) {
      _rawReasoning = pendingReasoning
      _reasonPos = pendingReasoning.length
      reasoningBuffer = pendingReasoning
      isReasoning = true
    }
    if (pendingAI) {
      _rawStream = pendingAI
      _streamPos = pendingAI.length
      streamBuffer = pendingAI
    }

    messages = msgs
  }

  async function onChatEvent(event) {
    switch (event.type) {
      case 'user_message':
        flushBuffers()
        messages = [...messages, { role: 'user', content: event.content }]
        statusPhase = ''
        break

      case 'status':
        statusPhase = event.phase || ''
        statusProgress = event.progress ?? 0
        break

      case 'reasoning_start':
        flushStream()
        _rawReasoning = ''
        _reasonPos = 0
        reasoningBuffer = ''
        isReasoning = true
        break
      case 'reasoning_delta':
        _rawReasoning += event.content || ''
        startTypewriter()
        return
      case 'reasoning_end':
        stopTypewriter()
        if (_rawReasoning) {
          messages = [...messages, { role: 'reasoning', content: _rawReasoning }]
        }
        _rawReasoning = ''
        _reasonPos = 0
        reasoningBuffer = ''
        isReasoning = false
        break

      case 'ai_delta':
        statusPhase = ''
        _rawStream += event.content
        startTypewriter()
        return
      case 'ai_complete':
        stopTypewriter()
        isReasoning = false
        _rawReasoning = ''
        _reasonPos = 0
        reasoningBuffer = ''
        statusPhase = ''
        messages = [...messages, mkAssistant(event.content || _rawStream, event.stats)]
        _rawStream = ''
        _streamPos = 0
        streamBuffer = ''
        break

      case 'error':
        stopTypewriter()
        messages = [...messages, { role: 'error', content: event.content }]
        _rawStream = ''
        _rawReasoning = ''
        _streamPos = 0
        _reasonPos = 0
        streamBuffer = ''
        isReasoning = false
        reasoningBuffer = ''
        statusPhase = ''
        break

      case 'tool_use':
        flushStream()
        messages = [...messages, { role: 'tool', content: event.content, tool: event.tool }]
        break
      case 'tool_call_start':
        flushStream()
        messages = [...messages, { role: 'tool_start', tool: event.tool }]
        break
      case 'tool_call_result': {
        const filtered = messages.filter((m, i) =>
          !(m.role === 'tool_start' && i === messages.length - 1) &&
          !(m.role === 'tool_start' && !m.tool)
        )
        messages = [...filtered, {
          role: 'tool_result',
          tool: event.tool,
          arguments: capPreview(event.arguments, TOOL_PREVIEW),
          output: capPreview(event.output, TOOL_PREVIEW),
          success: event.success,
          reason: event.reason
        }]
        break
      }
    }

    if (autoScroll) {
      await scrollToBottom()
    }
  }

  function flushStream() {
    const text = (_rawStream || streamBuffer || '').trim()
    stopTypewriter()
    if (text) {
      messages = [...messages, mkAssistant(text, null)]
    }
    _rawStream = ''
    _streamPos = 0
    streamBuffer = ''
  }

  function flushBuffers() {
    stopTypewriter()
    if (_rawReasoning || reasoningBuffer) {
      messages = [...messages, { role: 'reasoning', content: _rawReasoning || reasoningBuffer }]
      _rawReasoning = ''
      _reasonPos = 0
      reasoningBuffer = ''
      isReasoning = false
    }
    flushStream()
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

  const PREVIEW_LINES = 5
  const PREVIEW_CHARS = 500

  function previewLines(s) {
    if (!s) return { preview: '', full: '', needsExpand: false }
    const lines = s.split('\n')
    if (lines.length <= PREVIEW_LINES && s.length <= PREVIEW_CHARS) {
      return { preview: s, full: s, needsExpand: false }
    }
    const lineCut = lines.slice(0, PREVIEW_LINES).join('\n')
    const preview = lineCut.length > PREVIEW_CHARS ? lineCut.slice(0, PREVIEW_CHARS) + '...' : lineCut
    return { preview, full: s, needsExpand: true }
  }

  let expandedTools = {}

  function statusLabel(phase) {
    if (phase === 'prompt_processing') return 'Processing prompt'
    if (phase === 'model_load') return 'Loading model'
    return phase
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
      class="flex-1 overflow-y-auto p-4 space-y-3"
    >
      {#if messages.length === 0 && !streamBuffer && !reasoningBuffer && !statusPhase}
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
              <div class="max-w-[80%]">
                <div class="text-[10px] font-semibold uppercase tracking-wider text-violet-400 text-right mb-1">User</div>
                <div class="px-4 py-3 rounded-2xl rounded-br-md bg-violet-600 text-white text-sm">
                  <pre class="whitespace-pre-wrap font-sans">{msg.content}</pre>
                </div>
              </div>
            </div>

          {:else if msg.role === 'reasoning'}
            <div class="flex justify-start">
              <div class="max-w-[85%]">
                <div class="text-[10px] font-semibold uppercase tracking-wider text-blue-400 flex items-center gap-1.5 mb-1">
                  <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z"/></svg>
                  Reasoning
                </div>
                <div class="px-3 py-2 rounded-xl bg-blue-50 border border-blue-100 text-blue-800 text-xs leading-relaxed">
                  <pre class="whitespace-pre-wrap font-sans">{msg.content}</pre>
                </div>
              </div>
            </div>

          {:else if msg.role === 'assistant'}
            <div class="flex justify-start">
              <div class="max-w-[80%]">
                <div class="text-[10px] font-semibold uppercase tracking-wider text-emerald-500 mb-1">Agent</div>
                <div class="px-4 py-3 rounded-2xl rounded-bl-md bg-gray-100 text-gray-800 text-sm prose prose-sm prose-gray max-w-none">
                  {@html msg.html || md(msg.content)}
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

          {:else if msg.role === 'tool_start'}
            <div class="flex justify-center">
              <div class="px-3 py-1.5 rounded-lg bg-amber-50 border border-amber-100 text-amber-600 text-[11px] font-mono flex items-center gap-1.5">
                <span class="w-1.5 h-1.5 rounded-full bg-amber-400 animate-pulse"></span>
                {#if msg.tool}
                  Calling: {msg.tool}
                {:else}
                  <svg class="w-3 h-3 animate-spin" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path></svg>
                  Preparing tool call...
                {/if}
              </div>
            </div>

          {:else if msg.role === 'tool_result'}
            {@const argsP = previewLines(msg.arguments)}
            {@const outP = previewLines(msg.output)}
            {@const key = `${i}`}
            <div class="flex justify-center">
              <div class="max-w-[85%] w-full">
                <div class="text-[11px] font-mono flex items-center gap-1.5 py-1 {msg.success ? 'text-emerald-600' : 'text-red-600'}">
                  <span class="w-1.5 h-1.5 rounded-full {msg.success ? 'bg-emerald-400' : 'bg-red-400'}"></span>
                  {msg.tool}: {msg.success ? 'success' : 'failed'}
                  {#if msg.reason}
                    <span class="text-red-500 font-normal">({msg.reason})</span>
                  {/if}
                </div>
                <div class="rounded-lg border text-[11px] font-mono overflow-hidden {msg.success ? 'border-emerald-200 bg-emerald-50' : 'border-red-200 bg-red-50'}">
                  {#if msg.arguments}
                    <div class="px-3 py-1.5 border-b {msg.success ? 'border-emerald-200' : 'border-red-200'}">
                      <div class="text-gray-500 mb-0.5">args:</div>
                      <pre class="whitespace-pre-wrap text-gray-700">{expandedTools[key + '_args'] ? argsP.full : argsP.preview}</pre>
                      {#if argsP.needsExpand}
                        <button class="text-[10px] text-violet-500 hover:text-violet-700 mt-1" on:click={() => { expandedTools[key + '_args'] = !expandedTools[key + '_args']; expandedTools = expandedTools }}>
                          {expandedTools[key + '_args'] ? 'Show less' : 'Show more...'}
                        </button>
                      {/if}
                    </div>
                  {/if}
                  {#if msg.output}
                    <div class="px-3 py-1.5">
                      <div class="text-gray-500 mb-0.5">output:</div>
                      <pre class="whitespace-pre-wrap text-gray-700">{expandedTools[key + '_out'] ? outP.full : outP.preview}</pre>
                      {#if outP.needsExpand}
                        <button class="text-[10px] text-violet-500 hover:text-violet-700 mt-1" on:click={() => { expandedTools[key + '_out'] = !expandedTools[key + '_out']; expandedTools = expandedTools }}>
                          {expandedTools[key + '_out'] ? 'Show less' : 'Show more...'}
                        </button>
                      {/if}
                    </div>
                  {/if}
                </div>
              </div>
            </div>
          {/if}
        {/each}

        <!-- Live status indicator -->
        {#if statusPhase}
          <div class="flex justify-center">
            <div class="px-4 py-2 rounded-lg bg-indigo-50 border border-indigo-100 text-indigo-600 text-xs flex items-center gap-2">
              <svg class="w-3.5 h-3.5 animate-spin" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path></svg>
              {statusLabel(statusPhase)}
              {#if statusProgress > 0 && statusProgress < 1}
                <div class="w-20 h-1.5 bg-indigo-100 rounded-full overflow-hidden">
                  <div class="h-full bg-indigo-500 rounded-full transition-all" style="width: {statusProgress * 100}%"></div>
                </div>
              {/if}
            </div>
          </div>
        {/if}

        <!-- Live reasoning stream -->
        {#if isReasoning && reasoningBuffer}
          <div class="flex justify-start">
            <div class="max-w-[85%]">
              <div class="text-xs text-blue-500 font-medium flex items-center gap-1.5 mb-1">
                <svg class="w-3.5 h-3.5 animate-pulse" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z"/></svg>
                Thinking...
              </div>
              <div class="px-3 py-2 rounded-xl bg-blue-50 border border-blue-100 text-blue-800 text-xs leading-relaxed">
                <pre class="whitespace-pre-wrap font-sans">{reasoningBuffer}</pre>
                <span class="inline-block w-1.5 h-3 bg-blue-400 animate-pulse ml-0.5 align-text-bottom"></span>
              </div>
            </div>
          </div>
        {/if}

        <!-- Live message stream (plain text for smooth flow, markdown on complete) -->
        {#if streamBuffer}
          <div class="flex justify-start">
            <div class="max-w-[80%]">
              <div class="text-[10px] font-semibold uppercase tracking-wider text-emerald-500 mb-1">Agent</div>
              <div class="px-4 py-3 rounded-2xl rounded-bl-md bg-gray-100 text-gray-800 text-sm">
                <pre class="whitespace-pre-wrap font-sans leading-relaxed">{streamBuffer}</pre>
                <span class="inline-block w-2 h-4 bg-violet-500 animate-pulse ml-0.5 align-text-bottom"></span>
              </div>
            </div>
          </div>
        {/if}
      {/if}
    </div>

    {#if !autoScroll && (messages.length > 0 || streamBuffer || reasoningBuffer)}
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
