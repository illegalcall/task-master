"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Textarea } from "@/components/ui/textarea"
import { FileDropzone } from "@/components/file-dropzone"
import { JsonEditor } from "@/components/json-editor"
import { createJob } from "@/app/jobs/pdf-parser/new/action"
import { useToastContext } from "@/components/toast-provider"

type TabValue = "upload" | "url"

export function JobForm() {
  const router = useRouter()
  const { toast } = useToastContext()
  const [pdfBase64, setPdfBase64] = useState("")
  const [pdfSource, setPdfSource] = useState("")
  const [activeTab, setActiveTab] = useState<TabValue>("upload")
  const [isSubmitting, setIsSubmitting] = useState(false)

  const handleSubmit = async (formData: FormData) => {
    setIsSubmitting(true)
    try {
      const jobName = formData.get("job_name") as string
      const expectedSchema = formData.get("expected_schema") as string
      const source = activeTab === "upload" ? pdfBase64 : pdfSource
      const description = formData.get("description") as string
      
      const result = await createJob({
        jobName,
        pdfSource: source,
        expectedSchema,
        description,
      })

      if (result.success) {
        toast({
          title: "Success",
          description: "Job created successfully",
          variant: "success",
        })
        // Redirect to jobs list page
        router.push("/jobs/pdf-parser")
      } else {
        toast({
          title: "Error",
          description: result.error || "Failed to create job",
          variant: "destructive",
        })
        setIsSubmitting(false)
      }
    } catch (error) {
      toast({
        title: "Error",
        description: "An unexpected error occurred",
        variant: "destructive",
      })
      setIsSubmitting(false)
    }
  }

  return (
    <form action={handleSubmit}>
      <div className="grid gap-6">
        <div className="grid gap-2">
          <Label htmlFor="job_name">Job Name</Label>
          <Input
            id="job_name"
            name="job_name"
            type="text"
            placeholder="Enter your Job Name"
            required
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="description">Description</Label>
          <Textarea
            id="description"
            name="description"
            placeholder="Enter your Job Description"
          />
        </div>
        <div className="grid gap-2">
          <Label>PDF Source</Label>
          <Tabs
            value={activeTab}
            onValueChange={(value) => setActiveTab(value as TabValue)}
          >
            <TabsList className="grid w-full grid-cols-2">
              <TabsTrigger value="upload">Upload PDF</TabsTrigger>
              <TabsTrigger value="url">URL / Base64</TabsTrigger>
            </TabsList>
            <TabsContent value="upload">
              <FileDropzone onFileSelect={setPdfBase64} />
              <p className="text-sm text-muted-foreground">
                Upload a PDF file to process. The file will be converted to
                base64 for processing.
              </p>
            </TabsContent>
            <TabsContent value="url">
              <Input
                placeholder="Enter PDF URL or base64 string"
                value={pdfSource}
                onChange={(e) => setPdfSource(e.target.value)}
                className="font-mono"
              />
              <p className="text-sm text-muted-foreground">
                Enter a URL to a PDF file or a base64-encoded PDF string.
              </p>
            </TabsContent>
          </Tabs>
        </div>
        <div className="grid gap-2">
          <Label htmlFor="expected_schema">Expected Schema</Label>
          <JsonEditor name="expected_schema" />
          <p className="text-sm text-muted-foreground">
            Enter the JSON schema that defines the expected structure of the
            extracted data.
          </p>
        </div>
        <Button
          type="submit"
          className="w-full"
          disabled={
            isSubmitting ||
            !(
              (activeTab === "upload" && pdfBase64) ||
              (activeTab === "url" && pdfSource)
            )
          }
        >
          {isSubmitting ? "Creating Job..." : "Create Job"}
        </Button>
      </div>
    </form>
  )
}
