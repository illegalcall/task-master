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

    return { success: true }
  } catch (error: any) {
    console.error("Unexpected error during sign up:", error.message)
    return { error: "An unexpected error occurred during sign up" }
  }
} 