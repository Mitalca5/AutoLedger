import { ref } from 'vue'
import { t } from '@/i18n'
import { useToast } from '@/composables/useToast'

export interface ConfirmOptions {
  title?: string
  message: string
  confirmText?: string
  cancelText?: string
  type?: 'danger' | 'warning' | 'info' | 'success'
  isAlert?: boolean
}

const isOpen = ref(false)
const options = ref<ConfirmOptions>({
  title: '',
  message: '',
  confirmText: '',
  cancelText: '',
  type: 'danger',
  isAlert: false,
})

let resolvePromise: ((value: boolean) => void) | null = null

export function useConfirm() {
  function showConfirm(opts: ConfirmOptions | string): Promise<boolean> {
    if (typeof opts === 'string') {
      options.value = {
        title: t('shell.confirm.title'),
        message: opts,
        confirmText: t('common.confirm'),
        cancelText: t('common.cancel'),
        type: 'danger',
        isAlert: false,
      }
    } else {
      options.value = {
        title: opts.title || (opts.type === 'danger' ? t('shell.confirm.deleteTitle') : t('shell.confirm.title')),
        message: opts.message,
        confirmText: opts.confirmText || (opts.type === 'danger' ? t('common.delete') : t('common.confirm')),
        cancelText: opts.cancelText || t('common.cancel'),
        type: opts.type || 'danger',
        isAlert: false,
      }
    }
    isOpen.value = true

    return new Promise((resolve) => {
      resolvePromise = resolve
    })
  }

  /** A success is a transient toast; anything else that needs reading blocks behind a dialog. */
  function showAlert(message: string, title?: string, type: 'info' | 'warning' | 'danger' | 'success' = 'info'): Promise<void> {
    if (type === 'success') {
      useToast().showToast(message)
      return Promise.resolve()
    }
    options.value = {
      title: title || (type === 'danger' ? t('shell.confirm.error') : t('shell.confirm.information')),
      message,
      confirmText: t('shell.confirm.gotIt'),
      cancelText: '',
      type: type || 'info',
      isAlert: true,
    }
    isOpen.value = true

    return new Promise((resolve) => {
      resolvePromise = () => resolve()
    })
  }

  function onConfirm() {
    isOpen.value = false
    if (resolvePromise) {
      resolvePromise(true)
      resolvePromise = null
    }
  }

  function onCancel() {
    isOpen.value = false
    if (resolvePromise) {
      resolvePromise(false)
      resolvePromise = null
    }
  }

  return {
    isOpen,
    options,
    showConfirm,
    showAlert,
    onConfirm,
    onCancel,
  }
}
