<script>
  import Sidebar from './lib/Sidebar.svelte'
  import Sessions from './lib/Sessions.svelte'
  import LiveChat from './lib/LiveChat.svelte'
  import ChatArchive from './lib/ChatArchive.svelte'
  import Profiles from './lib/Profiles.svelte'
  import Settings from './lib/Settings.svelte'
  import Toast from './lib/Toast.svelte'

  let currentPage = 'sessions'
  let archiveSessionId = null

  function handleNavigate(e) {
    currentPage = e.detail
    if (e.detail !== 'archive') {
      archiveSessionId = null
    }
  }

  function handleViewChat(e) {
    archiveSessionId = e.detail
    currentPage = 'archive'
  }

  function handleWatchLive(e) {
    currentPage = 'live'
  }
</script>

<div class="flex h-screen bg-gray-50">
  <Sidebar {currentPage} on:navigate={handleNavigate} />

  <main class="flex-1 overflow-y-auto ml-64">
    <div class="p-8 max-w-6xl">
      {#if currentPage === 'sessions'}
        <Sessions on:viewChat={handleViewChat} on:watchLive={handleWatchLive} />
      {:else if currentPage === 'live'}
        <LiveChat />
      {:else if currentPage === 'archive'}
        <ChatArchive sessionId={archiveSessionId} />
      {:else if currentPage === 'profiles'}
        <Profiles />
      {:else if currentPage === 'settings'}
        <Settings />
      {/if}
    </div>
  </main>

  <Toast />
</div>
