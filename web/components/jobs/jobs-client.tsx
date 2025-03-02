"use client"

import { useRouter, useSearchParams } from "next/navigation"
import { format } from "date-fns"

import { JobsFilter } from "./jobs-filter"

export function JobsClient() {
  const router = useRouter()
  const searchParams = useSearchParams()

  const status = searchParams.get("status")
  const date = searchParams.get("date")

  const selectedDate = date ? new Date(date) : null

  const updateSearchParams = (params: {
    status?: string | null
    date?: string | null
  }) => {
    const newSearchParams = new URLSearchParams(searchParams.toString())

    Object.entries(params).forEach(([key, value]) => {
      if (value === null) {
        newSearchParams.delete(key)
      } else {
        newSearchParams.set(key, value)
      }
    })

    router.push(`?${newSearchParams.toString()}`)
  }

  return (
    <JobsFilter
      statusFilter={status}
      selectedDate={selectedDate}
      onStatusChange={(newStatus) => {
        updateSearchParams({ status: newStatus })
      }}
      onDateChange={(date) => {
        updateSearchParams({
          date: date ? format(date, "yyyy-MM-dd") : null,
        })
      }}
      onReset={() => {
        router.push("")
      }}
    />
  )
}
