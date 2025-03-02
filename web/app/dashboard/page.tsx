"use client"

import { useState } from "react"
import { FileText, UploadCloud, X } from "lucide-react"
import { useDropzone } from "react-dropzone"

import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Textarea } from "@/components/ui/textarea"

export default function UploadPage() {
  const [files, setFiles] = useState<File[]>([])
  const [jsonSchema, setJsonSchema] = useState<string>("")
  const [description, setDescription] = useState<string>("")

  const onDrop = (acceptedFiles: File[]) => {
    if (acceptedFiles.length + files.length > 4) {
      alert("You can only upload up to 4 files.")
      return
    }
    setFiles((prevFiles) => [...prevFiles, ...acceptedFiles])
  }

  const removeFile = (fileName: string) => {
    setFiles((prevFiles) => prevFiles.filter((file) => file.name !== fileName))
  }

  const handleSubmit = () => {
    if (!files.length) {
      alert("Please upload at least one file.")
      return
    }
    if (!jsonSchema.trim()) {
      alert("Please enter a valid JSON schema.")
      return
    }
    if (!description.trim()) {
      alert("Please enter a description.")
      return
    }

    console.log("Uploading Data:", {
      files,
      jsonSchema,
      description,
    })

    alert("Data ingestion request submitted!")
  }

  const { getRootProps, getInputProps } = useDropzone({
    onDrop,
    accept: {
      "image/*": [".png", ".jpg", ".jpeg", ".gif"],
      "application/pdf": [".pdf"],
    },
    maxSize: 4 * 1024 * 1024, 
  })

  return (
    <section className="container grid gap-6 pb-8 pt-6 md:py-10 max-w-2xl mx-auto">
      {/* Page Heading */}
      <h1 className="text-3xl font-extrabold tracking-tighter">
        Data Ingestion Job
      </h1>

      <div
        {...getRootProps()}
        className="border-2 border-dashed border-gray-300 p-10 rounded-lg flex flex-col items-center justify-center text-gray-500 cursor-pointer hover:border-gray-400"
      >
        <UploadCloud className="h-10 w-10 mb-2" />
        <p className="text-sm">Drag & drop files here, or click to select</p>
        <p className="text-xs text-gray-400">
          You can upload up to 4 files (max 4MB each)
        </p>
        <input {...getInputProps()} />
      </div>

      {files.length > 0 && (
        <Card className="p-4">
          <h3 className="text-lg font-semibold">Uploaded files</h3>
          <ul className="mt-4 w-full">
            {files.map((file, index) => (
              <li
                key={index}
                className="flex items-center justify-between py-2 border-b"
              >
                <div className="flex items-center space-x-2">
                  <FileText className="h-5 w-5 text-gray-500" />
                  <span className="truncate">{file.name}</span>
                </div>
                <div className="flex items-center space-x-2">
                  <span className="text-gray-400 text-xs">
                    {(file.size / 1024).toFixed(2)} KB
                  </span>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => removeFile(file.name)}
                  >
                    <X className="h-5 w-5 text-red-500" />
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        </Card>
      )}

      <div>
        <label className="text-sm font-semibold">JSON Schema</label>
        <Textarea
          placeholder='Enter JSON schema, e.g. { "name": "string", "age": "number" }'
          value={jsonSchema}
          onChange={(e) => setJsonSchema(e.target.value)}
          className="h-32 font-mono"
        />
      </div>

      <div>
        <label className="text-sm font-semibold">Description</label>
        <Textarea
          placeholder="Enter description"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
      </div>

      <Button onClick={handleSubmit} className="w-full">
        Save
      </Button>
    </section>
  )
}
