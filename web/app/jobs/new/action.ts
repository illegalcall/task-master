"use server"

import { cookies } from "next/headers"

export async function createJob({
  jobName,
  pdfSource,
  expectedSchema,
}: {
  jobName: string
  pdfSource: string
  expectedSchema: string
}) {
  try {
    const response = await fetch(
      `${process.env.API_URL}/api/jobs/parse-document`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ jobName, pdfSource, expectedSchema }),
      }
    )

    const data = await response.json()

    if (!response.ok) {
      return { error: data.error || "Invalid credentials" }
    }

    cookies().set("token", data.token, {
      httpOnly: true,
      secure: process.env.NODE_ENV === "production",
      sameSite: "lax",
      path: "/",
    })
  } catch (error) {
    return { error: "An unexpected error occurred" }
  }
}
