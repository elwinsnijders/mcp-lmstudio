<script>
  import { toasts, removeToast } from '../stores/toast.js'
</script>

{#if $toasts.length > 0}
  <div class="fixed bottom-4 right-4 z-50 flex flex-col gap-2">
    {#each $toasts as toast (toast.id)}
      <div
        class="px-4 py-3 rounded-lg shadow-lg text-sm font-medium flex items-center gap-2 min-w-[280px] animate-slide-in
          {toast.type === 'error' ? 'bg-red-600 text-white' :
           toast.type === 'success' ? 'bg-emerald-600 text-white' :
           'bg-slate-800 text-white'}"
      >
        <span class="flex-1">{toast.message}</span>
        <button class="text-white/70 hover:text-white" on:click={() => removeToast(toast.id)}>
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    {/each}
  </div>
{/if}

<style>
  @keyframes slide-in {
    from { transform: translateX(100%); opacity: 0; }
    to { transform: translateX(0); opacity: 1; }
  }
  .animate-slide-in {
    animation: slide-in 0.2s ease-out;
  }
</style>
