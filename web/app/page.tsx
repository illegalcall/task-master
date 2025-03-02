import { cookies } from "next/headers"
import { redirect } from "next/navigation"

async function checkSession() {
  const cookieStore = cookies()
  const sessionToken = cookieStore.get("sessionToken")
  return !!sessionToken
}

export default async function Home() {
  const isLoggedIn = await checkSession()
  if (isLoggedIn) {
    redirect("/dashboard")
  } else {
    redirect("/login")
  }
}
