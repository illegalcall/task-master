"use server"

import { cookies } from "next/headers"
import { redirect } from "next/navigation"
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

    // Also create user profile in our backend to keep them in sync
    try {
      // Only create profile if user was created successfully and we have the user ID
      if (data?.user?.id) {
        const response = await fetch(`${process.env.API_URL}/api/users`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({ 
            user_id: data.user.id 
          }),
        })

        if (!response.ok) {
          console.error("Error creating user profile:", await response.text())
          // We don't return an error here because the user was already created in Supabase auth
          // The profile can be created later if needed
        } else {
          console.log("User profile created successfully")
        }
      }
    } catch (profileError) {
      console.error("Error creating user profile:", profileError)
      // We don't fail the signup if profile creation fails
      // The profile can be created later
    }

    return { success: true }
  } catch (error: any) {
    console.error("Unexpected error during sign up:", error.message)
    return { error: "An unexpected error occurred during sign up" }
  }
}

// Handle Google OAuth login
export async function signInWithGoogle() {
  const origin = process.env.NEXT_PUBLIC_APP_URL || process.env.NEXT_PUBLIC_VERCEL_URL || "http://localhost:3000"
  const redirectUrl = `${origin}/auth/callback`
  
  const { data, error } = await supabase.auth.signInWithOAuth({
    provider: 'google',
    options: {
      redirectTo: redirectUrl,
      queryParams: {
        // Optional custom OAuth scopes
        access_type: 'offline',
        prompt: 'consent',
      },
    },
  })

  if (error) {
    console.error("Error with Google sign in:", error.message)
    return { error: error.message }
  }

  // Return the URL that the client should redirect to
  return { url: data.url }
} 