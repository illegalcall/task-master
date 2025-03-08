"use server"

import { cookies } from "next/headers"

export async function createJob({
  jobName,
  pdfSource,
  expectedSchema,
  description,
}: {
  jobName: string
  pdfSource: string
  expectedSchema: string
  description: string
}) {
  try {
    const payload = JSON.stringify({
      name: jobName,
      pdf_source: pdfSource,
      expected_schema: expectedSchema,
      description: description,
    })

    const token = cookies().get("token")

    const response = await fetch(
      `${process.env.API_URL}/api/jobs/parse-document`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token?.value}`,
        },
        body: payload,
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
    console.error("error", error)
    return { error: "An unexpected error occurred" }
  }
}
