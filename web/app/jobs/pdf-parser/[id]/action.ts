"use server"

import { cookies } from "next/headers"

export interface Job {
  id: number
  name: string
  status: string
  type: string
  created_at: string
  response: string
}

/**
 * Fetches a single job by its ID
 * @param id The ID of the job to fetch
 * @returns The job details
 */
export async function fetchJobById(id: string) {
  const token = cookies().get("token")?.value
  const response = await fetch(`${process.env.API_URL}/api/jobs/${id}`, {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  })

  if (!response.ok) {
    throw new Error(`Failed to fetch job with ID ${id}`)
  }

  const resJson = await response.json()
  const job = resJson.job

  return job as Job
}
