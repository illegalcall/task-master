"use server"

import { cookies } from "next/headers"
import { supabase } from '@/lib/supabase'

export async function signUpUser({
  email,
  username,
  password,
}: {
  email: string
  username: string
  password: string
}) {
  try {
    // Register user with Supabase
    const { data, error } = await supabase.auth.signUp({
      email,
      password,
      options: {
        data: {
          username,
        },
      },
    })

    if (error) {
      console.error("Error signing up:", error.message)
      return { error: error.message }
    }

    // Also create user in our backend to keep them in sync
    try {
      const response = await fetch(`${process.env.API_URL}/api/users`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ 
          email, 
          username, 
          password,
          supabase_id: data.user?.id 
        }),
      })

      if (!response.ok) {
        const errorData = await response.json()
        console.error("Error creating user in backend:", errorData)
        return { error: errorData.error || "Failed to create user" }
      }
    } catch (backendError) {
      console.error("Error syncing with backend:", backendError)
      // We don't return the error here because the user is already created in Supabase
      // The backend sync can be retried later
    }

    return { success: true }
  } catch (error: any) {
    console.error("Unexpected error during sign up:", error.message)
    return { error: "An unexpected error occurred during sign up" }
  }
} 