<script>
  import { onMount, tick } from 'svelte'
  import { ListSessions, LoadChatLog, GetGroup } from '../../wailsjs/go/main/App'
  import { marked } from 'marked'

  marked.setOptions({ breaks: true, gfm: true })

  function md(text) {
    if (!text) return ''
    return marked.parse(text)
  }

  function mkAssistant(content, stats, ts) {
    return { role: 'assistant', content, html: md(content), stats, ts }
  }

  export let sessionId = null

  let sessions = []
  let selected = sessionId
  let events = []
  let messages = []
  let loading = false
  let expanded = {}
  let groupInfo = null
  let loadedGroupId = null

  $: if (selected) {
    const sess = sessions.find(s => s.id === selected)
    if (loadedGroupId && sess?.groupId === loadedGroupId && groupInfo?.sessionIds?.includes(selected)) {
      const idx = groupInfo.sessionIds.indexOf(selected)
      if (idx >= 0) tick().then(() => setTimeout(() => scrollToStep(idx), 50))
    } else {
      loadChat(selected)
    }
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
    expandedTools = {}
    groupInfo = null
    loadedGroupId = null
    try {
      const sess = sessions.find(s => s.id === id)
      if (sess?.groupId) {
        try { groupInfo = await GetGroup(sess.groupId) } catch (_) {}
      }

      if (groupInfo?.sessionIds?.length > 1) {
        loadedGroupId = sess.groupId
        const sw = getStepWord(groupInfo.type)
        let allMessages = []
        for (let i = 0; i < groupInfo.sessionIds.length; i++) {
          allMessages.push({
            role: 'step_divider', stepIdx: i,
            content: `${sw} ${i + 1} of ${groupInfo.totalSteps}`
          })
          try {
            const result = await LoadChatLog(groupInfo.sessionIds[i])
            if (result) allMessages.push(...collapseEvents(result))
          } catch (_) {}
        }
        events = []
        messages = allMessages
      } else {
        const result = await LoadChatLog(id)
        events = result || []
        messages = collapseEvents(events)
      }
    } catch (e) {
      console.error('Failed to load chat:', e)
      events = []
      messages = []
    } finally {
      loading = false
    }

    if (groupInfo?.sessionIds) {
      const stepIdx = groupInfo.sessionIds.indexOf(id)
      if (stepIdx >= 0) {
        await tick()
        setTimeout(() => scrollToStep(stepIdx), 100)
      }
    }
  }

  function collapseEvents(evts) {
    let msgs = []
    let pendingDelta = ''
    let pendingReasoning = ''

    for (const ev of evts) {
      switch (ev.type) {
        case 'user_message':
          if (pendingDelta.trim()) {
            msgs.push(mkAssistant(pendingDelta.trim(), null, ev.ts))
          }
          pendingDelta = ''
          pendingReasoning = ''
          msgs.push({ role: 'user', content: ev.content, ts: ev.ts })
          break
        case 'reasoning_start':
          if (pendingDelta.trim()) {
            msgs.push(mkAssistant(pendingDelta.trim(), null, ev.ts))
          }
          pendingDelta = ''
          pendingReasoning = ''
          break
        case 'reasoning_delta':
          pendingReasoning += ev.content || ''
          break
        case 'reasoning_end':
          if (pendingReasoning) {
            msgs.push({ role: 'reasoning', content: pendingReasoning, ts: ev.ts })
            pendingReasoning = ''
          }
          break
        case 'ai_delta':
          pendingDelta += ev.content
          break
        case 'ai_complete':
          msgs.push(mkAssistant(ev.content || pendingDelta, ev.stats, ev.ts))
          pendingDelta = ''
          break
        case 'error':
          msgs.push({ role: 'error', content: ev.content, ts: ev.ts })
          pendingDelta = ''
          break
        case 'tool_use':
          if (pendingDelta.trim()) {
            msgs.push(mkAssistant(pendingDelta.trim(), null, ev.ts))
          }
          pendingDelta = ''
          msgs.push({ role: 'tool', content: ev.content, tool: ev.tool, ts: ev.ts })
          break
        case 'tool_call_start':
          if (pendingDelta.trim()) {
            msgs.push(mkAssistant(pendingDelta.trim(), null, ev.ts))
          }
          pendingDelta = ''
          break
        case 'tool_call_result':
          msgs.push({
            role: 'tool_result',
            tool: ev.tool,
            arguments: ev.arguments,
            output: ev.output,
            success: ev.success,
            reason: ev.reason,
            ts: ev.ts
          })
          break
        case 'status':
          break
        case 'group_start':
          msgs.push({
            role: 'group_event', eventType: 'start',
            content: `${(ev.group_type || 'group').toUpperCase()} started — ${ev.group_total} steps`,
            ts: ev.ts
          })
          break
        case 'group_step':
          msgs.push({
            role: 'group_event', eventType: 'step',
            content: `Step ${ev.group_step}/${ev.group_total}`,
            ts: ev.ts
          })
          break
        case 'group_complete':
          msgs.push({
            role: 'group_event', eventType: 'complete',
            content: ev.content || `${(ev.group_type || 'group').toUpperCase()} complete`,
            ts: ev.ts
          })
          break
      }
    }

    if (pendingReasoning) {
      msgs.push({ role: 'reasoning', content: pendingReasoning, ts: '' })
    }
    if (pendingDelta) {
      msgs.push(mkAssistant(pendingDelta, null, ''))
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

  function getSessionInfo(id) {
    return sessions.find((s) => s.id === id)
  }

  $: sessionInfo = selected ? getSessionInfo(selected) : null

  function groupTypeColor(type) {
    if (type === 'queue') return 'indigo'
    if (type === 'chain') return 'teal'
    return 'orange'
  }

  function getStepWord(type) {
    if (type === 'loop') return 'Iteration'
    if (type === 'queue') return 'Task'
    return 'Step'
  }

  function scrollToStep(idx) {
    const el = document.getElementById(`archive-step-${idx}`)
    if (el) el.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }
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
          <option value={s.id}>{s.id} [{s.status}] {s.profile || ''}{s.project ? ' | ' + s.project : ''}</option>
        {/each}
      </select>
    </div>
  </div>

  {#if sessionInfo && !loadedGroupId}
    <div class="mb-3 shrink-0 px-4 py-2.5 bg-slate-50 rounded-lg border border-slate-200 flex items-center gap-6 text-xs text-slate-600">
      <span><strong>Task:</strong> {sessionInfo.task?.slice(0, 100) || '-'}</span>
      <span><strong>Model:</strong> {sessionInfo.model || '-'}</span>
      <span><strong>Tokens:</strong> {sessionInfo.tokensUsed?.toLocaleString() || 0} / {sessionInfo.tokensMax?.toLocaleString() || 0}</span>
      <span><strong>Exchanges:</strong> {sessionInfo.exchanges || 0}</span>
    </div>
  {/if}

  {#if groupInfo && groupInfo.sessionIds?.length > 1}
    {@const g = groupInfo}
    {@const pct = g.totalSteps > 0 ? (g.currentStep / g.totalSteps) * 100 : 0}
    {@const sw = getStepWord(g.type)}
    <div class="mb-3 shrink-0 px-4 py-2.5 bg-gray-50 rounded-lg border border-gray-200 flex items-center gap-3 text-xs">
      {#if g.type === 'queue'}
        <span class="font-bold uppercase tracking-wider text-indigo-600">{g.type}</span>
      {:else if g.type === 'chain'}
        <span class="font-bold uppercase tracking-wider text-teal-600">{g.type}</span>
      {:else}
        <span class="font-bold uppercase tracking-wider text-orange-600">{g.type}</span>
      {/if}
      <span class="text-gray-600 font-medium">{g.totalSteps} {sw.toLowerCase()}s</span>
      <div class="w-24 h-1.5 bg-gray-200 rounded-full overflow-hidden">
        {#if g.status === 'failed'}
          <div class="h-full rounded-full transition-all duration-300 bg-red-500" style="width: {Math.min(pct, 100)}%"></div>
        {:else if g.status === 'completed'}
          <div class="h-full rounded-full transition-all duration-300 bg-emerald-500" style="width: {Math.min(pct, 100)}%"></div>
        {:else if g.type === 'queue'}
          <div class="h-full rounded-full transition-all duration-300 bg-indigo-500" style="width: {Math.min(pct, 100)}%"></div>
        {:else if g.type === 'chain'}
          <div class="h-full rounded-full transition-all duration-300 bg-teal-500" style="width: {Math.min(pct, 100)}%"></div>
        {:else}
          <div class="h-full rounded-full transition-all duration-300 bg-orange-500" style="width: {Math.min(pct, 100)}%"></div>
        {/if}
      </div>
      <span class="font-semibold {g.status === 'running' ? 'text-emerald-600' : g.status === 'completed' ? 'text-gray-500' : 'text-red-600'}">
        {g.status}
      </span>
      {#if g.chainMode}
        <span class="text-gray-400">mode: {g.chainMode}</span>
      {/if}
      {#if g.stoppedEarly}
        <span class="text-orange-500 font-medium">stopped early</span>
      {/if}
      <div class="ml-auto flex items-center gap-1">
        {#each g.sessionIds as sid, idx}
          <button
            class="w-6 h-6 rounded text-[10px] font-bold flex items-center justify-center transition-colors
              {idx < g.currentStep
                ? (g.type === 'queue' ? 'bg-indigo-600 text-white shadow-sm' : g.type === 'chain' ? 'bg-teal-600 text-white shadow-sm' : 'bg-orange-600 text-white shadow-sm')
                : 'bg-gray-200 text-gray-600 hover:bg-gray-300'}"
            on:click={() => scrollToStep(idx)}
            title="{sw} {idx + 1}"
          >{idx + 1}</button>
        {/each}
      </div>
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
          {#if msg.role === 'step_divider'}
            <div id="archive-step-{msg.stepIdx}" class="scroll-mt-4 flex items-center gap-3 py-3 my-1">
              <div class="flex-1 h-px bg-gray-300"></div>
              {#if groupInfo?.type === 'queue'}
                <div class="px-4 py-1.5 rounded-full text-[11px] font-bold uppercase tracking-wider border bg-indigo-50 border-indigo-200 text-indigo-700">
                  {msg.content}
                </div>
              {:else if groupInfo?.type === 'chain'}
                <div class="px-4 py-1.5 rounded-full text-[11px] font-bold uppercase tracking-wider border bg-teal-50 border-teal-200 text-teal-700">
                  {msg.content}
                </div>
              {:else}
                <div class="px-4 py-1.5 rounded-full text-[11px] font-bold uppercase tracking-wider border bg-orange-50 border-orange-200 text-orange-700">
                  {msg.content}
                </div>
              {/if}
              <div class="flex-1 h-px bg-gray-300"></div>
            </div>
          {:else}
          <div class="group">
            <div class="flex items-center gap-2 mb-1.5">
              <span class="text-[10px] font-semibold uppercase tracking-wider
                {msg.role === 'user' ? 'text-violet-500' :
                 msg.role === 'assistant' ? 'text-emerald-600' :
                 msg.role === 'reasoning' ? 'text-blue-500' :
                 msg.role === 'error' ? 'text-red-500' :
                 msg.role === 'group_event' ? 'text-indigo-500' :
                 msg.role === 'tool_start' || msg.role === 'tool_result' ? 'text-amber-600' :
                 'text-amber-600'}">
                {msg.role === 'user' ? 'User' :
                 msg.role === 'assistant' ? 'AI' :
                 msg.role === 'reasoning' ? 'Reasoning' :
                 msg.role === 'error' ? 'Error' :
                 msg.role === 'group_event' ? 'Group' :
                 msg.role === 'tool_start' ? 'Tool Call' :
                 msg.role === 'tool_result' ? 'Tool Result' :
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

            {:else if msg.role === 'reasoning'}
              <div class="pl-3 border-l-2 border-blue-200">
                <div class="px-3 py-2 bg-blue-50 rounded text-xs text-blue-800 leading-relaxed">
                  <pre class="whitespace-pre-wrap font-sans">{msg.content}</pre>
                </div>
              </div>

            {:else if msg.role === 'assistant'}
              <div class="pl-3 border-l-2 border-emerald-200 prose prose-sm prose-gray max-w-none">
                {@html msg.html || md(msg.content)}
              </div>
              {#if expanded[i] && msg.stats}
                <div class="mt-2 ml-3 px-3 py-2 bg-slate-50 rounded text-[11px] text-slate-500 font-mono">
                  {formatStats(msg.stats)}
                </div>
              {/if}

            {:else if msg.role === 'group_event'}
              <div class="flex justify-center">
                <div class="px-4 py-1.5 rounded-full text-[11px] font-semibold
                  {msg.eventType === 'start' ? 'bg-indigo-50 border border-indigo-200 text-indigo-700' :
                   msg.eventType === 'complete' ? 'bg-emerald-50 border border-emerald-200 text-emerald-700' :
                   'bg-gray-100 border border-gray-200 text-gray-600'}">
                  {msg.content}
                </div>
              </div>

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

            {:else if msg.role === 'tool_start'}
              <div class="pl-3 border-l-2 border-amber-200">
                <div class="px-3 py-1.5 bg-amber-50 rounded text-xs font-mono text-amber-600">
                  {msg.tool ? `Calling: ${msg.tool}` : 'Preparing tool call...'}
                </div>
              </div>

            {:else if msg.role === 'tool_result'}
              {@const argsP = previewLines(msg.arguments)}
              {@const outP = previewLines(msg.output)}
              {@const tkey = `${i}`}
              <div class="pl-3 border-l-2 border-amber-200">
                <div class="rounded overflow-hidden border text-xs font-mono {msg.success ? 'border-emerald-200' : 'border-red-200'}">
                  <div class="px-3 py-1.5 {msg.success ? 'bg-emerald-50 text-emerald-700' : 'bg-red-50 text-red-700'}">
                    {msg.tool}: {msg.success ? 'success' : 'failed'}
                    {#if msg.reason}
                      <span class="text-red-500"> ({msg.reason})</span>
                    {/if}
                  </div>
                  {#if msg.arguments}
                    <div class="px-3 py-1.5 border-t {msg.success ? 'border-emerald-200 bg-emerald-50/50' : 'border-red-200 bg-red-50/50'} text-gray-600">
                      <div class="text-gray-400 mb-0.5">args:</div>
                      <pre class="whitespace-pre-wrap">{expandedTools[tkey + '_args'] ? argsP.full : argsP.preview}</pre>
                      {#if argsP.needsExpand}
                        <button class="text-[10px] text-violet-500 hover:text-violet-700 mt-1" on:click={() => { expandedTools[tkey + '_args'] = !expandedTools[tkey + '_args']; expandedTools = expandedTools }}>
                          {expandedTools[tkey + '_args'] ? 'Show less' : 'Show more...'}
                        </button>
                      {/if}
                    </div>
                  {/if}
                  {#if msg.output}
                    <div class="px-3 py-1.5 border-t {msg.success ? 'border-emerald-200 bg-white' : 'border-red-200 bg-white'} text-gray-700">
                      <div class="text-gray-400 mb-0.5">output:</div>
                      <pre class="whitespace-pre-wrap">{expandedTools[tkey + '_out'] ? outP.full : outP.preview}</pre>
                      {#if outP.needsExpand}
                        <button class="text-[10px] text-violet-500 hover:text-violet-700 mt-1" on:click={() => { expandedTools[tkey + '_out'] = !expandedTools[tkey + '_out']; expandedTools = expandedTools }}>
                          {expandedTools[tkey + '_out'] ? 'Show less' : 'Show more...'}
                        </button>
                      {/if}
                    </div>
                  {/if}
                </div>
              </div>
            {/if}
          </div>
          {/if}
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
