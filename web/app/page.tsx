import { cookies } from "next/headers"
import { redirect } from "next/navigation"

import { LoginForm } from "./login/page"

async function checkSession() {
  const cookieStore = cookies()
  const sessionToken = cookieStore.get("sessionToken")
  return !!sessionToken
}

export default async function Home() {
  const isLoggedIn = await checkSession()

  if (isLoggedIn) {
    redirect("/dashboard")
  }

  return (
    <main className="flex min-h-screen flex-col items-center justify-center p-24">
      <LoginForm />
    </main>
  )
}
