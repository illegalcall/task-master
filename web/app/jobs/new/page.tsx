import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { JsonEditor } from "@/components/json-editor"

import { createJob } from "./action"

export default function NewJobPage() {
  return (
    <div className="container mx-auto py-10">
      <Card className="max-w-2xl mx-auto">
        <CardHeader>
          <CardTitle>Create New Job</CardTitle>
          <CardDescription>
            Fill in the details below to create a new PDF processing job.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form
            action={async (formData) => {
              "use server"
              const jobName = formData.get("job_name") as string
              const pdfSource = formData.get("pdf_source") as string
              const expectedSchema = formData.get("expected_schema") as string
              await createJob({ jobName, pdfSource, expectedSchema })
            }}
          >
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
                <Label htmlFor="pdf_source">PDF Source</Label>
                <Input
                  id="pdf_source"
                  name="pdf_source"
                  type="text"
                  placeholder="Enter your PDF Source"
                  required
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="expected_schema">Expected Schema</Label>
                <JsonEditor name="expected_schema" />
                <p className="text-sm text-muted-foreground">
                  Enter the JSON schema that defines the expected structure of
                  the extracted data.
                </p>
              </div>
              <Button type="submit" className="w-full">
                Create Job
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
