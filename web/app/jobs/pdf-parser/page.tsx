import Link from "next/link"
import { format } from "date-fns"

import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
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
    <section className="container mx-auto py-10">
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-3xl font-extrabold">Jobs</h1>
        <Link href="/jobs/new">
          <Button size="sm">Add Job</Button>
        </Link>
      </div>
      <JobsClient />
      <JobsList jobs={filteredJobs} />
    </section>
  )
}
