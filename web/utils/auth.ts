export async function fetchWithAuth(url: string, options: RequestInit = {}) {
  try {
    const token = localStorage.getItem("accessToken")
    const tokenType = localStorage.getItem("tokenType") || "Bearer"

    if (!token) {
      console.error("Authentication token is missing")
      throw new Error("Unauthorized: No access token found")
    }

    options.headers = {
      ...options.headers,
      Authorization: `${tokenType} ${token}`,
      "Content-Type": "application/json",
    }

    options.credentials = "include" // ✅ Fix for CORS with auth

    const response = await fetch(url, options)

    if (!response.ok) {
      const errorText = await response.text() // Read response error
      console.error(`Error ${response.status}: ${errorText}`)
      throw new Error(
        `Request failed with status ${response.status}: ${errorText}`
      )
    }

    const data = await response.json()
    return data
  } catch (error: any) {
    console.error(
      "Fetch error:",
      error.message || "An unexpected error occurred"
    )
    return [] // Return empty array on failure to prevent crashes
  }
}
