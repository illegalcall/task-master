import { useState, useEffect, useCallback } from "react"

interface ToastProps {
  id: string
  title?: string
  description?: string
  action?: React.ReactNode
  variant?: "default" | "destructive" | "success"
}

interface ToastState {
  toasts: ToastProps[]
}

const TOAST_LIMIT = 3
const TOAST_REMOVE_DELAY = 5000

export const useToast = () => {
  const [state, setState] = useState<ToastState>({
    toasts: [],
  })

  const toast = useCallback((props: Omit<ToastProps, "id">) => {
    const id = Math.random().toString(36).substring(2, 9)
    setState((state) => {
      return {
        toasts: [{ id, ...props }, ...state.toasts].slice(0, TOAST_LIMIT),
      }
    })
    return id
  }, [])

  const dismiss = useCallback((toastId: string) => {
    setState((state) => {
      return {
        toasts: state.toasts.filter((t) => t.id !== toastId),
      }
    })
  }, [])

  const dismissAll = useCallback(() => {
    setState({ toasts: [] })
  }, [])

  return {
    toast,
    dismiss,
    dismissAll,
    toasts: state.toasts,
  }
}

export const ToastContext = () => {
  const [state, setState] = useState<ToastState>({
    toasts: [],
  })

  const addToast = useCallback((props: Omit<ToastProps, "id">) => {
    const id = Math.random().toString(36).substring(2, 9)
    setState((state) => {
      return {
        toasts: [{ id, ...props }, ...state.toasts].slice(0, TOAST_LIMIT),
      }
    })
    return id
  }, [])

  const removeToast = useCallback((id: string) => {
    setState((state) => ({
      toasts: state.toasts.filter((toast) => toast.id !== id),
    }))
  }, [])

  useEffect(() => {
    const timeoutIds = new Map<string, NodeJS.Timeout>()
    
    state.toasts.forEach((toast) => {
      if (!timeoutIds.has(toast.id)) {
        const timeoutId = setTimeout(() => {
          removeToast(toast.id)
          timeoutIds.delete(toast.id)
        }, TOAST_REMOVE_DELAY)
        
        timeoutIds.set(toast.id, timeoutId)
      }
    })
    
    return () => {
      timeoutIds.forEach((timeoutId) => {
        clearTimeout(timeoutId)
      })
    }
  }, [state.toasts, removeToast])

  return {
    ...state,
    addToast,
    removeToast,
  }
}

export { type ToastProps } 