import { createClient } from '@supabase/supabase-js'

// Extract just the project reference ID from a Supabase URL
// From: akrqbuajqkirdekonpzy.supabase.co
// To: akrqbuajqkirdekonpzy
const extractProjectRef = (url: string): string => {
  // Remove any protocol prefix
  url = url.replace(/^https?:\/\//, '')
  
  // Split by the first dot to get just the project reference
  return url.split('.')[0]
}

// Initialize the Supabase client
const supabaseUrl = `https://${process.env.NEXT_PUBLIC_SUPABASE_URL}`
const supabaseKey = process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY as string

// For client-side usage, createClient expects a full URL with https://
export const supabase = createClient(supabaseUrl, supabaseKey)

// Create a function for client-side usage
export const createSupabaseClient = () => {
  const url = `https://${process.env.NEXT_PUBLIC_SUPABASE_URL}`
  const key = process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY as string
  return createClient(url, key)
} 