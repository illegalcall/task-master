"use client"

import { useState } from "react"
import { FileText, UploadCloud, X } from "lucide-react"
import { useDropzone } from "react-dropzone"

import { cn } from "@/lib/utils"

interface FileDropzoneProps {
  onFileSelect: (base64: string) => void
  className?: string
}

export function FileDropzone({ onFileSelect, className }: FileDropzoneProps) {
  const [file, setFile] = useState<File | null>(null)
  const [isLoading, setIsLoading] = useState(false)

  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    accept: {
      "application/pdf": [".pdf"],
    },
    maxFiles: 1,
    onDrop: async (acceptedFiles) => {
      const selectedFile = acceptedFiles[0]
      if (selectedFile) {
        setFile(selectedFile)
        setIsLoading(true)
        try {
          const base64 = await convertToBase64(selectedFile)
          onFileSelect(base64)
        } catch (error) {
          console.error("Error converting file to base64:", error)
        } finally {
          setIsLoading(false)
        }
      }
    },
  })

  const removeFile = () => {
    setFile(null)
    onFileSelect("")
  }

  return (
    <div className={cn("w-full", className)}>
      <div
        {...getRootProps()}
        className={cn(
          "border-2 border-dashed rounded-lg p-8 text-center cursor-pointer transition-colors",
          "hover:border-primary/50 hover:bg-muted/50",
          isDragActive && "border-primary bg-muted",
          file && "border-primary/50"
        )}
      >
        <input {...getInputProps()} />
        {file ? (
          <div className="flex flex-col items-center gap-2">
            <FileText className="h-8 w-8 text-primary" />
            <p className="text-sm font-medium">{file.name}</p>
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation()
                removeFile()
              }}
              className="text-muted-foreground hover:text-foreground transition-colors"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        ) : (
          <div className="flex flex-col items-center gap-2">
            <UploadCloud className="h-8 w-8 text-muted-foreground" />
            <div className="text-sm">
              <p className="font-medium">
                {isDragActive
                  ? "Drop the PDF here"
                  : "Drag & drop a PDF file here"}
              </p>
              <p className="text-muted-foreground">or click to select</p>
            </div>
          </div>
        )}
      </div>
      {isLoading && (
        <p className="text-sm text-muted-foreground mt-2 text-center">
          Processing file...
        </p>
      )}
    </div>
  )
}

function convertToBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.readAsDataURL(file)
    reader.onload = () => {
      const base64String = reader.result as string
      // Remove the data URL prefix (e.g., "data:application/pdf;base64,")
      const base64 = base64String.split(",")[1]
      resolve(base64)
    }
    reader.onerror = (error) => reject(error)
  })
}
