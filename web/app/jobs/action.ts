"use server"

import { cookies } from "next/headers"

export async function fetchJobs() {
  const token = cookies().get("token")?.value

  const response = await fetch(`${process.env.API_URL}/api/jobs`, {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  })

  if (!response.ok) {
    throw new Error("Failed to fetch jobs")
  }

  return response.json()
}
