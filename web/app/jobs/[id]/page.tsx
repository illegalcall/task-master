import React from "react"
import Link from "next/link"
import { notFound } from "next/navigation"
import { format } from "date-fns"
import { ArrowLeft, Clock, Copy, RefreshCw } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"

import { CopyButton } from "./CopyButton"
import { fetchJobById } from "./action"

interface PageProps {
  params: {
    id: string
  }
}

export default async function JobDetailsPage({ params }: PageProps) {
  let job
  try {
    job = await fetchJobById(params.id)
  } catch (error) {
    notFound()
  }

  // Determine the badge color based on job status
  const getBadgeVariant = (status: string) => {
    switch (status) {
      case "pending":
        return "secondary"
      case "processing":
        return "secondary"
      case "completed":
        return "default"
      case "failed":
        return "destructive"
      default:
        return "outline"
    }
  }

  // Get status icon based on job status
  const getStatusIcon = (status: string) => {
    switch (status) {
      case "pending":
        return <Clock className="h-4 w-4 mr-1" />
      case "processing":
        return <RefreshCw className="h-4 w-4 mr-1 animate-spin" />
      case "completed":
        return <span className="mr-1">✓</span>
      case "failed":
        return <span className="mr-1">✕</span>
      default:
        return null
    }
  }

  // Sample JSON response for demonstration
  const sampleJsonResponse = {
    job: {
      id: job.id,
      name: job.name,
      status: job.status,
      type: job.type,
      created_at: job.created_at,
      details: {
        steps: ["initialize", "process", "complete"],
        progress:
          job.status === "completed" ? 100 : job.status === "failed" ? 75 : 50,
        logs: [
          { timestamp: "2023-05-10T10:00:00Z", message: "Job started" },
          { timestamp: "2023-05-10T10:05:00Z", message: "Processing data" },
        ],
      },
    },
  }

  return (
    <div className="container mx-auto py-6 space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Button variant="outline" size="icon" asChild>
            <Link href="/jobs">
              <ArrowLeft className="h-4 w-4" />
            </Link>
          </Button>
          <h1 className="text-3xl font-bold tracking-tight">Job Details</h1>
        </div>

        {/* Status badge moved to top level for better visibility */}
        <Badge
          variant={getBadgeVariant(job.status)}
          className="px-3 py-1 text-sm"
        >
          <span className="flex items-center">
            {getStatusIcon(job.status)}
            {job.status.charAt(0).toUpperCase() + job.status.slice(1)}
          </span>
        </Badge>
      </div>

      <Tabs defaultValue="details" className="w-full">
        <TabsList className="grid w-full max-w-md grid-cols-2">
          <TabsTrigger value="details">Details</TabsTrigger>
          <TabsTrigger value="json">JSON Response</TabsTrigger>
        </TabsList>

        <TabsContent value="details">
          <Card>
            <CardHeader>
              <CardTitle>{job.name}</CardTitle>
              <CardDescription>
                Job ID: {job.id} • Created on{" "}
                {format(new Date(job.created_at), "PPP")}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                <div className="space-y-4">
                  <div className="bg-muted/50 p-4 rounded-lg">
                    <p className="text-sm font-medium text-muted-foreground mb-1">
                      Type
                    </p>
                    <p className="font-medium">{job.type}</p>
                  </div>

                  <div className="bg-muted/50 p-4 rounded-lg">
                    <p className="text-sm font-medium text-muted-foreground mb-1">
                      Created
                    </p>
                    <p className="font-medium">
                      {format(new Date(job.created_at), "PPpp")}
                    </p>
                  </div>
                </div>

                <div className="bg-muted/50 p-4 rounded-lg">
                  <p className="text-sm font-medium text-muted-foreground mb-1">
                    Timeline
                  </p>
                  <div className="space-y-2 mt-2">
                    <div className="flex items-center">
                      <div className="w-2 h-2 rounded-full bg-green-500 mr-2"></div>
                      <p className="text-sm">
                        Created: {format(new Date(job.created_at), "h:mm a")}
                      </p>
                    </div>
                    {job.status !== "pending" && (
                      <div className="flex items-center">
                        <div className="w-2 h-2 rounded-full bg-blue-500 mr-2"></div>
                        <p className="text-sm">
                          Processing started:{" "}
                          {format(new Date(job.created_at), "h:mm a")}
                        </p>
                      </div>
                    )}
                    {(job.status === "completed" ||
                      job.status === "failed") && (
                      <div className="flex items-center">
                        <div
                          className={`w-2 h-2 rounded-full ${
                            job.status === "completed"
                              ? "bg-green-500"
                              : "bg-red-500"
                          } mr-2`}
                        ></div>
                        <p className="text-sm">
                          {job.status === "completed" ? "Completed" : "Failed"}:{" "}
                          {format(new Date(job.created_at), "h:mm a")}
                        </p>
                      </div>
                    )}
                  </div>
                </div>
              </div>
            </CardContent>
            <CardFooter className="flex justify-end">
              {/* Only show retry button for failed jobs */}
              {job.status === "failed" && (
                <Button variant="destructive">Retry Job</Button>
              )}
            </CardFooter>
          </Card>
        </TabsContent>

        <TabsContent value="json">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <div>
                <CardTitle>API Response</CardTitle>
                <CardDescription>
                  Raw JSON response from the API
                </CardDescription>
              </div>
              <CopyButton jsonData={sampleJsonResponse} />
            </CardHeader>
            <CardContent>
              <div className="bg-black text-white p-4 rounded-lg overflow-auto max-h-[400px]">
                <pre className="text-sm">
                  <code>{JSON.stringify(sampleJsonResponse, null, 2)}</code>
                </pre>
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  )
}
