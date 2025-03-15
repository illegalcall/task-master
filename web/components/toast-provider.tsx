"use client"

import { createContext, useContext, useState } from "react"

import {
  Toast,
  ToastClose,
  ToastDescription,
  ToastProvider as UIToastProvider,
  ToastTitle,
  ToastViewport,
} from "@/components/ui/toast"
import { useToast } from "@/hooks/use-toast"

export const ToastContext = createContext<ReturnType<typeof useToast> | null>(
  null
)

export function useToastContext() {
  const context = useContext(ToastContext)
  if (!context) {
    throw new Error("useToastContext must be used within a ToastProvider")
  }
  return context
}

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const { toast, dismiss, dismissAll, toasts } = useToast()

  return (
    <ToastContext.Provider value={{ toast, dismiss, dismissAll, toasts }}>
      <UIToastProvider>
        {children}
        {toasts.map(({ id, title, description, variant, action }) => (
          <Toast key={id} variant={variant} onOpenChange={() => dismiss(id)}>
            {title && <ToastTitle>{title}</ToastTitle>}
            {description && (
              <ToastDescription>{description}</ToastDescription>
            )}
            {action}
            <ToastClose />
          </Toast>
        ))}
        <ToastViewport />
      </UIToastProvider>
    </ToastContext.Provider>
  )
} 