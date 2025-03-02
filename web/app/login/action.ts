"use server"

import { cookies } from "next/headers"
import { redirect } from "next/navigation"

export async function loginUser({
  username,
  password,
}: {
  username: string
  password: string
}) {
  try {
    const response = await fetch(`${process.env.API_URL}/api/login`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ username, password }),
    })

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
