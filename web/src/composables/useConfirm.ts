// Asking "are you sure" without every view growing its own dialog state.
//
// `ask` returns a promise that resolves to the answer, so a call site reads exactly like
// the window.confirm it replaces — `if (!(await ask({...}))) return` — while rendering a
// real in-app dialog. One instance lives in the app shell; this is the module-level
// singleton it talks to, the same shape as useTheme.
import { ref } from 'vue'

export interface ConfirmRequest {
  title: string
  message: string
  confirmLabel?: string
  danger?: boolean
}

const request = ref<ConfirmRequest | null>(null)
let settle: ((confirmed: boolean) => void) | null = null

export function useConfirm() {
  function ask(options: ConfirmRequest): Promise<boolean> {
    // A second ask while one is open would strand the first promise unresolved and
    // leave whatever it guards hanging forever. Decline it instead.
    settle?.(false)
    request.value = options
    return new Promise<boolean>((resolve) => {
      settle = resolve
    })
  }

  function answer(confirmed: boolean) {
    request.value = null
    settle?.(confirmed)
    settle = null
  }

  return { request, ask, answer }
}
