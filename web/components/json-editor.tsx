"use client"

import Editor from "@monaco-editor/react"
import { useTheme } from "next-themes"

import { cn } from "@/lib/utils"

interface JsonEditorProps {
  name: string
  defaultValue?: string
  className?: string
}

export function JsonEditor({
  name,
  defaultValue = "{\n  \n}",
  className,
}: JsonEditorProps) {
  const { theme } = useTheme()

  return (
    <div
      className={cn(
        "relative rounded-md border focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-2",
        "bg-background",
        className
      )}
    >
      <div className="absolute top-2 right-2 px-2 py-1 text-xs rounded-sm bg-muted text-muted-foreground">
        JSON
      </div>
      <div className="h-[300px] [&_.monaco-editor]:rounded-md [&_.monaco-editor_.margin]:rounded-l-md">
        <Editor
          height="100%"
          defaultLanguage="json"
          defaultValue={defaultValue}
          theme={theme === "dark" ? "vs-dark" : "light"}
          options={{
            minimap: { enabled: false },
            scrollBeyondLastLine: false,
            fontSize: 14,
            tabSize: 2,
            lineNumbers: "on",
            roundedSelection: true,
            padding: { top: 16 },
            cursorStyle: "line",
            fontFamily: "var(--font-mono)",
            formatOnPaste: true,
            formatOnType: true,
          }}
          onChange={(value: string | undefined) => {
            const input = document.createElement("input")
            input.type = "hidden"
            input.name = name
            input.value = value || "{}"
            const form = document.querySelector("form")
            const oldInput = form?.querySelector(`input[name="${name}"]`)
            if (oldInput) {
              oldInput.remove()
            }
            form?.appendChild(input)
          }}
        />
      </div>
    </div>
  )
}
