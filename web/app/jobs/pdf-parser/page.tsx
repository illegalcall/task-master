import Link from "next/link"
import { format } from "date-fns"

import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import {
  SidebarInset,
  SidebarProvider,
  SidebarTrigger,
} from "@/components/ui/sidebar"
import { AppSidebar } from "@/components/app-sidebar"
import { JobsClient } from "@/components/jobs/jobs-client"

import { fetchJobs } from "./action"

interface Job {
  id: number
  name: string
  status: string
  created_at: string
}

interface JobsListProps {
  jobs: Job[]
}

function JobsList({ jobs }: JobsListProps) {
  return (
    <div className="grid gap-4">
      {jobs.length > 0 ? (
        jobs.map((job) => (
          <Link key={job.id} href={`/jobs/${job.id}`} className="block">
            <Card className="p-4 transition-all hover:shadow-md">
              <h2 className="text-lg font-semibold">{job.name}</h2>
              <p className="text-sm">
                Status:{" "}
                <span
                  className={`font-semibold capitalize ${
                    job.status === "pending"
                      ? "text-blue-500"
                      : job.status === "completed"
                      ? "text-green-500"
                      : "text-red-500"
                  }`}
                >
                  {job.status}
                </span>
              </p>
              <p className="text-xs text-gray-400">
                Created At: {format(new Date(job.created_at), "PPP")}
              </p>
            </Card>
          </Link>
        ))
      ) : (
        <p className="text-center text-gray-500">No jobs found.</p>
      )}
    </div>
  )
}

export default async function JobsPage({
  searchParams,
}: {
  searchParams?: { status?: string; date?: string }
}) {
  const jobs = await fetchJobs()

  let filteredJobs = [...jobs]

  if (searchParams?.status) {
    filteredJobs = filteredJobs.filter(
      (job) => job.status === searchParams.status
    )
  }

  if (searchParams?.date) {
    filteredJobs = filteredJobs.filter((job) =>
      job.created_at.startsWith(searchParams.date)
    )
  }
  return (
    <SidebarProvider>
      <AppSidebar />
      <SidebarInset>
        <header className="flex h-16 shrink-0 items-center gap-2 transition-[width,height] ease-linear group-has-[[data-collapsible=icon]]/sidebar-wrapper:h-12">
          <div className="flex items-center gap-2 px-4">
            <SidebarTrigger className="-ml-1" />
            <Separator orientation="vertical" className="mr-2 h-4" />
            <Breadcrumb>
              <BreadcrumbList>
                <BreadcrumbItem className="hidden md:block">
                  <BreadcrumbLink href="#">Jobs</BreadcrumbLink>
                </BreadcrumbItem>
                <BreadcrumbSeparator className="hidden md:block" />
                <BreadcrumbItem className="hidden md:block">
                  <BreadcrumbLink href="/jobs/pdf-parser">
                    PDF Parser
                  </BreadcrumbLink>
                </BreadcrumbItem>
              </BreadcrumbList>
            </Breadcrumb>
          </div>
        </header>
        <div className="flex flex-1 flex-col gap-4 p-4 pt-0">
          <JobsClient />
          <JobsList jobs={filteredJobs} />
        </div>
      </SidebarInset>
    </SidebarProvider>
  )
}
