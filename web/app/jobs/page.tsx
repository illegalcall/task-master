"use client"

import { useEffect, useState } from "react"
import { format } from "date-fns"
import { CalendarIcon } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Calendar } from "@/components/ui/calendar"
import { Card } from "@/components/ui/card"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"

import { fetchJobs } from "./action"

interface Job {
  id: number
  name: string
  status: string
  created_at: string
}

export default function JobsPage() {
  const [jobs, setJobs] = useState<Job[]>([])
  const [filteredJobs, setFilteredJobs] = useState<Job[]>([])
  const [statusFilter, setStatusFilter] = useState<string | null>(null)
  const [selectedDate, setSelectedDate] = useState<Date | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    async function loadJobs() {
      setError(null)
      try {
        const data = await fetchJobs()
        setJobs(data)
        setFilteredJobs(data)
      } catch (err) {
        setError("Failed to load jobs")
      }
    }
    loadJobs()
  }, [])

  useEffect(() => {
    if (jobs.length === 0) return

    let filtered = [...jobs]

    if (statusFilter) {
      filtered = filtered.filter((job) => job.status === statusFilter)
    }

    if (selectedDate) {
      const formattedDate = format(selectedDate, "yyyy-MM-dd")
      filtered = filtered.filter((job) =>
        job.created_at.startsWith(formattedDate)
      )
    }

    setFilteredJobs(filtered)
  }, [jobs, statusFilter, selectedDate])

  return (
    <section className="container mx-auto py-10">
      <h1 className="text-3xl font-extrabold mb-6">Jobs</h1>
      {error && <p className="text-red-500 text-center">{error}</p>}

      <div className="flex space-x-4 mb-6">
        <Button
          variant={statusFilter === "pending" ? "default" : "outline"}
          onClick={() =>
            setStatusFilter(statusFilter === "pending" ? null : "pending")
          }
        >
          Pending
        </Button>
        <Button
          variant={statusFilter === "completed" ? "default" : "outline"}
          onClick={() =>
            setStatusFilter(statusFilter === "completed" ? null : "completed")
          }
        >
          Completed
        </Button>

        {/* Date Picker */}
        <Popover>
          <PopoverTrigger asChild>
            <Button variant="outline">
              <CalendarIcon className="mr-2 h-4 w-4" />
              {selectedDate ? format(selectedDate, "PPP") : "Select Date"}
            </Button>
          </PopoverTrigger>
          <PopoverContent align="start">
            <Calendar
              mode="single"
              selected={selectedDate}
              onSelect={setSelectedDate}
            />
          </PopoverContent>
        </Popover>

        {/* Reset Filters */}
        <Button
          variant="destructive"
          onClick={() => {
            setStatusFilter(null)
            setSelectedDate(null)
          }}
        >
          Reset
        </Button>
      </div>

      {/* Jobs List */}
      <div className="grid gap-4">
        {filteredJobs.length > 0 ? (
          filteredJobs.map((job) => (
            <Card key={job.id} className="p-4">
              <h2 className="text-lg font-semibold">{job.name}</h2>
              <p className="text-sm">
                Status:{" "}
                <span
                  className={`capitalize font-semibold ${
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
          ))
        ) : (
          <p className="text-gray-500 text-center">No jobs found.</p>
        )}
      </div>
    </section>
  )
}
